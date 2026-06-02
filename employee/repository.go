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
	var photoURL sql.NullString
	var email sql.NullString
	var phone sql.NullString
	var address sql.NullString
	var birthDate sql.NullString

	err := r.DB.QueryRow(`
		SELECT
			e.id,
			e.name,
			e.username,
			e.role,
			e.tipe,
			COALESCE(e.status, 'active') AS status,
			COALESCE(d.name, '-') AS division_name,
			COALESCE(b.name, '-') AS branch_name,
			e.photo_url,
			e.email,
			e.phone,
			e.address,
			TO_CHAR(e.birth_date, 'YYYY-MM-DD') AS birth_date
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
		&p.Status,
		&p.DivisionName,
		&p.BranchName,
		&photoURL,
		&email,
		&phone,
		&address,
		&birthDate,
	)
	if err != nil {
		return nil, err
	}

	if photoURL.Valid {
		p.PhotoURL = photoURL.String
	}
	if email.Valid {
		p.Email = email.String
	}
	if phone.Valid {
		p.Phone = phone.String
	}
	if address.Valid {
		p.Address = address.String
	}
	if birthDate.Valid {
		p.BirthDate = birthDate.String
	}

	return &p, nil
}

func (r *Repository) UpdateProfile(employeeID int, req UpdateProfileRequest) error {
	// Validasi birth_date — kalau kosong simpan NULL, kalau diisi harus format YYYY-MM-DD
	var birthDateVal interface{}
	if strings.TrimSpace(req.BirthDate) == "" {
		birthDateVal = nil
	} else {
		birthDateVal = req.BirthDate
	}

	_, err := r.DB.Exec(`
		UPDATE employees
		SET
			name       = $1,
			email      = NULLIF($2, ''),
			phone      = NULLIF($3, ''),
			address    = NULLIF($4, ''),
			birth_date = $5
		WHERE id = $6
	`,
		strings.TrimSpace(req.Name),
		strings.TrimSpace(req.Email),
		strings.TrimSpace(req.Phone),
		strings.TrimSpace(req.Address),
		birthDateVal,
		employeeID,
	)
	return err
}

// ─────────────────────────────────────────
// GetPasswordHash — tidak berubah
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
// F5: UpdatePhotoURL — simpan path foto ke DB
// ─────────────────────────────────────────

func (r *Repository) UpdatePhotoURL(employeeID int, photoURL string) error {
	_, err := r.DB.Exec(`
		UPDATE employees SET photo_url = $1 WHERE id = $2
	`, photoURL, employeeID)
	return err
}

// ─────────────────────────────────────────
// F5: GetPhotoURL — ambil path foto lama
// Dipakai sebelum upload baru / delete,
// supaya file lama bisa dihapus dari disk
// ─────────────────────────────────────────

func (r *Repository) GetPhotoURL(employeeID int) (string, error) {
	var photoURL sql.NullString
	err := r.DB.QueryRow(`
		SELECT photo_url FROM employees WHERE id = $1
	`, employeeID).Scan(&photoURL)
	if err != nil {
		return "", err
	}
	if photoURL.Valid {
		return photoURL.String, nil
	}
	return "", nil
}

// ─────────────────────────────────────────
// F5: ClearPhotoURL — hapus path foto dari DB
// Dipanggil saat karyawan hapus foto profil
// ─────────────────────────────────────────

func (r *Repository) ClearPhotoURL(employeeID int) error {
	_, err := r.DB.Exec(`
		UPDATE employees SET photo_url = NULL WHERE id = $1
	`, employeeID)
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

// di employee/repository.go — tambah fungsi ini
func (r *Repository) GetOnboardingFlags(employeeID int) (mustChange, faceRegistered, profileCompleted bool, err error) {
	err = r.DB.QueryRow(`
        SELECT
            COALESCE(must_change_password, false),
            COALESCE(face_registered, false),
            COALESCE(profile_completed, false)
        FROM employees
        WHERE id = $1
    `, employeeID).Scan(&mustChange, &faceRegistered, &profileCompleted)
	return
}

// employee/repository.go
func (r *Repository) MarkProfileCompleted(employeeID int) error {
	_, err := r.DB.Exec(`
        UPDATE employees SET profile_completed = true WHERE id = $1
    `, employeeID)
	return err
}
