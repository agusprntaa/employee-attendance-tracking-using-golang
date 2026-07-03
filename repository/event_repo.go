package repository

import (
	"absensi_karyawan/models"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"
)

type EventRepo struct {
	db *sql.DB
}

func NewEventRepo(db *sql.DB) *EventRepo {
	return &EventRepo{db: db}
}

// CreateEvent — buat event baru, qr_code dikosongkan dulu
// QR baru di-generate setelah admin klik generate-qr
func (r *EventRepo) CreateEvent(branchID, createdBy int, req models.CreateEventRequest) (int, error) {
	expiresAtStr := req.EndDate + " " + req.EndTime + ":00"

	secret := os.Getenv("QR_SECRET")
	raw := fmt.Sprintf("event:%d:%s:%s", branchID, req.StartDate, time.Now().Format("15:04:05.000"))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(raw))
	qrCode := hex.EncodeToString(mac.Sum(nil))

	var id int
	log.Printf("INSERT EVENT -> Latitude: %f | Longitude: %f", req.Latitude, req.Longitude)
	err := r.db.QueryRow(`
		INSERT INTO events 
			(branch_id, created_by, name, description, location,
			 latitude, longitude, radius_meter, date, start_date, end_date, start_time, end_time,
			 qr_code, expires_at)
		VALUES 
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`,
		branchID, createdBy, req.Name, req.Description, req.Location,
		req.Latitude, req.Longitude, req.RadiusMeter,
		req.StartDate, req.EndDate, req.StartTime, req.EndTime, qrCode, expiresAtStr,
	).Scan(&id)
	if err != nil {
		log.Printf("CREATE EVENT ERROR: %v", err)
		return 0, err
	}

	expiresAt, _ := time.ParseInLocation("2006-01-02 15:04:05", expiresAtStr, time.Local)
	_, err = r.GenerateQRToken(id, branchID, expiresAt)
	if err != nil {
		log.Printf("GENERATE QR AFTER CREATE ERROR: %v", err)
	}

	return id, nil
}

