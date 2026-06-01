package repository

// File: repository/leave_repo.go

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"absensi_karyawan/utils"
)

type LeaveRepo struct {
	DB *sql.DB
}

func NewLeaveRepo(db *sql.DB) *LeaveRepo {
	return &LeaveRepo{DB: db}
}

// ─── LEAVE REQUESTS ──────────────────────────────────────────────────────────

// GetAllRequestsByBranch — semua pengajuan cuti karyawan di cabang tertentu
func (r *LeaveRepo) GetAllRequestsByBranch(branchID, page, limit int, status string) ([]map[string]interface{}, int, error) {
	offset := utils.Offset(page, limit)

	where := "WHERE e.branch_id = $1"
	args := []interface{}{branchID}
	argIdx := 2

	if status != "" {
		where += fmt.Sprintf(` AND lr.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	// Count
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM leave_requests lr
		JOIN employees e ON e.id = lr.employee_id
		%s`, where)

	var total int
	if err := r.DB.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data
	dataQuery := fmt.Sprintf(`
		SELECT 
			lr.id,
			lr.employee_id,
			COALESCE(e.name, '') AS employee_name,
			COALESCE(d.name, '') AS division_name,
			lr.leave_type,
			TO_CHAR(lr.start_date, 'YYYY-MM-DD'),
			TO_CHAR(lr.end_date, 'YYYY-MM-DD'),
			(lr.end_date - lr.start_date + 1) AS total_days,
			lr.reason,
			lr.status,
			lr.note,
			lr.created_at
		FROM leave_requests lr
		JOIN employees e ON e.id = lr.employee_id
		LEFT JOIN divisions d ON e.division_id = d.id
		%s
		ORDER BY lr.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	dataArgs := append(args, limit, offset)

	rows, err := r.DB.Query(dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var (
			id           int
			employeeID   int
			employeeName string
			divisionName string
			leaveType    string
			startDate    string
			endDate      string
			totalDays    int
			reason       string
			status_      string
			note         sql.NullString
			createdAt    time.Time
		)
		if err := rows.Scan(
			&id, &employeeID, &employeeName, &divisionName, &leaveType,
			&startDate, &endDate, &totalDays, &reason, &status_, &note, &createdAt,
		); err != nil {
			return nil, 0, err
		}

		row := map[string]interface{}{
			"id":            id,
			"employee_id":   employeeID,
			"employee_name": employeeName,
			"division_name": divisionName,
			"leave_type":    leaveType,
			"start_date":    startDate,
			"end_date":      endDate,
			"total_days":    totalDays,
			"reason":        reason,
			"status":        status_,
			"note":          nil,
			"created_at":    createdAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if note.Valid {
			row["note"] = note.String
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, total, nil
}

// GetRequestByID — detail satu pengajuan cuti + validasi branch
func (r *LeaveRepo) GetRequestByID(id, branchID int) (map[string]interface{}, error) {
	query := `
		SELECT 
			lr.id,
			lr.employee_id,
			COALESCE(e.name, '') AS employee_name,
			COALESCE(d.name, '') AS division_name,
			lr.leave_type,
			TO_CHAR(lr.start_date, 'YYYY-MM-DD'),
			TO_CHAR(lr.end_date, 'YYYY-MM-DD'),
			(lr.end_date - lr.start_date + 1) AS total_days,
			lr.reason,
			lr.status,
			COALESCE(lr.note, '') AS note,
			lr.created_at,
			COALESCE(lr.attachment_path, '') AS attachment_path
		FROM leave_requests lr
		JOIN employees e ON e.id = lr.employee_id
		LEFT JOIN divisions d ON e.division_id = d.id
		WHERE lr.id = $1 AND e.branch_id = $2`

	var (
		leaveID        int
		employeeID     int
		employeeName   string
		divisionName   string
		leaveType      string
		startDate      string
		endDate        string
		totalDays      int
		reason         string
		status         string
		note           string
		createdAt      time.Time
		attachmentPath string
	)

	err := r.DB.QueryRow(query, id, branchID).Scan(
		&leaveID, &employeeID, &employeeName, &divisionName, &leaveType,
		&startDate, &endDate, &totalDays, &reason, &status, &note, &createdAt, &attachmentPath,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		log.Printf("GET REQUEST BY ID ERROR: %v", err)
		return nil, err
	}

	result := map[string]interface{}{
		"id":            leaveID,
		"employee_id":   employeeID,
		"employee_name": employeeName,
		"division_name": divisionName,
		"leave_type":    leaveType,
		"start_date":    startDate,
		"end_date":      endDate,
		"total_days":    totalDays,
		"reason":        reason,
		"status":        status,
		"note":          nil,
		"attachment":    nil,
		"created_at":    createdAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if note != "" {
		result["note"] = note
	}
	if attachmentPath != "" {
		result["attachment"] = attachmentPath
	}

	return result, nil
}

// UpdateLeaveStatus — approve / reject + otomatis update kuota
// Hanya "Cuti Tahunan" yang memotong kuota (sesuai format BE1)
func (r *LeaveRepo) UpdateLeaveStatus(id int, status string, note *string) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var employeeID int
	var startDate, endDate, currentStatus, leaveType string
	err = tx.QueryRow(`
		SELECT employee_id, TO_CHAR(start_date,'YYYY-MM-DD'), TO_CHAR(end_date,'YYYY-MM-DD'), status, leave_type
		FROM leave_requests WHERE id = $1
	`, id).Scan(&employeeID, &startDate, &endDate, &currentStatus, &leaveType)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
    UPDATE leave_requests SET status = $1, note = $2, updated_at = NOW() WHERE id = $3
`, status, note, id)
	if err != nil {
		return err
	}

	calcDays := func() (int, int, error) {
		start, err := time.Parse("2006-01-02", startDate)
		if err != nil {
			return 0, 0, err
		}
		end, err := time.Parse("2006-01-02", endDate)
		if err != nil {
			return 0, 0, err
		}
		return int(end.Sub(start).Hours()/24) + 1, start.Year(), nil
	}

	// Hanya "Cuti Tahunan" yang memotong kuota
	if leaveType == "Cuti Tahunan" {
		if status == "approved" {
			days, year, err := calcDays()
			if err != nil {
				return err
			}
			_, err = tx.Exec(`
				UPDATE leave_quotas SET used = used + $1
				WHERE employee_id = $2 AND year = $3
			`, days, employeeID, year)
			if err != nil {
				return err
			}
		}

		// Reject setelah approved → kembalikan kuota
		if status == "rejected" && currentStatus == "approved" {
			days, year, err := calcDays()
			if err != nil {
				return err
			}
			_, err = tx.Exec(`
				UPDATE leave_quotas SET used = GREATEST(used - $1, 0)
				WHERE employee_id = $2 AND year = $3
			`, days, employeeID, year)
			if err != nil {
				return err
			}
		}
	}
	// "Cuti Sakit", "Izin Pribadi", "Cuti Melahirkan" → tidak ada perubahan kuota

	return tx.Commit()
}

// GetSummaryByBranch — statistik kartu atas UI
func (r *LeaveRepo) GetSummaryByBranch(branchID int) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE lr.status = 'pending')  AS pending,
			COUNT(*) FILTER (WHERE lr.status = 'approved') AS approved,
			COUNT(*) FILTER (WHERE lr.status = 'rejected') AS rejected
		FROM leave_requests lr
		JOIN employees e ON e.id = lr.employee_id
		WHERE e.branch_id = $1`

	var total, pending, approved, rejected int
	err := r.DB.QueryRow(query, branchID).Scan(&total, &pending, &approved, &rejected)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_requests": total,
		"pending":        pending,
		"approved":       approved,
		"rejected":       rejected,
	}, nil
}

// ─── LEAVE QUOTA ─────────────────────────────────────────────────────────────

// GetQuotaByEmployee — ambil kuota cuti tahunan karyawan
func (r *LeaveRepo) GetQuotaByEmployee(employeeID, year int) (map[string]interface{}, error) {
	var id, empID, yr, total, used, remaining int
	err := r.DB.QueryRow(`
		SELECT id, employee_id, year, total, used, (total - used) AS remaining
		FROM leave_quotas WHERE employee_id = $1 AND year = $2
	`, employeeID, year).Scan(&id, &empID, &yr, &total, &used, &remaining)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":          id,
		"employee_id": empID,
		"year":        yr,
		"total":       total,
		"used":        used,
		"remaining":   remaining,
	}, nil
}

// UpsertQuota — insert atau update kuota cuti tahunan
func (r *LeaveRepo) UpsertQuota(employeeID, year, total int) error {
	_, err := r.DB.Exec(`
		INSERT INTO leave_quotas (employee_id, year, total, used)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (employee_id, year)
		DO UPDATE SET total = EXCLUDED.total
	`, employeeID, year, total)
	return err
}

// ─── PUBLIC HOLIDAYS ─────────────────────────────────────────────────────────

// GetAllHolidays — semua hari libur + kolom category
// category: "nasional" (dari pemerintah) | "khusus" (ditambah admin)
func (r *LeaveRepo) GetAllHolidays(year int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			id, 
			TO_CHAR(date, 'YYYY-MM-DD'), 
			name, 
			COALESCE(description, ''),
			COALESCE(category, 'nasional')
		FROM public_holidays`

	args := []interface{}{}
	if year > 0 {
		query += ` WHERE EXTRACT(YEAR FROM date) = $1`
		args = append(args, year)
	}
	query += ` ORDER BY date ASC`

	rows, err := r.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var date, name, description, category string
		if err := rows.Scan(&id, &date, &name, &description, &category); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":          id,
			"date":        date,
			"name":        name,
			"description": description,
			"category":    category, // "nasional" atau "khusus"
		})
	}
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// CreateHoliday — tambah hari libur + category
// category otomatis "khusus" kalau ditambah dari admin cabang
func (r *LeaveRepo) CreateHoliday(date, name, description, category string) (int, error) {
	var id int
	err := r.DB.QueryRow(`
		INSERT INTO public_holidays (date, name, description, category) 
		VALUES ($1, $2, $3, $4) 
		ON CONFLICT (date) 
		DO UPDATE SET 
			name        = EXCLUDED.name, 
			description = EXCLUDED.description,
			category    = EXCLUDED.category
		RETURNING id
	`, date, name, description, category).Scan(&id)

	if err == sql.ErrNoRows {
		err = r.DB.QueryRow(`SELECT id FROM public_holidays WHERE date = $1`, date).Scan(&id)
	}

	if err != nil {
		log.Printf("CREATE HOLIDAY ERROR: %v", err)
	}
	return id, err
}

