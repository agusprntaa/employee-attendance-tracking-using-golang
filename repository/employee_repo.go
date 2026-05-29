package repository

import (
	"absensi_karyawan/models"
	"absensi_karyawan/utils"
	"database/sql"
	"fmt"
	"log"
)

type EmployeeRepo struct {
	db *sql.DB
}

func NewEmployeeRepo(db *sql.DB) *EmployeeRepo {
	return &EmployeeRepo{db: db}
}

func (r *EmployeeRepo) ListByBranch(branchID, page, limit int, search, status string) ([]models.EmployeeDetail, int, error) {
	offset := utils.Offset(page, limit)

	where := "WHERE e.branch_id = $1 AND e.role = 'karyawan'"
	args := []interface{}{branchID}
	argIdx := 2

	if search != "" {
		where += fmt.Sprintf(" AND (e.username ILIKE $%d OR e.name ILIKE $%d)", argIdx, argIdx+1)
		args = append(args, "%"+search+"%", "%"+search+"%")
		argIdx += 2
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM employees e %s`, where)
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT 
			e.id,
			e.username,
			COALESCE(e.name, '') AS name,
			e.role,
			e.tipe,
			COALESCE(e.status, 'active') AS status,
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
			&emp.FullName,
			&emp.Role,
			&emp.Tipe,
			&emp.Status,
			&emp.DivisionID,
			&emp.DivisionName,
			&emp.BranchID,
			&emp.BranchName,
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
			e.id,
			COALESCE(e.name, '') AS full_name,
			e.username,
			COALESCE(e.email, '') AS email,
			COALESCE(e.phone, '') AS phone,
			COALESCE(e.address, '') AS address,
			COALESCE(e.birth_date::text, '') AS birth_date,
			COALESCE(e.photo_url, '') AS photo_url,

			e.role,
			e.tipe,
			COALESCE(e.status, 'active') AS status,

			e.division_id,
			COALESCE(d.name, '') AS division_name,

			e.branch_id,
			COALESCE(b.name, '') AS branch_name,

			e.created_at

		FROM employees e
		LEFT JOIN divisions d ON e.division_id = d.id
		LEFT JOIN branches b ON e.branch_id = b.id

		WHERE e.id = $1
		AND e.branch_id = $2
	`

	var emp models.EmployeeDetail

	err := r.db.QueryRow(query, id, branchID).Scan(
		&emp.ID,
		&emp.FullName,
		&emp.Username,
		&emp.Email,
		&emp.Phone,
		&emp.Address,
		&emp.BirthDate,
		&emp.PhotoURL,

		&emp.Role,
		&emp.Tipe,
		&emp.Status,

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

	return &emp, nil
}

func (r *EmployeeRepo) UsernameExists(username string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM employees WHERE username = $1`, username).Scan(&count)
	return count > 0, err
}

func (r *EmployeeRepo) Create(req *models.CreateEmployeeRequest, hashedPassword string, branchID int) (int, error) {
	var id int

	err := r.db.QueryRow(`
		INSERT INTO employees (
			username,
			password,
			name,
			role,
			tipe,
			status,
			division_id,
			branch_id,
			must_change_password
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, true
		)
		RETURNING id
	`,
		req.Username,   // $1
		hashedPassword, // $2
		req.Name,       // $3
		"karyawan",     // $4
		"cabang",       // $5
		"active",       // $6
		req.DivisionID, // $7
		branchID,       // $8
	).Scan(&id)

	return id, err
}

// Update — fix: tambah name di SET
// Sebelum: hanya update role dan division_id
// Sesudah: update name, role, division_id
func (r *EmployeeRepo) Update(id, branchID int, req *models.UpdateEmployeeRequest) error {

	result, err := r.db.Exec(`
		UPDATE employees 
		SET 
			username = $1,
			name = $2,
			role = $3,
			tipe = $4,
			division_id = $5,
			status = $6
		WHERE id = $7 AND branch_id = $8
	`,
		req.Username,
		req.Name,
		req.Role,
		req.Tipe,
		req.DivisionID,
		req.Status,
		id,
		branchID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("karyawan tidak ditemukan")
	}

	return nil
}

func (r *EmployeeRepo) Delete(id, branchID int) error {
	result, err := r.db.Exec(`
		DELETE FROM employees WHERE id = $1 AND branch_id = $2
	`, id, branchID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("karyawan tidak ditemukan di cabang ini")
	}

	return nil
}

func (r *EmployeeRepo) Activate(id, branchID int) error {
	result, err := r.db.Exec(`
		UPDATE employees
		SET status = 'active'
		WHERE id = $1 AND branch_id = $2
	`, id, branchID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("karyawan tidak ditemukan")
	}

	return nil
}

func (r *EmployeeRepo) Deactivate(id, branchID int) error {
	result, err := r.db.Exec(`
		UPDATE employees
		SET status = 'inactive'
		WHERE id = $1 AND branch_id = $2
	`, id, branchID)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	log.Printf("DEACTIVATE -> ID=%d BRANCH=%d ROWS=%d", id, branchID, rows)

	if rows == 0 {
		return fmt.Errorf("karyawan tidak ditemukan")
	}

	return nil
}
