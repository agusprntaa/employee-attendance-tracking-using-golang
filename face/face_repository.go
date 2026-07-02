package face

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Repository struct {
	DB *sql.DB
}

// ─────────────────────────────────────────
// Struct internal
// ─────────────────────────────────────────

type FaceToken struct {
	ID              int
	EmployeeID      int
	Token           string
	ExpiresAt       time.Time
	IsUsed          bool
	FaceVerified    bool
	ConfidenceScore float64
	EventID         *int // nil = token WFO, terisi = token untuk event ini
}

// PoseEmbedding — satu baris embedding dengan label pose-nya
type PoseEmbedding struct {
	Pose      string
	Embedding []float64
}

// EmployeeFaceData — data dasar karyawan untuk proses checkin
type EmployeeFaceData struct {
	FaceRegistered bool
	BranchID       int
}

type EventData struct {
	ID          int
	Name        string
	Latitude    float64
	Longitude   float64
	RadiusMeter int
	BranchID    int
	Date        time.Time
	ExpiresAt   time.Time
}

// ─────────────────────────────────────────
// EMPLOYEE FACE DATA (dasar, tanpa embedding)
// ─────────────────────────────────────────

// GetEmployeeFaceData — ambil status registrasi + branch_id saja
// Embedding TIDAK diambil di sini — pakai GetAllPoseEmbeddings terpisah
// karena sekarang ada 5 baris, bukan 1 kolom.
func (r *Repository) GetEmployeeFaceData(employeeID int) (*EmployeeFaceData, error) {
	var (
		faceRegistered bool
		branchID       sql.NullInt64
	)
	err := r.DB.QueryRow(`
		SELECT face_registered, branch_id
		FROM employees
		WHERE id = $1
	`, employeeID).Scan(&faceRegistered, &branchID)
	if err != nil {
		return nil, err
	}
	return &EmployeeFaceData{
		FaceRegistered: faceRegistered,
		BranchID:       int(branchID.Int64),
	}, nil
}

// MarkFaceRegistered — set flag setelah semua 5 pose berhasil disimpan
// Dipanggil terpisah dari SavePoseEmbedding agar flag hanya true
// jika SEMUA pose sukses (dipanggil di akhir, setelah loop 5 pose selesai)
func (r *Repository) MarkFaceRegistered(employeeID int) error {
	_, err := r.DB.Exec(`
		UPDATE employees
		SET face_registered    = true,
		    face_registered_at = NOW()
		WHERE id = $1
	`, employeeID)
	return err
}

// ─────────────────────────────────────────
// POSE EMBEDDINGS — tabel baru employee_face_embeddings
// ─────────────────────────────────────────

// SavePoseEmbedding — simpan satu embedding untuk satu pose
// Pakai ON CONFLICT supaya re-register otomatis replace pose yang sama,
// tidak perlu DELETE manual dulu.
func (r *Repository) SavePoseEmbedding(employeeID int, pose string, embedding []float64) error {
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return err
	}
	_, err = r.DB.Exec(`
		INSERT INTO employee_face_embeddings (employee_id, pose, embedding)
		VALUES ($1, $2, $3)
		ON CONFLICT (employee_id, pose) DO UPDATE
		SET embedding  = EXCLUDED.embedding,
		    created_at = NOW()
	`, employeeID, pose, string(embeddingJSON))
	return err
}

