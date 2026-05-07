package repository

import (
	"absensi/models"
	"absensi/utils"
	"database/sql"
	"fmt"
)

type EmployeeRepo struct {
	db *sql.DB
}

func NewEmployeeRepo(db *sql.DB) *EmployeeRepo {
	return &EmployeeRepo{db: db}
}

// ListByBranch - list karyawan milik cabang ini
// Kolom employees: id, username, role, tipe, division_id, branch_id, status, created_at
// TIDAK ADA kolom "name"
func (r *EmployeeRepo) ListByBranch(branchID, page, limit int, search, status string) ([]models.EmployeeDetail, int, error) {
	offset := utils.Offset(page, limit)

	where := "WHERE e.branch_id = $1"
	args := []interface{}{branchID}
	argIdx := 2

	if search != "" {
		where += fmt.Sprintf(" AND e.username ILIKE $%d", argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" && status != "all" {
		where += fmt.Sprintf(" AND e.status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM employees e %s`, where)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT 
			e.id, e.username, e.role, e.tipe, e.status,
			e.division_id, COALESCE(d.name, '') AS division_name,
			e.branch_id, COALESCE(b.name, '') AS branch_name,
			e.created_at
		FROM employees e
		LEFT JOIN divisions d ON e.division_id = d.id
		LEFT JOIN branches b ON e.branch_id = b.id
		%s
		ORDER BY e.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, limit, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var employees []models.EmployeeDetail
	for rows.Next() {
		var emp models.EmployeeDetail
		err := rows.Scan(
			&emp.ID, &emp.Username, &emp.Role, &emp.Tipe, &emp.Status,
			&emp.DivisionID, &emp.DivisionName,
			&emp.BranchID, &emp.BranchName,
			&emp.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		employees = append(employees, emp)
	}
	return employees, total, nil
}

func (r *EmployeeRepo) GetByID(id, branchID int) (*models.EmployeeDetail, error) {
	query := `
		SELECT 
			e.id, e.username, e.role, e.tipe, e.status,
			e.division_id, COALESCE(d.name, '') AS division_name,
			e.branch_id, COALESCE(b.name, '') AS branch_name,
			e.created_at
		FROM employees e
		LEFT JOIN divisions d ON e.division_id = d.id
		LEFT JOIN branches b ON e.branch_id = b.id
		WHERE e.id = $1 AND e.branch_id = $2
	`
	var emp models.EmployeeDetail
	err := r.db.QueryRow(query, id, branchID).Scan(
		&emp.ID, &emp.Username, &emp.Role, &emp.Tipe, &emp.Status,
		&emp.DivisionID, &emp.DivisionName,
		&emp.BranchID, &emp.BranchName,
		&emp.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &emp, err
}

func (r *EmployeeRepo) UsernameExists(username string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM employees WHERE username = $1`, username).Scan(&count)
	return count > 0, err
}

// Create - INSERT ke employees tanpa kolom "name" (tidak ada di DB)
func (r *EmployeeRepo) Create(req *models.CreateEmployeeRequest, hashedPassword string, branchID int) (int, error) {
	var id int
	err := r.db.QueryRow(`
		INSERT INTO employees (username, password, role, tipe, division_id, branch_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'active')
		RETURNING id
	`, req.Username, hashedPassword, req.Role, req.Tipe, req.DivisionID, branchID).Scan(&id)
	return id, err
}

// Update - UPDATE employees, hanya kolom yang ada di DB
func (r *EmployeeRepo) Update(id, branchID int, req *models.UpdateEmployeeRequest) error {
	_, err := r.db.Exec(`
		UPDATE employees 
		SET role = $1, status = $2, division_id = $3
		WHERE id = $4 AND branch_id = $5
	`, req.Role, req.Status, req.DivisionID, id, branchID)
	return err
}

func (r *EmployeeRepo) Deactivate(id, branchID int) error {
	_, err := r.db.Exec(`
		UPDATE employees SET status = 'inactive' 
		WHERE id = $1 AND branch_id = $2
	`, id, branchID)
	return err
}
