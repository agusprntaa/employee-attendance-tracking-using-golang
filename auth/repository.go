package auth

import (
	"absensi_karyawan/models"
	"absensi_karyawan/utils"
	"database/sql"
	"errors"
	"time"
)

type Repository struct {
	DB *sql.DB
}

// ============================================================
// FindUser — tidak berubah dari sebelumnya
// ============================================================

func (r *Repository) FindUser(username string) (*User, string, error) {
	var user User
	var hashed string
	var branchID sql.NullInt64 // ← handle NULL

	err := r.DB.QueryRow(`
		SELECT id, name, username, role, tipe, branch_id, password
		FROM employees
		WHERE username = $1
	`, username).Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Role,
		&user.EmployeeType,
		&branchID, // ← scan ke NullInt64
		&hashed,
	)
	if err != nil {
		return nil, "", err
	}

	if branchID.Valid {
		user.BranchID = int(branchID.Int64)
	}

	return &user, hashed, nil
}

// ============================================================
// FindUserWithPasswordFlag — sama seperti FindUser tapi juga
// ambil must_change_password untuk keperluan F4
// ============================================================

func (r *Repository) FindUserWithPasswordFlag(username string) (*User, string, bool, error) {
	var user User
	var hashed string
	var mustChange bool
	var branchID sql.NullInt64 // ← handle NULL

	err := r.DB.QueryRow(`
		SELECT id, name, username, role, tipe, branch_id, password,
		       COALESCE(must_change_password, false)
		FROM employees
		WHERE username = $1
	`, username).Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Role,
		&user.EmployeeType,
		&branchID, // ← scan ke NullInt64
		&hashed,
		&mustChange,
	)
	if err != nil {
		return nil, "", false, err
	}

	if branchID.Valid {
		user.BranchID = int(branchID.Int64)
	}

	return &user, hashed, mustChange, nil
}

// ============================================================
// SaveRefreshToken
//
// PERUBAHAN dari versi lama:
//   - token yang disimpan ke DB adalah HASH bcrypt dari token asli
//   - kolom role, tipe, branch_id ditambahkan
//   - satu karyawan bisa punya lebih dari 1 refresh token
//     (multi-device). Kalau mau single-session, pakai UpsertRefreshToken.
// ============================================================

func (r *Repository) SaveRefreshToken(userID int, rawToken string, role, tipe string, branchID int, exp time.Time) error {
	// Hash token sebelum simpan ke DB
	hashed, err := utils.HashToken(rawToken)
	if err != nil {
		return err
	}

	_, err = r.DB.Exec(`
		INSERT INTO refresh_tokens (employee_id, token, role, tipe, branch_id, expires_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), $6)
	`, userID, hashed, role, tipe, branchID, exp)
	return err
}

// UpsertRefreshToken — pakai ini kalau ingin single-session per user
// (login baru akan hapus token lama)
func (r *Repository) UpsertRefreshToken(userID int, rawToken string, role, tipe string, branchID int, exp time.Time) error {
	// Hapus token lama milik user ini dulu
	_, err := r.DB.Exec(`DELETE FROM refresh_tokens WHERE employee_id = $1`, userID)
	if err != nil {
		return err
	}

	return r.SaveRefreshToken(userID, rawToken, role, tipe, branchID, exp)
}

// ============================================================
// ValidateRefreshToken
//
// PERUBAHAN dari versi lama:
//   - Tidak bisa lagi query langsung WHERE token = $1 karena
//     yang di DB adalah hash, bukan plain text.
//   - Strategi: ambil semua token aktif milik user dari JWT payload,
//     lalu compare satu per satu pakai bcrypt.
//
// Flow:
//   1. Service decode JWT refresh token → dapat user_id
//   2. Panggil FindActiveTokensByUser(userID) → dapat list hash
//   3. Loop compare rawToken dengan setiap hash
//   4. Kalau ada yang cocok → valid, return data user
// ============================================================

// TokenRow — baris dari tabel refresh_tokens
type TokenRow struct {
	ID          int
	HashedToken string
	Role        string
	Tipe        string
	BranchID    int
}

