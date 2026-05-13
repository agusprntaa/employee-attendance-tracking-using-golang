package auth

import (
	"database/sql"
	"time"
)

type Repository struct {
	DB *sql.DB
}

// FindUser
func (r *Repository) FindUser(username string) (*User, string, error) {
	var user User
	var hashed string

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
		&user.BranchID,
		&hashed,
	)

	if err != nil {
		return nil, "", err
	}

	return &user, hashed, nil
}

func (r *Repository) SaveRefreshToken(userID int, token string, exp time.Time) error {
	_, err := r.DB.Exec(`
		INSERT INTO refresh_tokens (employee_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, userID, token, exp)
	return err
}

// ValidateRefreshToken — return 4 nilai: userID, role, tipe, branchID, error
func (r *Repository) ValidateRefreshToken(token string) (int, string, string, int, error) {
	var userID, branchID int
	var role, tipe string

	err := r.DB.QueryRow(`
		SELECT rt.employee_id, e.role, e.tipe, COALESCE(e.branch_id, 0)
		FROM refresh_tokens rt
		JOIN employees e ON e.id = rt.employee_id
		WHERE rt.token = $1 AND rt.expires_at > NOW()
	`, token).Scan(&userID, &role, &tipe, &branchID)

	return userID, role, tipe, branchID, err
}

func (r *Repository) DeleteRefreshToken(token string) {
	r.DB.Exec(`DELETE FROM refresh_tokens WHERE token = $1`, token)
}

func (r *Repository) CreateUser(username, password, name, role, tipe string, branchID, divisionID int) error {
	_, err := r.DB.Exec(`
		INSERT INTO employees (username, password, name, role, tipe, branch_id, division_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, 0), NULLIF($7, 0))
	`, username, password, name, role, tipe, branchID, divisionID)
	return err
}

// ─────────────────────────────────────────────────────────────
// UsernameExists — cek apakah username sudah dipakai
// Dipanggil di service sebelum insert user baru
// ─────────────────────────────────────────────────────────────
func (r *Repository) UsernameExists(username string) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM employees WHERE username = $1
	`, username).Scan(&count)
	return count > 0, err
}
