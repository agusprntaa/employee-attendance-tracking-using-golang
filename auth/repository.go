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

// ValidateRefreshToken — return 4 nilai: userID, role, branchID, error
func (r *Repository) ValidateRefreshToken(token string) (int, string, int, error) {
	var userID int
	var role string
	var branchID int

	err := r.DB.QueryRow(`
		SELECT rt.employee_id, e.role, COALESCE(e.branch_id, 0)
		FROM refresh_tokens rt
		JOIN employees e ON e.id = rt.employee_id
		WHERE rt.token = $1 AND rt.expires_at > NOW()
	`, token).Scan(&userID, &role, &branchID)

	return userID, role, branchID, err
}

func (r *Repository) DeleteRefreshToken(token string) {
	r.DB.Exec(`DELETE FROM refresh_tokens WHERE token = $1`, token)
}

func (r *Repository) CreateUser(username, password, name, role, tipe string) error {
	_, err := r.DB.Exec(`
		INSERT INTO employees (username, password, name, role, tipe)
		VALUES ($1, $2, $3, $4, $5)
	`, username, password, name, role, tipe)
	return err
}