// FindActiveTokensByUser — ambil semua RT aktif milik user
func (r *Repository) FindActiveTokensByUser(userID int) ([]TokenRow, error) {
	rows, err := r.DB.Query(`
		SELECT id, token, role, tipe, COALESCE(branch_id, 0)
		FROM refresh_tokens
		WHERE employee_id = $1 AND expires_at > NOW()
		ORDER BY expires_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TokenRow
	for rows.Next() {
		var row TokenRow
		if err := rows.Scan(&row.ID, &row.HashedToken, &row.Role, &row.Tipe, &row.BranchID); err != nil {
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

// ValidateAndGetToken — cari token yang cocok dari list aktif
// Return: role, tipe, branchID, error
func (r *Repository) ValidateAndGetToken(userID int, rawToken string) (string, string, int, error) {
	rows, err := r.FindActiveTokensByUser(userID)
	if err != nil || len(rows) == 0 {
		return "", "", 0, errors.New("no active session found")
	}

	for _, row := range rows {
		if utils.CheckToken(rawToken, row.HashedToken) {
			return row.Role, row.Tipe, row.BranchID, nil
		}
	}

	return "", "", 0, errors.New("refresh token not found or expired")
}

// ============================================================
// DeleteRefreshToken — logout
// Hapus berdasarkan match hash, bukan plain token
// ============================================================

func (r *Repository) DeleteRefreshToken(rawToken string, userID int) {
	// Ambil semua token aktif user, cari yang cocok, hapus by ID
	rows, err := r.FindActiveTokensByUser(userID)
	if err != nil {
		return
	}

	for _, row := range rows {
		if utils.CheckToken(rawToken, row.HashedToken) {
			r.DB.Exec(`DELETE FROM refresh_tokens WHERE id = $1`, row.ID)
			return
		}
	}
}

// DeleteAllRefreshTokens — logout semua device / paksa re-login
// Dipakai saat ganti password
func (r *Repository) DeleteAllRefreshTokens(userID int) {
	r.DB.Exec(`DELETE FROM refresh_tokens WHERE employee_id = $1`, userID)
}

// ============================================================
// CreateUser — tambah must_change_password = true
// Dipakai oleh super_admin / admin_cabang saat buat akun baru (F4)
// ============================================================

func (r *Repository) CreateUser(username, password, name, role, tipe string, branchID, divisionID int) error {
	_, err := r.DB.Exec(`
		INSERT INTO employees (
			username, password, name, role, tipe,
			branch_id, division_id,
			must_change_password
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, 0), NULLIF($7, 0), true)
	`, username, password, name, role, tipe, branchID, divisionID)
	return err
}

// ============================================================
// UsernameExists — tidak berubah
// ============================================================

func (r *Repository) UsernameExists(username string) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM employees WHERE username = $1
	`, username).Scan(&count)
	return count > 0, err
}

// ============================================================
// ClearMustChangePassword — dipanggil setelah karyawan
// berhasil ganti password pertama kali (F4)
// ============================================================

func (r *Repository) ClearMustChangePassword(userID int) error {
	_, err := r.DB.Exec(`
		UPDATE employees
		SET must_change_password = false
		WHERE id = $1
	`, userID)
	return err
}

// ADD BRANCH ADMIN
func (r *Repository) FindBranchAdmin(username string) (*User, string, error) {
	var user User
	var hashed string

	err := r.DB.QueryRow(`
		SELECT id, name, username, role, branch_id, password
		FROM employees
		WHERE username = $1 AND role = 'branch_admin' AND status = 'active'
	`, username).Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Role,
		&user.BranchID,
		&hashed,
	)

	if err != nil {
		return nil, "", err
	}

	user.EmployeeType = "admin"
	return &user, hashed, nil
}

func (r *Repository) CreateBranchAdmin(branchID int, username, passwordHash, name string) error {
	_, err := r.DB.Exec(`
		INSERT INTO employees (username, password, name, role, branch_id, status, tipe)
		VALUES ($1, $2, $3, $4, $5, 'active', 'admin')
	`, username, passwordHash, name, "branch_admin", branchID)
	return err
}

func (r *Repository) CheckBranchAdminUsernameExists(username string) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM employees WHERE username = $1 AND role = 'branch_admin'
	`, username).Scan(&count)
	return count > 0, err
}

func (r *Repository) GetBranchAdmins(branchID int) ([]models.EmployeeDetail, error) {
	rows, err := r.DB.Query(`
		SELECT 
			e.id,
			e.username,
			COALESCE(e.name, '') AS name,
			e.role,
			COALESCE(e.status, 'active') AS status,
			e.created_at,
			e.branch_id,
			COALESCE(b.name, '') AS branch_name
		FROM employees e
		LEFT JOIN branches b ON e.branch_id = b.id
		WHERE e.branch_id = $1 AND e.role = 'branch_admin'
		ORDER BY e.created_at DESC
	`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []models.EmployeeDetail
	for rows.Next() {
		var admin models.EmployeeDetail
		err := rows.Scan(
			&admin.ID,
			&admin.Username,
			&admin.FullName,
			&admin.Role,
			&admin.Status,
			&admin.CreatedAt,
			&admin.BranchID,
			&admin.BranchName,
		)
		if err != nil {
			return nil, err
		}
		admins = append(admins, admin)
	}
	return admins, nil
}

func (r *Repository) DeleteBranchAdmin(adminID int) error {
	_, err := r.DB.Exec(`DELETE FROM employees WHERE id = $1 AND role = 'branch_admin'`, adminID)
	return err
}