// DeleteHoliday — hapus hari libur berdasarkan ID
func (r *LeaveRepo) DeleteHoliday(id int) (bool, error) {
	result, err := r.DB.Exec(`DELETE FROM public_holidays WHERE id = $1`, id)
	if err != nil {
		log.Printf("DELETE HOLIDAY ERROR: %v", err)
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

// ─── KALENDER ────────────────────────────────────────────────────────────────

// GetCalendarDots — titik di kalender per bulan
// FIX: cuti ditampilkan di semua hari antara start_date dan end_date
func (r *LeaveRepo) GetCalendarDots(branchID, month, year int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	// 1. Ambil hari libur di bulan tersebut
	holidayRows, err := r.DB.Query(`
		SELECT 
			TO_CHAR(date, 'YYYY-MM-DD'), 
			name, 
			COALESCE(category, 'nasional')
		FROM public_holidays
		WHERE EXTRACT(MONTH FROM date) = $1
		AND EXTRACT(YEAR FROM date) = $2
		ORDER BY date ASC
	`, month, year)
	if err != nil {
		log.Printf("GET CALENDAR DOTS HOLIDAY ERROR: %v", err)
		return nil, err
	}
	defer holidayRows.Close()

	for holidayRows.Next() {
		var date, name, category string
		if err := holidayRows.Scan(&date, &name, &category); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"date":     date,
			"type":     "hari_libur",
			"name":     name,
			"category": category,
		})
	}

	// 2. Ambil semua tanggal yang ada karyawan cuti approved
	// FIX: pakai generate_series supaya titik muncul di setiap hari cuti
	leaveRows, err := r.DB.Query(`
		SELECT DISTINCT TO_CHAR(d.date, 'YYYY-MM-DD')
		FROM leave_requests lr
		JOIN employees e ON e.id = lr.employee_id
		JOIN generate_series(
			lr.start_date, 
			lr.end_date, 
			'1 day'::interval
		) AS d(date) ON true
		WHERE e.branch_id = $1
		AND lr.status = 'approved'
		AND EXTRACT(MONTH FROM d.date) = $2
		AND EXTRACT(YEAR FROM d.date) = $3
		ORDER BY 1 ASC
	`, branchID, month, year)
	if err != nil {
		log.Printf("GET CALENDAR DOTS LEAVE ERROR: %v", err)
		return nil, err
	}
	defer leaveRows.Close()

	for leaveRows.Next() {
		var date string
		if err := leaveRows.Scan(&date); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"date": date,
			"type": "cuti_approved",
		})
	}

	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// GetCalendarDetail — detail tanggal ketika diklik di kalender
