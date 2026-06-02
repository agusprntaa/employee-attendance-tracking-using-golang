package face

import (
	"database/sql"
	"time"
)

type Repository struct {
	DB *sql.DB
}

// ─────────────────────────────────────────
// Struct internal
// ─────────────────────────────────────────

type FaceToken struct {
	ID         int
	EmployeeID int
	Token      string
	ExpiresAt  time.Time
	IsUsed     bool

	FaceVerified    bool
	ConfidenceScore float64
	VerifiedAt      *time.Time
}

// ─────────────────────────────────────────
// FACE REFERENCE — baca/tulis dari kolom employees
//
// Tidak ada tabel employee_faces terpisah.
// face_reference_path dan face_registered hidup di tabel employees.
// Package face hanya baca/tulis dua kolom ini, sisanya urusan package lain.
// ─────────────────────────────────────────

// GetFaceReference — ambil path foto referensi dan status pendaftaran
// Return: path string, registered bool, err
// path kosong ("") berarti belum daftar wajah
// Hanya ambil apa yang face module butuhkan
func (r *Repository) GetFaceReference(employeeID int) (path string, registered bool, err error) {
	var nullPath sql.NullString
	err = r.DB.QueryRow(`
        SELECT COALESCE(face_reference_path, ''), COALESCE(face_registered, false)
        FROM employees WHERE id = $1
    `, employeeID).Scan(&nullPath, &registered)
	path = nullPath.String
	return
}

func (r *Repository) UpdateFaceReference(employeeID int, path string) error {
	_, err := r.DB.Exec(`
        UPDATE employees
        SET face_reference_path = $1,
            face_registered     = true,
            face_registered_at  = NOW()
        WHERE id = $2
    `, path, employeeID)
	return err
}

// ─────────────────────────────────────────
// FACE TOKEN
// ─────────────────────────────────────────

// InsertFaceToken — simpan token baru TTL 2 menit
// Trigger DB akan auto-cleanup token expired lama milik employee ini
func (r *Repository) InsertFaceToken(employeeID int, token string) error {
	_, err := r.DB.Exec(`
		INSERT INTO face_tokens (employee_id, token, expires_at, is_used)
		VALUES ($1, $2, NOW() + INTERVAL '2 minutes', false)
	`, employeeID, token)
	return err
}

// GetFaceToken — ambil token, validasi dilakukan di service
// Return nil jika token tidak ditemukan
func (r *Repository) GetFaceToken(token string, employeeID int) (*FaceToken, error) {

	var ft FaceToken

	err := r.DB.QueryRow(`
		SELECT
			id,
			employee_id,
			token,
			expires_at,
			is_used,
			face_verified,
			COALESCE(confidence_score,0)
		FROM face_tokens
		WHERE token = $1
		AND employee_id = $2
	`,
		token,
		employeeID,
	).Scan(
		&ft.ID,
		&ft.EmployeeID,
		&ft.Token,
		&ft.ExpiresAt,
		&ft.IsUsed,
		&ft.FaceVerified,
		&ft.ConfidenceScore,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &ft, nil
}

// MarkTokenUsed — invalidate token dalam transaksi DB
// Wajib dipanggil bersama InsertAttendanceWithFace dalam 1 tx
func (r *Repository) MarkTokenUsed(tx *sql.Tx, tokenID int) error {
	_, err := tx.Exec(`
		UPDATE face_tokens SET is_used = true WHERE id = $1
	`, tokenID)
	return err
}

// ─────────────────────────────────────────
// VERIFICATION LOG
// ─────────────────────────────────────────

// CountDailyFailures — hitung percobaan mismatch hari ini
// Dipakai sebelum proses gambar untuk cek MAX_ATTEMPT_EXCEEDED
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

// InsertVerificationLog — catat hasil verifikasi (selalu, berhasil maupun gagal)
// tx = nil → pakai DB langsung (kasus mismatch, tidak dalam transaksi)
// tx != nil → pakai transaksi (kasus match, bersama insert attendance)
func (r *Repository) InsertVerificationLog(
	tx *sql.Tx,
	employeeID, attendanceID int,
	result string,
	score float64,
	ipAddress string,
) error {
	var attendanceVal interface{}
	if attendanceID > 0 {
		attendanceVal = attendanceID
	}

	q := `
		INSERT INTO face_verification_logs
			(employee_id, attendance_id, result, confidence_score, ip_address)
		VALUES ($1, $2, $3, $4, $5)
	`
	if tx != nil {
		_, err := tx.Exec(q, employeeID, attendanceVal, result, score, ipAddress)
		return err
	}
	_, err := r.DB.Exec(q, employeeID, attendanceVal, result, score, ipAddress)
	return err
}

// ─────────────────────────────────────────
// ATTENDANCE (dalam transaksi)
// ─────────────────────────────────────────

// InsertAttendanceWithFace — INSERT attendance dengan face_verified=true
// Harus dalam transaksi bersama MarkTokenUsed dan InsertVerificationLog
func (r *Repository) InsertAttendanceWithFace(
	tx *sql.Tx,
	employeeID int,
	score float64,
) (int, error) {
	var id int
	err := tx.QueryRow(`
		INSERT INTO attendance
			(employee_id, date, check_in, face_verified, confidence_score)
		VALUES ($1, CURRENT_DATE, NOW(), true, $2)
		RETURNING id
	`, employeeID, score).Scan(&id)
	return id, err
}

// HasCheckedInToday — cek sudah checkin hari ini
// Dipanggil saat generate face-token untuk early return sebelum proses apapun
func (r *Repository) HasCheckedInToday(employeeID int) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM attendance
		WHERE employee_id = $1 AND date = CURRENT_DATE
	`, employeeID).Scan(&count)
	return count > 0, err
}

func (r *Repository) MarkFaceVerified(
	tokenID int,
	score float64,
) error {

	_, err := r.DB.Exec(`
		UPDATE face_tokens
		SET
			face_verified = TRUE,
			confidence_score = $2,
			verified_at = NOW()
		WHERE id = $1
	`,
		tokenID,
		score,
	)

	return err
}

func (r *Repository) IsFaceVerified(
	employeeID int,
	token string,
) (bool, error) {

	var verified bool

	err := r.DB.QueryRow(`
		SELECT face_verified
FROM face_tokens
WHERE employee_id = $1
AND token = $2
AND expires_at > NOW()
AND is_used = false
AND face_verified = true
	`, employeeID, token).Scan(
		&verified,
	)

	if err == sql.ErrNoRows {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return verified, nil
}

func (r *Repository) ConsumeFaceToken(
	token string,
) error {

	_, err := r.DB.Exec(`
		UPDATE face_tokens
		SET is_used = true
		WHERE token = $1
	`, token)

	return err
}
