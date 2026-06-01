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
func (r *Repository) GetFaceReference(employeeID int) (path string, registered bool, err error) {
	var nullPath sql.NullString
	err = r.DB.QueryRow(`
		SELECT
			COALESCE(face_reference_path, ''),
			COALESCE(face_registered, false)
		FROM employees
		WHERE id = $1
	`, employeeID).Scan(&nullPath, &registered)
	if nullPath.Valid {
		path = nullPath.String
	}
	return
}

// UpdateFaceReference — simpan path foto + tandai face_registered = true
// Dipanggil setelah foto berhasil disimpan ke disk
func (r *Repository) UpdateFaceReference(employeeID int, path string) error {
	_, err := r.DB.Exec(`
		UPDATE employees
		SET
			face_reference_path = $1,
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
		SELECT id, employee_id, token, expires_at, is_used
		FROM face_tokens
		WHERE token = $1 AND employee_id = $2
	`, token, employeeID).Scan(
		&ft.ID, &ft.EmployeeID, &ft.Token, &ft.ExpiresAt, &ft.IsUsed,
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
		INSERT INTO attendances
			(employee_id, date, checkin_time, face_verified, confidence_score)
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
		SELECT COUNT(*) FROM attendances
		WHERE employee_id = $1 AND date = CURRENT_DATE
	`, employeeID).Scan(&count)
	return count > 0, err
}