// RegenerateAllEventQR — dipanggil setiap hari untuk regenerate QR semua event aktif
// RegenerateAllEventQR — sekarang cek end_date bukan date
func (r *EventRepo) RegenerateAllEventQR() error {
	rows, err := r.db.Query(`
		SELECT id, branch_id, expires_at
		FROM events
		WHERE expires_at > NOW()
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var eventID, branchID int
		var expiresAt time.Time
		if err := rows.Scan(&eventID, &branchID, &expiresAt); err != nil {
			continue
		}
		if _, err := r.GenerateQRToken(eventID, branchID, expiresAt); err != nil {
			log.Printf("REGENERATE EVENT QR ERROR event_id=%d: %v", eventID, err)
		}
	}
	return nil
}

// GetEventsByBranch — list semua event milik cabang ini
// Filter opsional: status (upcoming/active/done) dan date
func (r *EventRepo) GetEventsByBranch(branchID int, status, date string) ([]models.Event, error) {
	where := "WHERE e.branch_id = $1"
	args := []interface{}{branchID}
	argIdx := 2

	if date != "" {
		where += fmt.Sprintf(" AND $%d BETWEEN e.start_date AND e.end_date", argIdx)
		args = append(args, date)
		argIdx++
	}

	switch status {
	case "upcoming":
		where += " AND (e.start_date + e.start_time::interval) > NOW()"
	case "active":
		where += " AND NOW() BETWEEN (e.start_date + e.start_time::interval) AND (e.end_date + e.end_time::interval)"
	case "done":
		where += " AND (e.end_date + e.end_time::interval) <= NOW()"
	}

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT 
			e.id,
			COALESCE(e.branch_id, 0),
			COALESCE(e.created_by, 0),
			e.name,
			COALESCE(e.description, ''),
			COALESCE(e.location, ''),
			COALESCE(e.latitude, 0),
			COALESCE(e.longitude, 0),
			e.radius_meter,
			TO_CHAR(e.start_date, 'YYYY-MM-DD'),
			TO_CHAR(e.end_date, 'YYYY-MM-DD'),
			COALESCE(TO_CHAR(e.start_time, 'HH24:MI'), ''),
			COALESCE(TO_CHAR(e.end_time, 'HH24:MI'), ''),
			COALESCE(TO_CHAR(e.expires_at, 'YYYY-MM-DD HH24:MI:SS'), ''),
			TO_CHAR(e.created_at, 'YYYY-MM-DD HH24:MI:SS'),
			(SELECT COUNT(*) FROM event_participants ep WHERE ep.event_id = e.id) AS total_participants,
			(SELECT COUNT(*) FROM attendance a WHERE a.event_id = e.id AND a.checkin_type = 'qr_event') AS total_present,
			CASE
				WHEN (e.start_date + e.start_time::interval) > NOW() THEN 'Belum Dimulai'
				WHEN NOW() BETWEEN (e.start_date + e.start_time::interval) AND (e.end_date + e.end_time::interval) THEN 'Berlangsung'
				ELSE 'Selesai'
			END AS event_status
		FROM events e
		%s
		ORDER BY e.start_date DESC, e.start_time ASC
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(
			&e.ID, &e.BranchID, &e.CreatedBy,
			&e.Name, &e.Description, &e.Location,
			&e.Latitude, &e.Longitude, &e.RadiusMeter,
			&e.StartDate, &e.EndDate, &e.StartTime, &e.EndTime,
			&e.ExpiresAt, &e.CreatedAt,
			&e.TotalParticipants, &e.TotalPresent, &e.Status,
		); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	if results == nil {
		results = []models.Event{}
	}
	return results, nil
}

// GetEventByID — detail satu event, validasi branch supaya admin cabang
// tidak bisa akses event cabang lain
func (r *EventRepo) GetEventByID(eventID, branchID int) (*models.Event, error) {
	var e models.Event
	err := r.db.QueryRow(`
		SELECT 
			id, COALESCE(branch_id, 0), COALESCE(created_by, 0),
			name, COALESCE(description, ''), COALESCE(location, ''),
			COALESCE(latitude, 0), COALESCE(longitude, 0), radius_meter,
			TO_CHAR(start_date, 'YYYY-MM-DD'),
			TO_CHAR(end_date, 'YYYY-MM-DD'),
			COALESCE(TO_CHAR(start_time, 'HH24:MI'), ''),
			COALESCE(TO_CHAR(end_time, 'HH24:MI'), ''),
			COALESCE(TO_CHAR(expires_at, 'YYYY-MM-DD HH24:MI:SS'), ''),
			TO_CHAR(created_at, 'YYYY-MM-DD HH24:MI:SS')
		FROM events
		WHERE id = $1 AND branch_id = $2
	`, eventID, branchID).Scan(
		&e.ID, &e.BranchID, &e.CreatedBy,
		&e.Name, &e.Description, &e.Location,
		&e.Latitude, &e.Longitude, &e.RadiusMeter,
		&e.StartDate, &e.EndDate, &e.StartTime, &e.EndTime,
		&e.ExpiresAt, &e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// GetEventSummary — summary peserta + absensi untuk GET /events/:id
func (r *EventRepo) GetEventSummary(eventID, branchID int, date string) (int, int, int, error) {
	// Default date jika kosong
	if date == "" {
		var startDate, endDate time.Time
		err := r.db.QueryRow(`
			SELECT start_date, end_date FROM events WHERE id = $1 AND branch_id = $2
		`, eventID, branchID).Scan(&startDate, &endDate)
		if err == nil {
			today := time.Now()
			if today.Before(startDate) {
				date = startDate.Format("2006-01-02")
			} else if today.After(endDate) {
				date = endDate.Format("2006-01-02")
			} else {
				date = today.Format("2006-01-02")
			}
		}
	}

	var totalParticipants, totalHadir int

	err := r.db.QueryRow(`
		SELECT 
			COUNT(DISTINCT ep.employee_id) AS total_participants,
			COUNT(DISTINCT a.employee_id) AS total_hadir
		FROM event_participants ep
		LEFT JOIN attendance a 
			ON a.employee_id = ep.employee_id
			AND a.event_id = ep.event_id
			AND a.checkin_type = 'qr_event'
			AND a.date = $3::date
		WHERE ep.event_id = $1
		AND EXISTS (
			SELECT 1 
			FROM events e 
			WHERE e.id = ep.event_id 
			AND e.branch_id = $2
		)
	`, eventID, branchID, date).Scan(&totalParticipants, &totalHadir)

	if err != nil {
		return 0, 0, 0, err
	}

	totalBelum := totalParticipants - totalHadir
	return totalParticipants, totalHadir, totalBelum, nil
}

// GetEventAttendanceByDate — tabel absensi peserta per tanggal
// Default date: hari ini jika berlangsung, start_date jika belum mulai, end_date jika sudah selesai
func (r *EventRepo) GetEventAttendanceByDate(eventID, branchID int, date string) ([]map[string]interface{}, error) {

	args := []interface{}{eventID, branchID}
	dateFilter := ""

	if date != "" {
		dateFilter = "AND gs.date = $3::date"
		args = append(args, date)
	}

	query := fmt.Sprintf(`
		SELECT
			e.id AS employee_id,
			COALESCE(e.name, e.username) AS employee_name,
			COALESCE(d.name, '') AS division_name,
			TO_CHAR(gs.date, 'YYYY-MM-DD') AS date,
			COALESCE(TO_CHAR(a.check_in, 'HH24:MI:SS'), '') AS check_in,
			COALESCE(TO_CHAR(a.check_out, 'HH24:MI:SS'), '') AS check_out,
			CASE
				WHEN a.check_in IS NOT NULL AND a.check_out IS NOT NULL THEN 'Hadir'
				WHEN a.check_in IS NOT NULL AND a.check_out IS NULL THEN 'Belum Pulang'
				ELSE 'Belum Hadir'
			END AS status
		FROM event_participants ep
		JOIN employees e
			ON e.id = ep.employee_id
		LEFT JOIN divisions d
			ON d.id = e.division_id
		JOIN events ev
			ON ev.id = ep.event_id
		JOIN generate_series(
			ev.start_date,
			ev.end_date,
			interval '1 day'
		) AS gs(date)
			ON true
		LEFT JOIN attendance a
			ON a.employee_id = e.id
			AND a.event_id = ep.event_id
			AND a.checkin_type = 'qr_event'
			AND a.date = gs.date::date
		WHERE ep.event_id = $1
		AND e.branch_id = $2
		%s
		ORDER BY
    CASE
        WHEN a.check_in IS NOT NULL AND a.check_out IS NOT NULL THEN 0
        WHEN a.check_in IS NOT NULL AND a.check_out IS NULL THEN 1
        ELSE 2
    END,
    gs.date ASC,
    e.name ASC
	`, dateFilter)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}

	for rows.Next() {
		var (
			employeeID   int
			employeeName string
			divisionName string
			attendDate   string
			checkIn      string
			checkOut     string
			status       string
		)

		if err := rows.Scan(
			&employeeID,
			&employeeName,
			&divisionName,
			&attendDate,
			&checkIn,
			&checkOut,
			&status,
		); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"employee_id":   employeeID,
			"employee_name": employeeName,
			"division_name": divisionName,
			"date":          attendDate,
			"check_in":      checkIn,
			"check_out":     checkOut,
			"status":        status,
		})
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	return results, nil
}

// GenerateQRToken — generate token HMAC dan simpan ke qr_tokens
// Kalau sudah ada token untuk event ini, langsung replace (ON CONFLICT)
// Token expires sesuai end_time event
func (r *EventRepo) GenerateQRToken(eventID, branchID int, expiresAt time.Time) (string, error) {
	secret := os.Getenv("QR_SECRET")
	raw := fmt.Sprintf("event:%d:%d:%s", eventID, branchID, time.Now().Format("2006-01-02 15:04:05"))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(raw))
	token := hex.EncodeToString(mac.Sum(nil))

	_, err := r.db.Exec(`
		INSERT INTO qr_tokens (token, branch_id, date, expires_at, event_id, token_type)
		VALUES ($1, $2, CURRENT_DATE, $3, $4, 'event')
		ON CONFLICT (event_id)
		WHERE token_type = 'event' AND event_id IS NOT NULL
		DO UPDATE SET
			token      = EXCLUDED.token,
			expires_at = EXCLUDED.expires_at,
			date       = CURRENT_DATE
	`, token, branchID, expiresAt, eventID)
	if err != nil {
		log.Printf("GENERATE QR TOKEN ERROR: %v", err)
		return "", err
	}
	return token, nil
}

// GetActiveQRToken — ambil token QR event yang masih aktif (belum expired)
// Return token kosong jika belum di-generate atau sudah expired
func (r *EventRepo) GetActiveQRToken(eventID, branchID int) (string, time.Time, error) {
	var token string
	var expiresAt time.Time

	err := r.db.QueryRow(`
		SELECT token, expires_at
		FROM qr_tokens
		WHERE event_id = $1
		AND branch_id = $2
		AND token_type = 'event'
		AND expires_at > NOW()
		LIMIT 1
	`, eventID, branchID).Scan(&token, &expiresAt)

	if err == sql.ErrNoRows {
		return "", time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// GetParticipants — list karyawan yang terdaftar di event
func (r *EventRepo) GetParticipants(eventID, branchID int) ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`
		SELECT 
			e.id,
			COALESCE(e.name, e.username) AS name,
			COALESCE(d.name, '') AS division_name,
			CASE WHEN ep.employee_id IS NOT NULL THEN true ELSE false END AS terdaftar,
			COALESCE(TO_CHAR(a.check_in, 'HH24:MI:SS'), '') AS check_in,
			COALESCE(TO_CHAR(a.check_out, 'HH24:MI:SS'), '') AS check_out,
			CASE
				WHEN a.check_in IS NOT NULL AND a.check_out IS NOT NULL THEN 'Hadir'
				WHEN a.check_in IS NOT NULL AND a.check_out IS NULL THEN 'Belum Pulang'
				ELSE 'Belum Hadir'
			END AS status_absen
		FROM employees e
		LEFT JOIN divisions d ON d.id = e.division_id
		LEFT JOIN event_participants ep 
			ON ep.employee_id = e.id 
			AND ep.event_id = $1
		LEFT JOIN attendance a 
			ON a.employee_id = e.id
			AND a.event_id = $1
			AND a.checkin_type = 'qr_event'
		WHERE e.branch_id = $2
		AND e.role = 'karyawan'
		AND e.status = 'active'
		ORDER BY 
			CASE WHEN ep.employee_id IS NULL THEN 1 ELSE 0 END,
			e.name ASC
	`, eventID, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var (
			id           int
			name         string
			divisionName string
			terdaftar    bool
			checkIn      string
			checkOut     string
			statusAbsen  string
		)
		if err := rows.Scan(
			&id, &name, &divisionName, &terdaftar,
			&checkIn, &checkOut, &statusAbsen,
		); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"employee_id":   id,
			"employee_name": name,
			"division_name": divisionName,
			"terdaftar":     terdaftar,
			"check_in":      checkIn,
			"check_out":     checkOut,
			"status":        statusAbsen,
		})
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// GetEventParticipantsList — hanya peserta yang SUDAH terdaftar + data absensi
// Dipakai untuk tabel "Peserta Event" di halaman detail
// GetEventParticipantsList — peserta + data absensi per hari
// Tambah parameter date opsional untuk filter hari tertentu
func (r *EventRepo) GetEventParticipantsList(eventID, branchID int, date string) ([]map[string]interface{}, error) {
	// Kalau date kosong, default ke CURRENT_DATE atau start_date event
	if date == "" {
		var startDate, endDate time.Time
		err := r.db.QueryRow(`
			SELECT start_date, end_date FROM events WHERE id = $1 AND branch_id = $2
		`, eventID, branchID).Scan(&startDate, &endDate)
		if err == nil {
			today := time.Now()
			if today.Before(startDate) {
				date = startDate.Format("2006-01-02")
			} else if today.After(endDate) {
				date = endDate.Format("2006-01-02")
			} else {
				date = today.Format("2006-01-02")
			}
		}
	}

	where := `WHERE ep.event_id = $1 AND e.branch_id = $2`
	args := []interface{}{eventID, branchID}
	argIdx := 3

	if date != "" {
		where += fmt.Sprintf(` AND a.date = $%d`, argIdx)
		args = append(args, date)
		argIdx++
	}

	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT 
			e.id,
			COALESCE(e.name, e.username) AS name,
			COALESCE(d.name, '') AS division_name,
			COALESCE(TO_CHAR(a.date, 'YYYY-MM-DD'), '') AS attendance_date,
			COALESCE(TO_CHAR(a.check_in, 'HH24:MI:SS'), '') AS check_in,
			COALESCE(TO_CHAR(a.check_out, 'HH24:MI:SS'), '') AS check_out,
			CASE
				WHEN a.check_in IS NOT NULL AND a.check_out IS NOT NULL THEN 'Hadir'
				WHEN a.check_in IS NOT NULL AND a.check_out IS NULL THEN 'Belum Pulang'
				ELSE 'Belum Hadir'
			END AS status_absen
		FROM event_participants ep
		JOIN employees e ON e.id = ep.employee_id
		LEFT JOIN divisions d ON d.id = e.division_id
		LEFT JOIN attendance a 
			ON a.employee_id = e.id
			AND a.event_id = ep.event_id
			AND a.checkin_type = 'qr_event'
		%s
		ORDER BY e.name ASC, a.date ASC
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var (
			id             int
			name           string
			divisionName   string
			attendanceDate string
			checkIn        string
			checkOut       string
			statusAbsen    string
		)
		if err := rows.Scan(&id, &name, &divisionName, &attendanceDate, &checkIn, &checkOut, &statusAbsen); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"employee_id":   id,
			"employee_name": name,
			"division_name": divisionName,
			"date":          attendanceDate,
			"check_in":      checkIn,
			"check_out":     checkOut,
			"status":        statusAbsen,
		})
	}

	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// SetParticipants — set ulang peserta event (replace semua)
func (r *EventRepo) SetParticipants(eventID int, employeeIDs []int) error {
	tx, err := r.db.Begin()
	if err != nil {
		log.Printf("SET PARTICIPANTS BEGIN TX ERROR: %v", err)
		return err
	}
	defer tx.Rollback()

	for _, empID := range employeeIDs {
		_, err = tx.Exec(`
			INSERT INTO event_participants (event_id, employee_id)
			VALUES ($1, $2)
			ON CONFLICT (event_id, employee_id) DO NOTHING
		`, eventID, empID)

		if err != nil {
			log.Printf("SET PARTICIPANTS INSERT ERROR empID=%d: %v", empID, err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("SET PARTICIPANTS COMMIT ERROR: %v", err)
		return err
	}

	return nil
}

// RemoveParticipant — hapus 1 karyawan dari peserta event
func (r *EventRepo) RemoveParticipant(eventID, employeeID int) (bool, error) {
	result, err := r.db.Exec(`
		DELETE FROM event_participants 
		WHERE event_id = $1 AND employee_id = $2
	`, eventID, employeeID)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

// GetEventDateRange — generate list tanggal dari start_date sampai end_date
// Dipakai FE untuk dropdown filter search by tanggal
func (r *EventRepo) GetEventDateRange(eventID, branchID int) ([]string, error) {
	var startDate, endDate time.Time
	err := r.db.QueryRow(`
		SELECT start_date, end_date 
		FROM events 
		WHERE id = $1 AND branch_id = $2
	`, eventID, branchID).Scan(&startDate, &endDate)
	if err != nil {
		return nil, err
	}

	var dates []string
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	return dates, nil
}

// GetAllEmployeeIDs — ambil semua ID karyawan aktif di cabang
// Dipakai FE untuk fitur "Pilih Semua" peserta event
func (r *EventRepo) GetAllEmployeeIDs(branchID int) ([]int, error) {
	rows, err := r.db.Query(`
		SELECT id
		FROM employees
		WHERE branch_id = $1
		AND role = 'karyawan'
		AND status = 'active'
		ORDER BY name ASC
	`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if ids == nil {
		ids = []int{}
	}
	return ids, nil
}

// GetEventAttendance — rekap karyawan yang sudah scan QR event
// Filter checkin_type = 'qr_event' supaya tidak campur dengan absensi face harian
func (r *EventRepo) GetEventAttendance(eventID, branchID int) ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`
		SELECT
			e.id AS employee_id,
			COALESCE(e.name, e.username) AS employee_name,
			COALESCE(d.name, '') AS division_name,
			CASE WHEN a.id IS NOT NULL THEN true ELSE false END AS hadir,
			COALESCE(TO_CHAR(a.check_in, 'HH24:MI:SS'), '') AS check_in,
			COALESCE(a.status, '') AS status,
			COALESCE(a.distance_meter, 0) AS distance_meter
		FROM employees e
		LEFT JOIN attendance a 
			ON a.employee_id = e.id
			AND a.event_id = $1
			AND a.checkin_type = 'qr_event'
		LEFT JOIN divisions d ON d.id = e.division_id
		WHERE e.branch_id = $2
		AND e.role = 'karyawan'
		AND e.status = 'active'
		ORDER BY 
			CASE WHEN a.id IS NULL THEN 1 ELSE 0 END,
			a.check_in ASC,
			e.name ASC
	`, eventID, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var (
			employeeID   int
			employeeName string
			divisionName string
			hadir        bool
			checkIn      string
			status       string
			distance     float64
		)
		if err := rows.Scan(
			&employeeID, &employeeName, &divisionName,
			&hadir, &checkIn, &status, &distance,
		); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"employee_id":    employeeID,
			"employee_name":  employeeName,
			"division_name":  divisionName,
			"hadir":          hadir,
			"check_in":       checkIn,
			"status":         status,
			"distance_meter": distance,
		})
	}

	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// update event
func (r *EventRepo) UpdateEvent(eventID, branchID int, req models.UpdateEventRequest) error {
	expiresAtStr := req.EndDate + " " + req.EndTime + ":00"

	result, err := r.db.Exec(`
		UPDATE events
		SET
			name = $1,
			description = $2,
			location = $3,
			latitude = $4,
			longitude = $5,
			radius_meter = $6,
			start_date = $7,
			end_date = $8,
			start_time = $9,
			end_time = $10,
			expires_at = $11
		WHERE id = $12
		AND branch_id = $13
	`,
		req.Name,
		req.Description,
		req.Location,
		req.Latitude,
		req.Longitude,
		req.RadiusMeter,
		req.StartDate,
		req.EndDate,
		req.StartTime,
		req.EndTime,
		expiresAtStr,
		eventID,
		branchID,
	)

	if err != nil {
		log.Printf("UPDATE EVENT ERROR: %v", err)
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteEvent — hapus event, validasi branch supaya tidak bisa hapus event cabang lain
func (r *EventRepo) DeleteEvent(eventID, branchID int) (bool, error) {
	result, err := r.db.Exec(`
		DELETE FROM events WHERE id = $1 AND branch_id = $2
	`, eventID, branchID)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}