// Return: map dengan holidays dan leaves (konsisten dengan handler)
func (r *LeaveRepo) GetCalendarDetail(branchID int, date string) (map[string]interface{}, error) {

	// 1. Ambil hari libur di tanggal ini
	var holidays []map[string]interface{}
	holidayRows, err := r.DB.Query(`
		SELECT 
			id, 
			TO_CHAR(date, 'YYYY-MM-DD'), 
			name, 
			COALESCE(description, ''),
			COALESCE(category, 'nasional')
		FROM public_holidays
		WHERE date = $1
	`, date)
	if err != nil {
		log.Printf("GET CALENDAR DETAIL HOLIDAY ERROR: %v", err)
		return nil, err
	}
	defer holidayRows.Close()

	for holidayRows.Next() {
		var id int
		var d, name, description, category string
		if err := holidayRows.Scan(&id, &d, &name, &description, &category); err != nil {
			return nil, err
		}
		holidays = append(holidays, map[string]interface{}{
			"id":          id,
			"date":        d,
			"name":        name,
			"description": description,
			"category":    category,
		})
	}
	if holidays == nil {
		holidays = []map[string]interface{}{}
	}

	// 2. Ambil karyawan yang cuti approved di tanggal ini
	var leaves []map[string]interface{}
	leaveRows, err := r.DB.Query(`
		SELECT 
			lr.id,
			COALESCE(e.name, '') AS employee_name,
			COALESCE(d.name, '') AS division_name,
			lr.leave_type,
			TO_CHAR(lr.start_date, 'YYYY-MM-DD'),
			TO_CHAR(lr.end_date, 'YYYY-MM-DD'),
			lr.status
		FROM leave_requests lr
		JOIN employees e ON e.id = lr.employee_id
		LEFT JOIN divisions d ON e.division_id = d.id
		WHERE e.branch_id = $1
		AND lr.status = 'approved'
		AND $2::date BETWEEN lr.start_date AND lr.end_date
		ORDER BY e.name ASC
	`, branchID, date)
	if err != nil {
		log.Printf("GET CALENDAR DETAIL LEAVE ERROR: %v", err)
		return nil, err
	}
	defer leaveRows.Close()

	for leaveRows.Next() {
		var id int
		var employeeName, divisionName, leaveType, startDate, endDate, status string
		if err := leaveRows.Scan(
			&id, &employeeName, &divisionName, &leaveType, &startDate, &endDate, &status,
		); err != nil {
			return nil, err
		}
		leaves = append(leaves, map[string]interface{}{
			"id":            id,
			"employee_name": employeeName,
			"division_name": divisionName,
			"leave_type":    leaveType,
			"start_date":    startDate,
			"end_date":      endDate,
			"status":        status,
		})
	}
	if leaves == nil {
		leaves = []map[string]interface{}{}
	}

	return map[string]interface{}{
		"date":     date,
		"holidays": holidays,
		"leaves":   leaves,
	}, nil
}
// GetRecentActivity — ambil aktivitas terbaru seputar cuti di cabang tertentu
// Menggabungkan 2 jenis aktivitas:
// 1. "submitted" → karyawan baru mengajukan cuti (berdasarkan created_at)
// 2. "approved" / "rejected" → admin sudah memproses pengajuan (berdasarkan updated_at)
func (r *LeaveRepo) GetRecentActivity(branchID int, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			lr.id,
			COALESCE(e.name, '') AS employee_name,
			lr.status,
			lr.created_at,
			lr.updated_at
		FROM leave_requests lr
		JOIN employees e ON e.id = lr.employee_id
		WHERE e.branch_id = $1
		ORDER BY 
			GREATEST(lr.created_at, COALESCE(lr.updated_at, lr.created_at)) DESC
		LIMIT $2
	`
 
	rows, err := r.DB.Query(query, branchID, limit)
	if err != nil {
		log.Printf("GET RECENT ACTIVITY ERROR: %v", err)
		return nil, err
	}
	defer rows.Close()
 
	var results []map[string]interface{}
	for rows.Next() {
		var (
			id           int
			employeeName string
			status       string
			createdAt    time.Time
			updatedAt    sql.NullTime
		)
		if err := rows.Scan(&id, &employeeName, &status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
 
		// Tentukan type dan message berdasarkan status
		activityType := "submitted"
		message := employeeName + " submitted leave request"
		activityTime := createdAt
 
		if status == "approved" {
			activityType = "approved"
			message = "Admin approved leave for " + employeeName
			if updatedAt.Valid {
				activityTime = updatedAt.Time
			}
		} else if status == "rejected" {
			activityType = "rejected"
			message = "Admin rejected leave for " + employeeName
			if updatedAt.Valid {
				activityTime = updatedAt.Time
			}
		}
 
		results = append(results, map[string]interface{}{
			"id":            id,
			"type":          activityType,
			"employee_name": employeeName,
			"message":       message,
			"created_at":    activityTime.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
 
	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}