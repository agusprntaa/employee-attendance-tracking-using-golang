package employee

import (
	"database/sql"
	"fmt"
	"strings"
)

type Repository struct {
	DB *sql.DB
}

// ─────────────────────────────────────────
// GetProfile
// Ambil profil karyawan yang sedang login
// JOIN ke divisions dan branches untuk dapat nama divisi & cabang
// ─────────────────────────────────────────
func (r *Repository) GetProfile(employeeID int) (*ProfileResponse, error) {
	var p ProfileResponse

	err := r.DB.QueryRow(`
		SELECT
			e.id,
			e.name,
			e.username,
			e.role,
			e.tipe,
			COALESCE(d.name, '-') AS division_name,
			COALESCE(b.name, '-') AS branch_name
		FROM employees e
		LEFT JOIN divisions d ON d.id = e.division_id
		LEFT JOIN branches b  ON b.id = e.branch_id
		WHERE e.id = $1
	`, employeeID).Scan(
		&p.ID,
		&p.Name,
		&p.Username,
		&p.Role,
		&p.Tipe,
		&p.DivisionName,
		&p.BranchName,
	)

	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ─────────────────────────────────────────
// GetPasswordHash
// Ambil hash password untuk verifikasi saat change password
// ─────────────────────────────────────────
func (r *Repository) GetPasswordHash(employeeID int) (string, error) {
	var hash string
	err := r.DB.QueryRow(`
		SELECT password FROM employees WHERE id = $1
	`, employeeID).Scan(&hash)
	return hash, err
}

// ─────────────────────────────────────────
// UpdatePassword
// Update password setelah verifikasi berhasil
// ─────────────────────────────────────────
func (r *Repository) UpdatePassword(employeeID int, newHash string) error {
	_, err := r.DB.Exec(`
		UPDATE employees SET password = $1 WHERE id = $2
	`, newHash, employeeID)
	return err
}

// ─────────────────────────────────────────
// GetEmployeeList
// Untuk admin — list semua karyawan dengan:
// - search by name atau username
// - pagination (limit + offset)
// - filter by branch_id (admin cabang hanya lihat cabangnya)
//
// Logika query builder:
// WHERE selalu mulai dengan "1=1" supaya bisa tambah AND kapan saja
// args menyimpan nilai parameter, argsCount tracking nomor $1, $2, dst
// ─────────────────────────────────────────
func (r *Repository) GetEmployeeList(branchID int, search string, page, limit int) ([]*ProfileResponse, int, error) {
	offset := (page - 1) * limit

	// Query builder dinamis
	// "1=1" adalah trick supaya bisa tambah AND tanpa cek apakah WHERE sudah ada
	where := "WHERE 1=1"
	args := []interface{}{}
	argsCount := 1

	// Filter branch — admin cabang hanya bisa lihat karyawan di cabangnya
	// branchID = 0 artinya super_admin, tidak perlu filter
	if branchID > 0 {
		where += fmt.Sprintf(" AND e.branch_id = $%d", argsCount)
		args = append(args, branchID)
		argsCount++
	}

	// Search by name atau username (case-insensitive dengan ILIKE)
	// ILIKE = PostgreSQL version of LIKE yang ignore case
	// % = wildcard, artinya "mengandung" bukan "sama persis"
	if strings.TrimSpace(search) != "" {
		where += fmt.Sprintf(" AND (e.name ILIKE $%d OR e.username ILIKE $%d)", argsCount, argsCount+1)
		searchPattern := "%" + strings.TrimSpace(search) + "%"
		args = append(args, searchPattern, searchPattern)
		argsCount += 2
	}

	// Query 1: hitung total record (untuk FE tahu total halaman)
	var total int
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM employees e %s
	`, where)
	err := r.DB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Query 2: ambil data dengan LIMIT dan OFFSET
	dataQuery := fmt.Sprintf(`
		SELECT
			e.id,
			e.name,
			e.username,
			e.role,
			e.tipe,
			COALESCE(d.name, '-') AS division_name,
			COALESCE(b.name, '-') AS branch_name
		FROM employees e
		LEFT JOIN divisions d ON d.id = e.division_id
		LEFT JOIN branches b  ON b.id = e.branch_id
		%s
		ORDER BY e.id ASC
		LIMIT $%d OFFSET $%d
	`, where, argsCount, argsCount+1)

	args = append(args, limit, offset)

	rows, err := r.DB.Query(dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var employees []*ProfileResponse
	for rows.Next() {
		var e ProfileResponse
		if err := rows.Scan(
			&e.ID, &e.Name, &e.Username,
			&e.Role, &e.Tipe,
			&e.DivisionName, &e.BranchName,
		); err != nil {
			return nil, 0, err
		}
		employees = append(employees, &e)
	}

	return employees, total, nil
}
