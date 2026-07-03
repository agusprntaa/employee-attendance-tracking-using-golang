package repository

import (
	"absensi_karyawan/attendance"
	"absensi_karyawan/models"
	"absensi_karyawan/utils"
	"database/sql"
	"fmt"
	"log"
	"time"
)

type GlobalAdminRepository struct {
	db *sql.DB
}

func NewGlobalAdminRepository(db *sql.DB) *GlobalAdminRepository {
	return &GlobalAdminRepository{db: db}
}

// ─────────────────────────────────────────────────────────────
// GetDashboardStats
// Ambil angka ringkasan: total karyawan, hadir hari ini,
// attendance rate, total cabang
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetDashboardStats() (map[string]interface{}, error) {
	today := utils.NowWITA()

	var totalEmployees, presentToday, totalBranches int
	var attendanceRate float64

	// Total karyawan aktif (semua cabang)
	r.db.QueryRow(`
		SELECT COUNT(*) FROM employees
		WHERE role = 'karyawan'
	`).Scan(&totalEmployees)

	// Hadir hari ini (PRESENT + LATE + WFA)
	r.db.QueryRow(`
		SELECT COUNT(*) FROM attendance
		WHERE date = $1 AND status IN ('PRESENT', 'LATE', 'WFA')
	`, today).Scan(&presentToday)

	// Total cabang
	r.db.QueryRow(`SELECT COUNT(*) FROM branches`).Scan(&totalBranches)

	// Attendance rate hari ini
	if totalEmployees > 0 {
		attendanceRate = float64(presentToday) / float64(totalEmployees) * 100
	}

	return map[string]interface{}{
		"total_employees": totalEmployees,
		"present_today":   presentToday,
		"attendance_rate": attendanceRate,
		"total_branches":  totalBranches,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// GetAttendancePerBranch
// Data bar chart: hadir per cabang hari ini
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetAttendancePerBranch() ([]map[string]interface{}, error) {
	today := utils.NowWITA()

	rows, err := r.db.Query(`
		SELECT
			b.name AS branch_name,
			COUNT(CASE WHEN a.status IN ('PRESENT','LATE','WFA') THEN 1 END) AS present,
			COUNT(e.id) AS capacity
		FROM branches b
		LEFT JOIN employees e ON e.branch_id = b.id AND e.role = 'karyawan'
		LEFT JOIN attendance a ON a.employee_id = e.id AND a.date = $1
		GROUP BY b.id, b.name
		ORDER BY b.name
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var branchName string
		var present, capacity int
		rows.Scan(&branchName, &present, &capacity)
		result = append(result, map[string]interface{}{
			"branch_name": branchName,
			"present":     present,
			"capacity":    capacity,
		})
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────
// GetWorkModeDistribution
// Pie chart: WFO vs WFA hari ini
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetWorkModeDistribution() (map[string]interface{}, error) {
	today := utils.NowWITA()

	var wfoCount, wfaCount int
	r.db.QueryRow(`
		SELECT
			COUNT(CASE WHEN work_type = 'WFO' THEN 1 END),
			COUNT(CASE WHEN work_type = 'WFA' THEN 1 END)
		FROM attendance WHERE date = $1
	`, today).Scan(&wfoCount, &wfaCount)

	total := wfoCount + wfaCount
	var wfoPercent, wfaPercent float64
	if total > 0 {
		wfoPercent = float64(wfoCount) / float64(total) * 100
		wfaPercent = float64(wfaCount) / float64(total) * 100
	}

	return map[string]interface{}{
		"wfo_count":   wfoCount,
		"wfa_count":   wfaCount,
		"wfo_percent": wfoPercent,
		"wfa_percent": wfaPercent,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// GetBranchPerformance
// Tabel performa per cabang hari ini
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetBranchPerformance() ([]map[string]interface{}, error) {
	today := utils.NowWITA()

	rows, err := r.db.Query(`
		SELECT
			b.name AS branch,
			COUNT(DISTINCT e.id) AS total_employees,
			COUNT(CASE WHEN a.status IN ('PRESENT','LATE') THEN 1 END) AS present,
			COUNT(CASE WHEN a.status = 'ABSENT' THEN 1 END) AS absent,
			COUNT(CASE WHEN a.work_type = 'WFO' THEN 1 END) AS wfo,
			COUNT(CASE WHEN a.work_type = 'WFA' THEN 1 END) AS wfa
		FROM branches b
		LEFT JOIN employees e ON e.branch_id = b.id AND e.role = 'karyawan'
		LEFT JOIN attendance a ON a.employee_id = e.id AND a.date = $1
		GROUP BY b.id, b.name
		ORDER BY b.name
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var branch string
		var totalEmployees, present, absent, wfo, wfa int
		rows.Scan(&branch, &totalEmployees, &present, &absent, &wfo, &wfa)

		var rate float64
		if totalEmployees > 0 {
			rate = float64(present) / float64(totalEmployees) * 100
		}

		status := "Good"
		if rate < 70 {
			status = "Critical"
		} else if rate < 85 {
			status = "Warning"
		}

		result = append(result, map[string]interface{}{
			"branch":          branch,
			"total_employees": totalEmployees,
			"present":         present,
			"absent":          absent,
			"rate":            rate,
			"wfo":             wfo,
			"wfa":             wfa,
			"status":          status,
		})
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────
// GetAllEmployees
// List semua karyawan semua cabang dengan filter + pagination
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetAllEmployees(page, perPage int, search, branch, status string) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * perPage

	where := "WHERE e.role = 'karyawan'"
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		where += fmt.Sprintf(" AND (e.name ILIKE $%d OR e.username ILIKE $%d)", argIdx, argIdx+1)
		args = append(args, "%"+search+"%", "%"+search+"%")
		argIdx += 2
	}
	if branch != "" {
		where += fmt.Sprintf(" AND b.name ILIKE $%d", argIdx)
		args = append(args, "%"+branch+"%")
		argIdx++
	}

	var total int
	countQ := fmt.Sprintf(`
		SELECT COUNT(*) FROM employees e
		LEFT JOIN branches b ON b.id = e.branch_id
		%s
	`, where)
	r.db.QueryRow(countQ, args...).Scan(&total)

	dataQ := fmt.Sprintf(`
		SELECT
			'EMP' || LPAD(e.id::text, 3, '0') AS employee_id,
			COALESCE(e.name, e.username) AS full_name,
			COALESCE(d.name, '-') AS position,
			COALESCE(b.name, '-') AS branch,
			e.created_at
		FROM employees e
		LEFT JOIN divisions d ON d.id = e.division_id
		LEFT JOIN branches b ON b.id = e.branch_id
		%s
		ORDER BY e.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, perPage, offset)
	rows, err := r.db.Query(dataQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var employeeID, fullName, position, branch2 string
		var createdAt time.Time
		rows.Scan(&employeeID, &fullName, &position, &branch2, &createdAt)
		result = append(result, map[string]interface{}{
			"employee_id":  employeeID,
			"full_name":    fullName,
			"position":     position,
			"branch":       branch2,
			"status":       "Active",
			"created_date": createdAt,
		})
	}
	return result, total, nil
}

// ─────────────────────────────────────────────────────────────
// GetEmployeeDetail
// Detail satu karyawan + statistik attendance
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetEmployeeDetail(employeeID int) (map[string]interface{}, error) {
	var id int
	var name, username, role, tipe, divName, branchName, branchAddr string

	err := r.db.QueryRow(`
		SELECT
			e.id,
			COALESCE(e.name, e.username),
			e.username, e.role, e.tipe,
			COALESCE(d.name, '-'),
			COALESCE(b.name, '-'),
			COALESCE(b.address, '-')
		FROM employees e
		LEFT JOIN divisions d ON d.id = e.division_id
		LEFT JOIN branches b ON b.id = e.branch_id
		WHERE e.id = $1
	`, employeeID).Scan(&id, &name, &username, &role, &tipe, &divName, &branchName, &branchAddr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("karyawan tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	// Statistik attendance
	var totalPresent, totalAbsent, totalLate int
	r.db.QueryRow(`
		SELECT
			COUNT(CASE WHEN status IN ('PRESENT','LATE','WFA') THEN 1 END),
			COUNT(CASE WHEN status = 'ABSENT' THEN 1 END),
			COUNT(CASE WHEN status = 'LATE' THEN 1 END)
		FROM attendance WHERE employee_id = $1
	`, employeeID).Scan(&totalPresent, &totalAbsent, &totalLate)

	total := totalPresent + totalAbsent
	var attendanceRate float64
	if total > 0 {
		attendanceRate = float64(totalPresent) / float64(total) * 100
	}

	return map[string]interface{}{
		"employee_id":     fmt.Sprintf("EMP%03d", id),
		"full_name":       name,
		"username":        username,
		"role":            role,
		"tipe":            tipe,
		"division":        divName,
		"branch":          branchName,
		"branch_address":  branchAddr,
		"status":          "Active",
		"total_present":   totalPresent,
		"total_absent":    totalAbsent,
		"total_late":      totalLate,
		"attendance_rate": attendanceRate,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// GetTodayAttendanceOverview
// Ringkasan kehadiran per cabang hari ini
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetTodayAttendanceOverview(
	page,
	perPage int,
	search string,
) (map[string]interface{}, error) {

	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(loc).Format("2006-01-02")

	offset := (page - 1) * perPage

	where := "WHERE 1=1"
	args := []interface{}{today}
	argIdx := 2

	if search != "" {
		where += fmt.Sprintf(" AND b.name ILIKE $%d", argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	// total branch
	var total int

	countQuery := fmt.Sprintf(`
		SELECT COUNT(DISTINCT b.id)
		FROM branches b
		LEFT JOIN employees e
			ON e.branch_id = b.id
			AND e.role = 'karyawan'
		LEFT JOIN attendance a
			ON a.employee_id = e.id
			AND a.date = $1
		%s
	`, where)

	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT
			b.id,
			b.name,

			COUNT(DISTINCT e.id) AS total_employees,

			COUNT(
				DISTINCT CASE
					WHEN a.status IN ('PRESENT','LATE','WFA')
					THEN a.employee_id
				END
			) AS present_today

		FROM branches b

		LEFT JOIN employees e
			ON e.branch_id = b.id
			AND e.role = 'karyawan'

		LEFT JOIN attendance a
			ON a.employee_id = e.id
			AND a.date = $1

		%s

		GROUP BY b.id, b.name
		ORDER BY b.name

		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	rows, err := r.db.Query(
		query,
		append(args, perPage, offset)...,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []map[string]interface{}

	totalEmployeesGlobal := 0
	totalPresentGlobal := 0

	for rows.Next() {

		var branchID int
		var branchName string
		var totalEmployees int
		var presentToday int

		err := rows.Scan(
			&branchID,
			&branchName,
			&totalEmployees,
			&presentToday,
		)

		if err != nil {
			return nil, err
		}

		absent := totalEmployees - presentToday

		var attendanceRate float64

		if totalEmployees > 0 {
			attendanceRate =
				(float64(presentToday) / float64(totalEmployees)) * 100
		}

		status := "Critical"

		if attendanceRate >= 90 {
			status = "Excellent"
		} else if attendanceRate >= 70 {
			status = "Good"
		}

		totalEmployeesGlobal += totalEmployees
		totalPresentGlobal += presentToday

		branches = append(branches, map[string]interface{}{
			"branch_id":       branchID,
			"branch_name":     branchName,
			"total_employees": totalEmployees,
			"present_today":   presentToday,
			"absent":          absent,
			"attendance_rate": attendanceRate,
			"status":          status,
		})
	}

	totalAbsentGlobal := totalEmployeesGlobal - totalPresentGlobal

	var overallRate float64

	if totalEmployeesGlobal > 0 {
		overallRate =
			(float64(totalPresentGlobal) /
				float64(totalEmployeesGlobal)) * 100
	}

	return map[string]interface{}{
		"total_present":   totalPresentGlobal,
		"total_absent":    totalAbsentGlobal,
		"total_employees": totalEmployeesGlobal,
		"attendance_rate": overallRate,

		"branches": branches,

		"pagination": map[string]interface{}{
			"page":        page,
			"per_page":    perPage,
			"total_data":  total,
			"total_pages": (total + perPage - 1) / perPage,
		},
	}, nil
}

// ─────────────────────────────────────────────────────────────
// GetAttendanceAnalytics
// Trend attendance weekly / monthly
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetAttendanceAnalytics(
	period string,
) (map[string]interface{}, error) {

	var query string

	// WEEKLY
	if period == "weekly" {

		query = `
			SELECT
				TO_CHAR(date, 'Dy') AS label,

				ROUND(
					COUNT(
						CASE
							WHEN status IN ('PRESENT','LATE','WFA')
							THEN 1
						END
					)::numeric
					/
					NULLIF(COUNT(*), 0)
					* 100,
					1
				) AS attendance_rate

			FROM attendance

			WHERE date >= CURRENT_DATE - INTERVAL '6 days'

			GROUP BY date
			ORDER BY date
		`

	} else {

		// MONTHLY
		query = `
			SELECT
				TO_CHAR(date, 'Mon YYYY') AS label,

				ROUND(
					COUNT(
						CASE
							WHEN status IN ('PRESENT','LATE','WFA')
							THEN 1
						END
					)::numeric
					/
					NULLIF(COUNT(*), 0)
					* 100,
					1
				) AS attendance_rate

			FROM attendance

			WHERE date >= CURRENT_DATE - INTERVAL '11 months'

			GROUP BY
				DATE_TRUNC('month', date),
				TO_CHAR(date, 'Mon YYYY')

			ORDER BY DATE_TRUNC('month', date)
		`
	}

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var analytics []map[string]interface{}

	for rows.Next() {

		var label string
		var attendanceRate float64

		err := rows.Scan(
			&label,
			&attendanceRate,
		)

		if err != nil {
			return nil, err
		}

		analytics = append(analytics, map[string]interface{}{
			"label":           label,
			"attendance_rate": attendanceRate,
		})
	}

	return map[string]interface{}{
		"period": period,
		"data":   analytics,
	}, nil
}

// ─────────────────────────────────────────────────────────────
// GetAllBranches
// List semua cabang dengan jumlah karyawan
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetAllBranches(page, perPage int, search, status string) ([]map[string]interface{}, int, error) {
	offset := (page - 1) * perPage

	where := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		where += fmt.Sprintf(" AND b.name ILIKE $%d", argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	var total int
	r.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM branches b %s`, where), args...).Scan(&total)

	rows, err := r.db.Query(fmt.Sprintf(`
	SELECT
		b.id,
		b.name AS branch_name,
		COALESCE(b.address, '-') AS address,
		COALESCE(b.radius_meter, 0) AS radius_meter,

		COUNT(e.id) AS total_employees,

		COALESCE(
			ROUND(
				COUNT(
					CASE
						WHEN a.status IN ('PRESENT','LATE','WFA')
						THEN 1
					END
				)::numeric
				/
				NULLIF(COUNT(a.id), 0)
				* 100,
				1
			),
			0
		) AS attendance_rate_30d,

		b.created_at

	FROM branches b

	LEFT JOIN employees e
		ON e.branch_id = b.id
		AND e.role = 'karyawan'

	LEFT JOIN attendance a
		ON a.employee_id = e.id
		AND a.date >= CURRENT_DATE - INTERVAL '30 days'

	%s

	GROUP BY
		b.id,
		b.name,
		b.address,
		b.radius_meter,
		b.created_at

	ORDER BY b.name

	LIMIT $%d OFFSET $%d
`, where, argIdx, argIdx+1), append(args, perPage, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var id, totalEmployees, radiusMeter int
		var attendanceRate30d float64
		var name, address string
		var createdAt *time.Time
		rows.Scan(
			&id,
			&name,
			&address,
			&radiusMeter,
			&totalEmployees,
			&attendanceRate30d,
			&createdAt,
		)
		result = append(result, map[string]interface{}{
			"branch_id":           fmt.Sprintf("BR%03d", id),
			"branch_name":         name,
			"address":             address,
			"total_employees":     totalEmployees,
			"radius_meter":        radiusMeter,
			"attendance_rate_30d": attendanceRate30d,
			"status":              "Active",
			"created_date":        createdAt,
		})
	}
	return result, total, nil
}

func GetTodayAttendanceOverview(db *sql.DB) (*attendance.TodayAttendanceResponse, error) {
	loc, _ := time.LoadLocation("Asia/Jakarta")
	today := time.Now().In(loc).Format("2006-01-02")

	rows, err := db.Query(`
		SELECT
			b.id,
			b.name,
			COUNT(DISTINCT e.id) as total_employees,
			COUNT(DISTINCT a.employee_id) as present_today
		FROM branches b
		LEFT JOIN employees e
			ON e.branch_id = b.id
		LEFT JOIN attendance a
			ON a.employee_id = e.id
			AND a.date = $1
		GROUP BY b.id, b.name
		ORDER BY b.id
	`, today)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []attendance.BranchAttendance

	totalEmployees := 0
	totalPresent := 0

	for rows.Next() {
		var b attendance.BranchAttendance

		err := rows.Scan(
			&b.BranchID,
			&b.BranchName,
			&b.TotalEmployees,
			&b.PresentToday,
		)

		if err != nil {
			return nil, err
		}

		b.Absent = b.TotalEmployees - b.PresentToday

		if b.TotalEmployees > 0 {
			b.AttendanceRate =
				(float64(b.PresentToday) / float64(b.TotalEmployees)) * 100
		}

		if b.AttendanceRate >= 90 {
			b.Status = "Excellent"
		} else if b.AttendanceRate >= 70 {
			b.Status = "Good"
		} else {
			b.Status = "Critical"
		}

		totalEmployees += b.TotalEmployees
		totalPresent += b.PresentToday

		branches = append(branches, b)
	}

	totalAbsent := totalEmployees - totalPresent

	var overallRate float64

	if totalEmployees > 0 {
		overallRate =
			(float64(totalPresent) / float64(totalEmployees)) * 100
	}

	response := &attendance.TodayAttendanceResponse{
		TotalPresent:   totalPresent,
		TotalAbsent:    totalAbsent,
		TotalEmployees: totalEmployees,
		AttendanceRate: overallRate,
		Branches:       branches,
	}

	return response, nil
}

// ─────────────────────────────────────────────────────────────
// GetBranchDetail
// Detail satu cabang + statistik
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetBranchDetail(branchID int) (map[string]interface{}, error) {

	var id, radiusMeter int
	var name, address string
	var latitude, longitude float64
	var createdAt *time.Time

	err := r.db.QueryRow(`
		SELECT id, name, COALESCE(address,''), latitude, longitude, radius_meter, created_at
		FROM branches WHERE id = $1
	`, branchID).Scan(&id, &name, &address, &latitude, &longitude, &radiusMeter, &createdAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("cabang tidak ditemukan")
	}
	if err != nil {
		return nil, err
	}

	// Statistik karyawan
	var totalEmployees int
	r.db.QueryRow(`
		SELECT COUNT(*) FROM employees WHERE branch_id = $1 AND role = 'karyawan'
	`, branchID).Scan(&totalEmployees)

	// Statistik attendance hari ini
	today := utils.NowWITA()
	var todayPresent, todayAbsent int
	r.db.QueryRow(`
		SELECT
			COUNT(CASE WHEN a.status IN ('PRESENT','LATE','WFA') THEN 1 END),
			COUNT(CASE WHEN a.status = 'ABSENT' THEN 1 END)
		FROM attendance a
		JOIN employees e ON e.id = a.employee_id
		WHERE e.branch_id = $1 AND a.date = $2
	`, branchID, today).Scan(&todayPresent, &todayAbsent)

	// Attendance rate 30 hari
	var rate30d float64
	r.db.QueryRow(`
		SELECT ROUND(
			COUNT(CASE WHEN a.status IN ('PRESENT','LATE','WFA') THEN 1 END)::numeric /
			NULLIF(COUNT(a.id), 0) * 100, 1
		)
		FROM attendance a
		JOIN employees e ON e.id = a.employee_id
		WHERE e.branch_id = $1 AND a.date >= NOW() - INTERVAL '30 days'
	`, branchID).Scan(&rate30d)

	return map[string]interface{}{
		"branch_id":           fmt.Sprintf("BR%03d", id),
		"branch_name":         name,
		"address":             address,
		"latitude":            latitude,
		"longitude":           longitude,
		"radius_meter":        radiusMeter,
		"total_employees":     totalEmployees,
		"today_present":       todayPresent,
		"today_absent":        todayAbsent,
		"attendance_rate_30d": rate30d,
		"status":              "Active",
		"created_date":        createdAt,
	}, nil

}

// ─────────────────────────────────────────────────────────────
// CreateBranch
// Tambah cabang baru
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) CreateBranch(
	name string,
	address string,
	latitude float64,
	longitude float64,
	radiusMeter int,
	status string,
) (int, error) {

	var id int
	err := r.db.QueryRow(`
		INSERT INTO branches (
			name,
			address,
			latitude,
			longitude,
			radius_meter,
			status,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id
	`,
		name,
		address,
		latitude,
		longitude,
		radiusMeter,
		status,
	).Scan(&id)

	return id, err
}

// ─────────────────────────────────────────────────────────────
// UpdateBranch
// Edit data cabang
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) UpdateBranch(
	branchID int,
	name string,
	address string,
	latitude float64,
	longitude float64,
	radiusMeter int,
	status string,
) error {

	result, err := r.db.Exec(`
		UPDATE branches
		SET
			name = $1,
			address = $2,
			latitude = $3,
			longitude = $4,
			radius_meter = $5,
			status = $6
		WHERE id = $7
	`,
		name,
		address,
		latitude,
		longitude,
		radiusMeter,
		status,
		branchID,
	)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()

	if rowsAffected == 0 {
		return fmt.Errorf("cabang tidak ditemukan")
	}

	return nil
}

