package repository

import (
	"absensi_karyawan/models"
	"absensi_karyawan/utils"
	"database/sql"
	"fmt"
)

type EmployeeRepo struct {
	db *sql.DB
}

func NewEmployeeRepo(db *sql.DB) *EmployeeRepo {
	return &EmployeeRepo{db: db}
}

// ListByBranch — list karyawan per cabang dengan search + pagination
// Fix: tambah kolom name di SELECT
func (r *EmployeeRepo) ListByBranch(branchID, page, limit int, search, status string) ([]models.EmployeeDetail, int, error) {
	offset := utils.Offset(page, limit)

	where := "WHERE e.branch_id = $1"
	args := []interface{}{branchID}
	argIdx := 2

	if search != "" {
		// Search by username ATAU name
		where += fmt.Sprintf(" AND (e.username ILIKE $%d OR e.name ILIKE $%d)", argIdx, argIdx+1)
		args = append(args, "%"+search+"%", "%"+search+"%")
		argIdx += 2
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM employees e %s`, where)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fix: tambah e.name di SELECT dan di Scan
	query := fmt.Sprintf(`
		SELECT 
			e.id,
			e.username,
			COALESCE(e.name, '') AS name,
			e.role,
			e.tipe,
			e.division_id,
			COALESCE(d.name, '') AS division_name,
			e.branch_id,
			COALESCE(b.name, '') AS branch_name,
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
			&emp.ID,
			&emp.Username,
			&emp.Name, // tambah name
			&emp.Role,
			&emp.Tipe,
			&emp.DivisionID,
			&emp.DivisionName,
			&emp.BranchID,
			&emp.BranchName,
			&emp.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		// Set status aktif secara default (kolom status tidak ada di DB)
		emp.Status = "Active"
		employees = append(employees, emp)
	}
	return employees, total, nil
}

func (r *EmployeeRepo) GetByID(id, branchID int) (*models.EmployeeDetail, error) {
	query := `
		SELECT 
			e.id,
			e.username,
			COALESCE(e.name, '') AS name,
			e.role,
			e.tipe,
			e.division_id,
			COALESCE(d.name, '') AS division_name,
			e.branch_id,
			COALESCE(b.name, '') AS branch_name,
			e.created_at
		FROM employees e
		LEFT JOIN divisions d ON e.division_id = d.id
		LEFT JOIN branches b ON e.branch_id = b.id
		WHERE e.id = $1 AND e.branch_id = $2
	`
	var emp models.EmployeeDetail
	err := r.db.QueryRow(query, id, branchID).Scan(
		&emp.ID,
		&emp.Username,
		&emp.Name,
		&emp.Role,
		&emp.Tipe,
		&emp.DivisionID,
		&emp.DivisionName,
		&emp.BranchID,
		&emp.BranchName,
		&emp.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	emp.Status = "Active"
	return &emp, nil
}

func (r *EmployeeRepo) UsernameExists(username string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM employees WHERE username = $1`, username).Scan(&count)
	return count > 0, err
}

// Create — INSERT dengan kolom name
// Fix: tambah name di INSERT, pakai bcrypt bukan SHA256
func (r *EmployeeRepo) Create(req *models.CreateEmployeeRequest, hashedPassword string, branchID int) (int, error) {
	var id int
	err := r.db.QueryRow(`
		INSERT INTO employees (username, password, name, role, tipe, division_id, branch_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`,
		req.Username,
		hashedPassword,
		req.Name, // tambah name
		req.Role,
		req.Tipe,
		req.DivisionID,
		branchID,
	).Scan(&id)
	return id, err
}

func (r *EmployeeRepo) Update(id, branchID int, req *models.UpdateEmployeeRequest) error {
	_, err := r.db.Exec(`
		UPDATE employees 
		SET role = $1, division_id = $2
		WHERE id = $3 AND branch_id = $4
	`, req.Role, req.DivisionID, id, branchID)
	return err
}

func (r *EmployeeRepo) Delete(id, branchID int) error {
	_, err := r.db.Exec(`
		DELETE FROM employees WHERE id = $1 AND branch_id = $2
	`, id, branchID)
	return err
}

func (r *EmployeeRepo) Deactivate(id, branchID int) error {
	return r.Delete(id, branchID)
}