// GetAllPoseEmbeddings — ambil kelima embedding milik satu karyawan
// Dipakai saat verify-face: loop bandingkan satu-satu ke embedding baru
func (r *Repository) GetAllPoseEmbeddings(employeeID int) ([]PoseEmbedding, error) {
	rows, err := r.DB.Query(`
		SELECT pose, embedding
		FROM employee_face_embeddings
		WHERE employee_id = $1
	`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PoseEmbedding
	for rows.Next() {
		var pose, embeddingJSON string
		if err := rows.Scan(&pose, &embeddingJSON); err != nil {
			return nil, err
		}
		var emb []float64
		if err := json.Unmarshal([]byte(embeddingJSON), &emb); err != nil {
			return nil, err
		}
		result = append(result, PoseEmbedding{Pose: pose, Embedding: emb})
	}
	return result, rows.Err()
}

// GetRegisteredPoses — list nama pose yang sudah tersimpan (untuk FaceStatus)
func (r *Repository) GetRegisteredPoses(employeeID int) ([]string, error) {
	rows, err := r.DB.Query(`
		SELECT pose FROM employee_face_embeddings
		WHERE employee_id = $1
		ORDER BY pose
	`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var poses []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		poses = append(poses, p)
	}
	return poses, rows.Err()
}

// DeleteAllPoseEmbeddings — hapus semua pose milik karyawan
// Dipakai kalau registrasi gagal di tengah jalan (rollback manual)
// atau admin reset wajah karyawan
func (r *Repository) DeleteAllPoseEmbeddings(employeeID int) error {
	_, err := r.DB.Exec(`
		DELETE FROM employee_face_embeddings WHERE employee_id = $1
	`, employeeID)
	return err
}

// ─────────────────────────────────────────
// BRANCHES
// ─────────────────────────────────────────

func (r *Repository) GetBranchLocation(branchID int) (lat, lon float64, radiusMeter int, err error) {
	err = r.DB.QueryRow(`
		SELECT latitude, longitude, radius_meter
		FROM branches
		WHERE id = $1
	`, branchID).Scan(&lat, &lon, &radiusMeter)
	return
}

// ─────────────────────────────────────────
// FACE TOKEN
// ─────────────────────────────────────────

func (r *Repository) InsertFaceToken(employeeID int, token string) error {
	_, err := r.DB.Exec(`
		INSERT INTO face_tokens (employee_id, token, expires_at, is_used)
		VALUES ($1, $2, NOW() + INTERVAL '2 minutes', false)
	`, employeeID, token)
	return err
}

func (r *Repository) GetFaceToken(token string, employeeID int) (*FaceToken, error) {
	var ft FaceToken
	var eventID sql.NullInt64
	err := r.DB.QueryRow(`
		SELECT id, employee_id, token, expires_at, is_used,
		       face_verified, COALESCE(confidence_score, 0), event_id
		FROM face_tokens
		WHERE token = $1 AND employee_id = $2
	`, token, employeeID).Scan(
		&ft.ID, &ft.EmployeeID, &ft.Token, &ft.ExpiresAt,
		&ft.IsUsed, &ft.FaceVerified, &ft.ConfidenceScore, &eventID,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if eventID.Valid {
		id := int(eventID.Int64)
		ft.EventID = &id
	}
	return &ft, nil
}

func (r *Repository) MarkFaceVerified(tokenID int, score float64) error {
	_, err := r.DB.Exec(`
		UPDATE face_tokens
		SET face_verified    = true,
		    confidence_score = $2,
		    verified_at      = NOW()
		WHERE id = $1
	`, tokenID, score)
	return err
}

// ─────────────────────────────────────────
// VERIFICATION LOG
// ─────────────────────────────────────────

func (r *Repository) CountDailyFailures(employeeID int) (int, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM face_verification_logs
		WHERE employee_id = $1
		  AND result = 'mismatch'
		  AND attempted_at >= CURRENT_DATE
	`, employeeID).Scan(&count)
	return count, err
}

func (r *Repository) InsertVerificationLog(
	tx *sql.Tx,
	employeeID int,
	attendanceID int,
	result string,
	score float64,
	ipAddress string,
) error {
	const query = "INSERT INTO face_verification_logs (employee_id, attendance_id, result, confidence_score, ip_address) VALUES ($1, $2, $3, $4, $5)"

	var attendanceVal interface{}
	if attendanceID > 0 {
		attendanceVal = attendanceID
	} else {
		attendanceVal = nil
	}

	if tx != nil {
		_, err := tx.Exec(query, employeeID, attendanceVal, result, score, ipAddress)
		return err
	}
	_, err := r.DB.Exec(query, employeeID, attendanceVal, result, score, ipAddress)
	return err
}

// ─────────────────────────────────────────
// ATTENDANCE
// ─────────────────────────────────────────

func (r *Repository) HasCheckedInWFOToday(employeeID int) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM attendance
		WHERE employee_id = $1 AND date = CURRENT_DATE AND checkin_type = 'face_geo'
	`, employeeID).Scan(&count)
	return count > 0, err
}

func (r *Repository) InsertFaceCheckin(
	tx *sql.Tx,
	employeeID, branchID int,
	lat, lon, distanceMeter float64,
	lateMinutes int,
	status string,
	score float64,
) (attendanceID int, err error) {
	const query = `
		INSERT INTO attendance (
			employee_id, branch_id, date,
			check_in, check_in_lat, check_in_lon,
			distance_meter, late_minutes, status,
			face_verified, confidence_score, face_method,
			checkin_type, work_type
		) VALUES (
			$1, $2, CURRENT_DATE,
			NOW(), $3, $4,
			$5, $6, $7,
			true, $8, 'insightface',
			'face_geo', 'WFO'
		)
		RETURNING id
	`
	err = tx.QueryRow(query, employeeID, branchID, lat, lon,
		distanceMeter, lateMinutes, status, score,
	).Scan(&attendanceID)
	return
}

func (r *Repository) MarkTokenConsumed(tx *sql.Tx, tokenID int) error {
	_, err := tx.Exec(`UPDATE face_tokens SET is_used = true WHERE id = $1`, tokenID)
	return err
}

// ─────────────────────────────────────────
// QR EVENT
// ─────────────────────────────────────────

func (r *Repository) GetQRTokenByValue(token string) (*struct {
	ID        int
	EventID   int
	BranchID  int
	ExpiresAt time.Time
	TokenType string
}, error) {
	var t struct {
		ID        int
		EventID   int
		BranchID  int
		ExpiresAt time.Time
		TokenType string
	}
	var eventID sql.NullInt64
	err := r.DB.QueryRow(`
		SELECT id, COALESCE(event_id, 0), branch_id, expires_at, token_type
		FROM qr_tokens
		WHERE token = $1
	`, token).Scan(&t.ID, &eventID, &t.BranchID, &t.ExpiresAt, &t.TokenType)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.EventID = int(eventID.Int64)
	return &t, nil
}

func (r *Repository) GetEventData(eventID int) (*EventData, error) {
	var e EventData
	var lat, lon sql.NullFloat64
	var radius sql.NullInt64
	var branchID sql.NullInt64
	err := r.DB.QueryRow(`
		SELECT id, name, latitude, longitude, radius_meter, branch_id, date, expires_at
		FROM events
		WHERE id = $1
	`, eventID).Scan(&e.ID, &e.Name, &lat, &lon, &radius, &branchID, &e.Date, &e.ExpiresAt)
	if err != nil {
		return nil, err
	}
	e.Latitude = lat.Float64
	e.Longitude = lon.Float64
	e.RadiusMeter = int(radius.Int64)
	e.BranchID = int(branchID.Int64) // 0 kalau NULL
	return &e, nil
}

func (r *Repository) HasAttendedEvent(employeeID, eventID int) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM attendance
		WHERE employee_id = $1 AND event_id = $2
	`, employeeID, eventID).Scan(&count)
	return count > 0, err
}

func (r *Repository) InsertEventCheckin(
	tx *sql.Tx,
	employeeID, eventID, branchID int,
	lat, lon, distanceMeter float64,
	score float64,
) (attendanceID int, err error) {
	var branchVal interface{}
	if branchID > 0 {
		branchVal = branchID
	} else {
		branchVal = nil // event tanpa cabang spesifik
	}

	const query = `
		INSERT INTO attendance (
			employee_id, branch_id, date,
			check_in, check_in_lat, check_in_lon,
			distance_meter, status, checkin_type,
			work_type, event_id,
			face_verified, confidence_score, face_method
		) VALUES (
			$1, $2, CURRENT_DATE,
			NOW(), $3, $4,
			$5, 'PRESENT', 'qr_event',
			'EVENT', $6,
			true, $7, 'insightface'
		)
		RETURNING id
	`
	err = tx.QueryRow(query, employeeID, branchVal, lat, lon,
		distanceMeter, eventID, score,
	).Scan(&attendanceID)
	return
}

func (r *Repository) InsertEventFaceToken(employeeID, eventID int, token string) error {
	_, err := r.DB.Exec(`
		INSERT INTO face_tokens (employee_id, token, expires_at, is_used, event_id)
		VALUES ($1, $2, NOW() + INTERVAL '2 minutes', false, $3)
	`, employeeID, token, eventID)
	return err
}

func (r *Repository) IsEventParticipant(eventID, employeeID int) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM event_participants
		WHERE event_id = $1 AND employee_id = $2
	`, eventID, employeeID).Scan(&count)
	return count > 0, err
}

func (r *Repository) GetActiveEventsForEmployee(employeeID int) ([]EventListItem, error) {
	rows, err := r.DB.Query(`
		SELECT e.id, e.name, COALESCE(e. location, ''), e.date,
		COALESCE(TO_CHAR(e.start_time, 'HH24:MI'), ''),
		       COALESCE(TO_CHAR(e.end_time, 'HH24:MI'), ''),
		       EXISTS (
		           SELECT 1 FROM attendance a
		           WHERE a.employee_id = $1 AND a.event_id = e.id AND a.checkin_type = 'qr_event'
		       )
		FROM events e
		INNER JOIN event_participants ep ON ep.event_id = e.id AND ep.employee_id = $1
		WHERE e.date = CURRENT_DATE
		  AND e.expires_at > NOW()
		ORDER BY e.start_time NULLS LAST
	`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EventListItem
	for rows.Next() {
		var item EventListItem
		if err := rows.Scan(
			&item.EventID, &item.Name, &item.Location, &item.Date,
			&item.StartTime, &item.EndTime, &item.AlreadyCheckedIn,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