// ─────────────────────────────────────────────────────────────
// DeleteBranch
// Hapus cabang
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) DeleteBranch(branchID int) error {

	result, err := r.db.Exec(`
		DELETE FROM branches
		WHERE id = $1
	`, branchID)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()

	if rowsAffected == 0 {
		return fmt.Errorf("cabang tidak ditemukan")
	}

	return nil
}

// ─────────────────────────────────────────────────────────────
// GetAllBranchAdmins
// List semua admin cabang dari semua cabang (untuk halaman Admin Cabang)
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetAllBranchAdmins(page, limit int, search, status string) ([]models.EmployeeDetail, int, error) {

	offset := (page - 1) * limit

	where := "WHERE e.role = 'admin' AND e.tipe = 'cabang'"
	args := []interface{}{}
	argIdx := 1

	if search != "" {
		where += fmt.Sprintf(
			" AND (e.name ILIKE $%d OR e.username ILIKE $%d OR b.name ILIKE $%d)",
			argIdx, argIdx+1, argIdx+2,
		)
		args = append(args, "%"+search+"%", "%"+search+"%", "%"+search+"%")
		argIdx += 3
	}

	if status != "" {
		where += fmt.Sprintf(" AND e.status = $%d", argIdx)
		args = append(args, status)
		argIdx++
	}

	var total int
	countQ := fmt.Sprintf(`
		SELECT COUNT(*) FROM employees e
		LEFT JOIN branches b ON e.branch_id = b.id
		%s
	`, where)
	r.db.QueryRow(countQ, args...).Scan(&total)

	dataQ := fmt.Sprintf(`
		SELECT
			e.id,
			e.username,
			COALESCE(e.name, '') AS name,
			e.role,
			COALESCE(e.tipe, '') AS tipe,
			COALESCE(e.status, 'active') AS status,
			e.created_at,
			e.branch_id,
			COALESCE(b.name, '') AS branch_name
		FROM employees e
		LEFT JOIN branches b ON e.branch_id = b.id
		%s
		ORDER BY e.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.Query(dataQ, args...)
	if err != nil {
    log.Printf("GET ALL BRANCH ADMINS ERROR: %v", err)
    return nil, 0, err
}
	defer rows.Close()

	var admins []models.EmployeeDetail
	for rows.Next() {
		var admin models.EmployeeDetail
		var BranchID sql.NullInt64
		var BranchName sql.NullString
		err := rows.Scan(
			&admin.ID,
			&admin.Username,
			&admin.FullName,
			&admin.Role,
			&admin.Tipe,
			&admin.Status,
			&admin.CreatedAt,
			&BranchID,
			&BranchName,
		)
		if err != nil {
			 log.Printf("SCAN ERROR: %v", err)
			return nil, 0, err
		}
		admins = append(admins, admin)
	}

	return admins, total, nil
}

// ─────────────────────────────────────────────────────────────
// GetBranchIDByName
// Lookup branch_name → branch_id (FE kirim nama cabang)
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetBranchIDByName(name string) (int, error) {
	var id int
	err := r.db.QueryRow(`
		SELECT id FROM branches WHERE name = $1
	`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("cabang '%s' tidak ditemukan", name)
	}
	return id, err
}

// ─────────────────────────────────────────────────────────────
// CreateBranchAdmin
// DIPERBAIKI: tambah field name (nama lengkap)
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) CreateBranchAdmin(
	branchID int,
	username string,
	password string,
	name string,
) error {

	// cek username sudah ada
	var exists int
	r.db.QueryRow(`SELECT COUNT(*) FROM employees WHERE username = $1`, username).Scan(&exists)
	if exists > 0 {
		return fmt.Errorf("username '%s' sudah digunakan", username)
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		INSERT INTO employees (username, password, name, role, tipe, branch_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		username,
		hashedPassword,
		name,
		"admin",
		"cabang",
		branchID,
		"active",
	)

	return err
}

// ─────────────────────────────────────────────────────────────
// UpdateBranchAdmin
// Edit username, nama lengkap, cabang, dan status admin cabang
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) UpdateBranchAdmin(
	adminID int,
	username string,
	name string,
	branchID int,
	status string,
) error {

	// cek admin ada dengan role admin dan tipe cabang
	var exists int
	r.db.QueryRow(
		`SELECT COUNT(*) FROM employees WHERE id = $1 AND role = 'admin' AND tipe = 'cabang'`,
		adminID,
	).Scan(&exists)
	if exists == 0 {
		return fmt.Errorf("admin cabang tidak ditemukan")
	}

	// cek username tidak bentrok dengan akun lain
	var conflict int
	r.db.QueryRow(
		`SELECT COUNT(*) FROM employees WHERE username = $1 AND id != $2`,
		username, adminID,
	).Scan(&conflict)
	if conflict > 0 {
		return fmt.Errorf("username '%s' sudah digunakan", username)
	}

	result, err := r.db.Exec(`
		UPDATE employees
		SET username = $1, name = $2, branch_id = $3, status = $4
		WHERE id = $5 AND role = 'admin' AND tipe = 'cabang'
	`, username, name, branchID, status, adminID)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("admin cabang tidak ditemukan")
	}

	return nil
}

// ─────────────────────────────────────────────────────────────
// GetBranchAdmins (per branch - tetap dipertahankan)
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) GetBranchAdmins(branchID int) ([]models.EmployeeDetail, error) {

	rows, err := r.db.Query(`
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
		WHERE e.branch_id = $1
		AND e.role = 'admin' AND e.tipe = 'cabang'
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

// ─────────────────────────────────────────────────────────────
// DeleteBranchAdmin
// ─────────────────────────────────────────────────────────────
func (r *GlobalAdminRepository) DeleteBranchAdmin(adminID int) error {

	result, err := r.db.Exec(`
		DELETE FROM employees
		WHERE id = $1 AND role = 'admin' AND tipe = 'cabang'
	`, adminID)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("admin cabang tidak ditemukan")
	}

	return nil
}
