package repository

import (
	"absensi/models"
	"database/sql"
	"fmt"
	"strings"
)

type AttendanceRepo struct {
	db *sql.DB
}

func NewAttendanceRepo(db *sql.DB) *AttendanceRepo {
	return &AttendanceRepo{db: db}
}

// TodayByBranch - absensi hari ini untuk dashboard
// Semua kolom sesuai tabel attendance di DB
func (r *AttendanceRepo) TodayByBranch(branchID int, search, statusFilter string) ([]models.Attendance, error) {
	where := `WHERE a.branch_id = $1 AND a.date = CURRENT_DATE`
	args := []interface{}{branchID}
	argIdx := 2

	if search != "" {
		// Cari berdasarkan username karena tidak ada kolom name di employees
		where += fmt.Sprintf(` AND e.username ILIKE $%d`, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if statusFilter != "" && statusFilter != "all" && statusFilter != "All" {
		where += fmt.Sprintf(` AND a.status = $%d`, argIdx)
		args = append(args, statusFilter)
	}

	query := fmt.Sprintf(`
		SELECT 
			a.id,
			a.employee_id,
			e.username,
			a.date::text,
			COALESCE(a.work_mode, ''),
			a.work_type,
			a.status,
			a.check_in,
			a.check_out,
			a.check_in_lat,
			a.check_in_lon,
			a.late_minutes,
			a.distance_meter,
			a.is_auto_checkout,
			COALESCE(a.wfa_reason, ''),
			COALESCE(a.early_leave_reason, '')
		FROM attendance a
		JOIN employees e ON a.employee_id = e.id
		%s
		ORDER BY a.check_in DESC NULLS LAST
	`, where)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Attendance
	for rows.Next() {
		var a models.Attendance
		err := rows.Scan(
			&a.ID,
			&a.EmployeeID,
			&a.EmployeeUsername,
			&a.Date,
			&a.WorkMode,
			&a.WorkType,
			&a.Status,
			&a.CheckIn,
			&a.CheckOut,
			&a.CheckInLat,
			&a.CheckInLon,
			&a.LateMinutes,
			&a.DistanceMeter,
			&a.IsAutoCheckout,
			&a.WFAReason,
			&a.EarlyLeaveReason,
		)
		if err != nil {
			continue
		}
		list = append(list, a)
	}
	return list, nil
}

// DashboardStats - statistik absensi hari ini per cabang
func (r *AttendanceRepo) DashboardStats(branchID int) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	// Total karyawan aktif di cabang ini
	r.db.QueryRow(`
		SELECT COUNT(*) FROM employees 
		WHERE branch_id = $1 AND status = 'active'
	`, branchID).Scan(&stats.TotalEmployee)

	// Hitung per status hari ini
	rows, err := r.db.Query(`
		SELECT status, COUNT(*) 
		FROM attendance 
		WHERE branch_id = $1 AND date = CURRENT_DATE
		GROUP BY status
	`, branchID)
	if err != nil {
		return stats, nil
	}
	defer rows.Close()

	checkedIn := 0
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		switch status {
		case "PRESENT", "EARLY_LEAVE":
			stats.Present += count
			checkedIn += count
		case "LATE":
			stats.Late += count
			checkedIn += count
		case "WFA":
			stats.WFA += count
			checkedIn += count
		}
	}
	stats.Absent = stats.TotalEmployee - checkedIn
	if stats.Absent < 0 {
		stats.Absent = 0
	}
	return stats, nil
}

// HistoryByEmployee - riwayat absensi satu karyawan (paginated)
func (r *AttendanceRepo) HistoryByEmployee(employeeID, branchID, page, limit int) ([]models.Attendance, int, error) {
	offset := (page - 1) * limit

	var total int
	r.db.QueryRow(`
		SELECT COUNT(*) FROM attendance a
		JOIN employees e ON a.employee_id = e.id
		WHERE a.employee_id = $1 AND e.branch_id = $2
	`, employeeID, branchID).Scan(&total)

	rows, err := r.db.Query(`
		SELECT 
			a.id,
			a.employee_id,
			e.username,
			a.date::text,
			COALESCE(a.work_mode, ''),
			a.work_type,
			a.status,
			a.check_in,
			a.check_out,
			a.check_in_lat,
			a.check_in_lon,
			a.late_minutes,
			a.distance_meter,
			a.is_auto_checkout,
			COALESCE(a.wfa_reason, ''),
			COALESCE(a.early_leave_reason, '')
		FROM attendance a
		JOIN employees e ON a.employee_id = e.id
		WHERE a.employee_id = $1 AND e.branch_id = $2
		ORDER BY a.date DESC
		LIMIT $3 OFFSET $4
	`, employeeID, branchID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.Attendance
	for rows.Next() {
		var a models.Attendance
		rows.Scan(
			&a.ID,
			&a.EmployeeID,
			&a.EmployeeUsername,
			&a.Date,
			&a.WorkMode,
			&a.WorkType,
			&a.Status,
			&a.CheckIn,
			&a.CheckOut,
			&a.CheckInLat,
			&a.CheckInLon,
			&a.LateMinutes,
			&a.DistanceMeter,
			&a.IsAutoCheckout,
			&a.WFAReason,
			&a.EarlyLeaveReason,
		)
		list = append(list, a)
	}
	return list, total, nil
}

// ReportByDateRange - laporan harian per rentang tanggal
func (r *AttendanceRepo) ReportByDateRange(branchID int, startDate, endDate string) ([]models.AttendanceReport, error) {
	rows, err := r.db.Query(`
		SELECT 
			a.date::text,
			COUNT(*) FILTER (WHERE a.status IN ('PRESENT','LATE','EARLY_LEAVE')) AS total_present,
			COUNT(*) FILTER (WHERE a.status = 'LATE') AS total_late,
			COUNT(*) FILTER (WHERE a.status = 'WFA') AS total_wfa,
			COUNT(*) FILTER (WHERE a.status = 'ABSENT') AS total_absent
		FROM attendance a
		WHERE a.branch_id = $1
		  AND a.date BETWEEN $2::date AND $3::date
		GROUP BY a.date
		ORDER BY a.date DESC
	`, branchID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []models.AttendanceReport
	for rows.Next() {
		var rep models.AttendanceReport
		rows.Scan(&rep.Date, &rep.TotalPresent, &rep.TotalLate, &rep.TotalWFA, &rep.TotalAbsent)
		reports = append(reports, rep)
	}
	return reports, nil
}

// DivisionReport - attendance rate per divisi
func (r *AttendanceRepo) DivisionReport(branchID int, startDate, endDate string) ([]models.DivisionReport, error) {
	rows, err := r.db.Query(`
		SELECT 
			d.name,
			COUNT(DISTINCT e.id) AS total_employees,
			ROUND(
				100.0 * COUNT(a.id) FILTER (WHERE a.status IN ('PRESENT','LATE','WFA','EARLY_LEAVE'))
				/ NULLIF(COUNT(DISTINCT e.id) * (($3::date - $2::date) + 1), 0),
				1
			) AS attendance_rate
		FROM divisions d
		JOIN employees e ON e.division_id = d.id
		LEFT JOIN attendance a 
			ON a.employee_id = e.id 
			AND a.date BETWEEN $2::date AND $3::date
		WHERE e.branch_id = $1 AND e.status = 'active'
		GROUP BY d.id, d.name
		ORDER BY attendance_rate DESC NULLS LAST
	`, branchID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []models.DivisionReport
	for rows.Next() {
		var rep models.DivisionReport
		rows.Scan(&rep.DivisionName, &rep.TotalEmployees, &rep.AttendanceRate)
		reports = append(reports, rep)
	}
	return reports, nil
}

// ReportSummary - hitung summary stats + perbandingan minggu lalu
func (r *AttendanceRepo) ReportSummary(branchID int, startDate, endDate string) (*models.ReportSummary, error) {
	summary := &models.ReportSummary{}

	// ── Minggu ini ────────────────────────────────────────────────────────────
	var totalDays, totalPresent, totalLate, totalEmployeePerDay int

	err := r.db.QueryRow(`
		SELECT
			COUNT(DISTINCT a.date)                                                        AS total_days,
			COUNT(a.id) FILTER (WHERE a.status IN ('PRESENT','LATE','WFA','EARLY_LEAVE')) AS total_present,
			COUNT(a.id) FILTER (WHERE a.status = 'LATE')                                  AS total_late,
			COALESCE(
				(SELECT COUNT(*) FROM employees WHERE branch_id = $1 AND status = 'active'), 0
			)                                                                             AS total_employee
		FROM attendance a
		WHERE a.branch_id = $1
		  AND a.date BETWEEN $2::date AND $3::date
	`, branchID, startDate, endDate).Scan(&totalDays, &totalPresent, &totalLate, &totalEmployeePerDay)
	if err != nil {
		return summary, err
	}

	summary.TotalPresent = totalPresent
	summary.LateArrivals = totalLate

	// Hitung average attendance rate minggu ini
	if totalDays > 0 && totalEmployeePerDay > 0 {
		possible := totalDays * totalEmployeePerDay
		summary.AverageAttendanceRate = roundFloat(float64(totalPresent)/float64(possible)*100, 1)
	}

	// ── Minggu lalu (untuk perbandingan) ─────────────────────────────────────
	var prevPresent, prevLate int
	var prevDays, prevEmployee int

	r.db.QueryRow(`
		SELECT
			COUNT(DISTINCT a.date)                                                        AS total_days,
			COUNT(a.id) FILTER (WHERE a.status IN ('PRESENT','LATE','WFA','EARLY_LEAVE')) AS total_present,
			COUNT(a.id) FILTER (WHERE a.status = 'LATE')                                  AS total_late,
			COALESCE(
				(SELECT COUNT(*) FROM employees WHERE branch_id = $1 AND status = 'active'), 0
			)                                                                             AS total_employee
		FROM attendance a
		WHERE a.branch_id = $1
		  AND a.date BETWEEN ($2::date - INTERVAL '7 days') AND ($3::date - INTERVAL '7 days')
	`, branchID, startDate, endDate).Scan(&prevDays, &prevPresent, &prevLate, &prevEmployee)

	// Hitung perubahan persentase
	summary.TotalPresentChange = percentChange(prevPresent, totalPresent)
	summary.LateArrivalsChange = percentChange(prevLate, totalLate)

	var prevRate float64
	if prevDays > 0 && prevEmployee > 0 {
		prevRate = float64(prevPresent) / float64(prevDays*prevEmployee) * 100
	}
	if prevRate > 0 {
		summary.AttendanceRateChange = roundFloat(summary.AverageAttendanceRate-prevRate, 1)
	}

	return summary, nil
}

// DivisionReportWithStatus - attendance rate per divisi + status label
func (r *AttendanceRepo) DivisionReportWithStatus(branchID int, startDate, endDate string) ([]models.DivisionReport, error) {
	rows, err := r.db.Query(`
		SELECT
			d.name,
			COUNT(DISTINCT e.id) AS total_employees,
			ROUND(
				100.0 * COUNT(a.id) FILTER (WHERE a.status IN ('PRESENT','LATE','WFA','EARLY_LEAVE'))
				/ NULLIF(COUNT(DISTINCT e.id) * (($3::date - $2::date) + 1), 0),
				1
			) AS attendance_rate
		FROM divisions d
		JOIN employees e ON e.division_id = d.id
		LEFT JOIN attendance a
			ON a.employee_id = e.id
			AND a.date BETWEEN $2::date AND $3::date
		WHERE e.branch_id = $1 AND e.status = 'active'
		GROUP BY d.id, d.name
		ORDER BY attendance_rate DESC NULLS LAST
	`, branchID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []models.DivisionReport
	for rows.Next() {
		var rep models.DivisionReport
		rows.Scan(&rep.DivisionName, &rep.TotalEmployees, &rep.AttendanceRate)

		// Tentukan status label berdasarkan attendance rate
		switch {
		case rep.AttendanceRate >= 90:
			rep.Status = "Excellent"
		case rep.AttendanceRate >= 80:
			rep.Status = "Good"
		default:
			rep.Status = "Poor"
		}

		reports = append(reports, rep)
	}
	return reports, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func percentChange(prev, current int) float64 {
	if prev == 0 {
		if current > 0 {
			return 100.0
		}
		return 0
	}
	return roundFloat(float64(current-prev)/float64(prev)*100, 1)
}

func roundFloat(val float64, precision int) float64 {
	p := 1.0
	for i := 0; i < precision; i++ {
		p *= 10
	}
	return float64(int(val*p+0.5)) / p
}

// WeeklySchedule - ambil status absensi per karyawan per hari dalam satu minggu
func (r *AttendanceRepo) WeeklySchedule(branchID int, startDate, endDate string) ([]models.EmployeeSchedule, error) {
	// Ambil semua karyawan aktif di cabang ini
	rows, err := r.db.Query(`
		SELECT 
			e.id,
			e.username,
			COALESCE(d.name, '') as division_name,
			d.work_days
		FROM employees e
		LEFT JOIN divisions d ON e.division_id = d.id
		WHERE e.branch_id = $1 AND e.status = 'active'
		ORDER BY e.username
	`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type empData struct {
		ID       int
		Username string
		Division string
		WorkDays string
	}

	var employees []empData
	for rows.Next() {
		var e empData
		rows.Scan(&e.ID, &e.Username, &e.Division, &e.WorkDays)
		employees = append(employees, e)
	}

	// Untuk setiap karyawan, ambil status per hari dalam minggu ini
	var schedules []models.EmployeeSchedule
	for _, emp := range employees {
		schedule := models.EmployeeSchedule{
			EmployeeID:       emp.ID,
			EmployeeUsername: emp.Username,
			Division:         emp.Division,
		}

		// Ambil attendance minggu ini
		attRows, err := r.db.Query(`
			SELECT 
				EXTRACT(DOW FROM date)::int as day_of_week,
				work_type,
				status
			FROM attendance
			WHERE employee_id = $1
			  AND date BETWEEN $2::date AND $3::date
		`, emp.ID, startDate, endDate)
		if err != nil {
			continue
		}

		// Map day_of_week → status (0=Sunday, 1=Monday, ..., 6=Saturday)
		dayMap := map[int]string{}
		for attRows.Next() {
			var dow int
			var workType, status string
			attRows.Scan(&dow, &workType, &status)
			dayMap[dow] = workType
		}
		attRows.Close()

		// Tentukan status per hari
		// Cek work_days untuk tau hari kerja atau libur
		workDays := emp.WorkDays // contoh: "1,2,3,4,5"

		schedule.Monday    = getDayStatus(dayMap, 1, workDays)
		schedule.Tuesday   = getDayStatus(dayMap, 2, workDays)
		schedule.Wednesday = getDayStatus(dayMap, 3, workDays)
		schedule.Thursday  = getDayStatus(dayMap, 4, workDays)
		schedule.Friday    = getDayStatus(dayMap, 5, workDays)
		schedule.Saturday  = getDayStatus(dayMap, 6, workDays)
		schedule.Sunday    = getDayStatus(dayMap, 0, workDays)

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}

// getDayStatus - tentukan status hari: WFO / WFA / Off
func getDayStatus(dayMap map[int]string, dow int, workDays string) string {
	// Cek apakah ada data attendance untuk hari ini
	if status, ok := dayMap[dow]; ok {
		return status
	}

	// Cek apakah hari ini adalah hari kerja
	dowStr := fmt.Sprintf("%d", dow)
	if strings.Contains(workDays, dowStr) {
		return "Off" // hari kerja tapi tidak ada absensi
	}

	return "Off" // hari libur
}

// WeeklyChartData - data untuk Weekly Attendance bar chart
// Group per hari dalam seminggu: Mon, Tue, Wed, Thu, Fri
func (r *AttendanceRepo) WeeklyChartData(branchID int, startDate, endDate string) ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`
		SELECT
			TO_CHAR(a.date, 'Dy') AS day_name,
			a.date,
			COUNT(a.id) FILTER (WHERE a.status IN ('PRESENT','EARLY_LEAVE')) AS present,
			COUNT(a.id) FILTER (WHERE a.status = 'LATE')                      AS late,
			COUNT(a.id) FILTER (WHERE a.status = 'WFA')                       AS wfa,
			COUNT(a.id) FILTER (WHERE a.status = 'ABSENT')                    AS absent
		FROM attendance a
		WHERE a.branch_id = $1
		  AND a.date BETWEEN $2::date AND $3::date
		GROUP BY a.date, TO_CHAR(a.date, 'Dy')
		ORDER BY a.date
	`, branchID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var dayName, date string
		var present, late, wfa, absent int
		rows.Scan(&dayName, &date, &present, &late, &wfa, &absent)
		result = append(result, map[string]interface{}{
			"day":     dayName, // Mon, Tue, Wed, Thu, Fri
			"date":    date,
			"present": present,
			"late":    late,
			"wfa":     wfa,
			"absent":  absent,
		})
	}
	return result, nil
}

// MonthlyChartData - data untuk Monthly Trend line chart
// Attendance rate % per bulan
func (r *AttendanceRepo) MonthlyChartData(branchID int, year int) ([]map[string]interface{}, error) {
	rows, err := r.db.Query(`
		SELECT
			TO_CHAR(a.date, 'Mon') AS month_name,
			EXTRACT(MONTH FROM a.date)::int AS month_num,
			COUNT(a.id) FILTER (WHERE a.status IN ('PRESENT','LATE','WFA','EARLY_LEAVE')) AS total_present,
			COUNT(DISTINCT e.id) AS total_employee
		FROM attendance a
		JOIN employees e ON a.employee_id = e.id
		WHERE a.branch_id = $1
		  AND EXTRACT(YEAR FROM a.date) = $2
		GROUP BY TO_CHAR(a.date, 'Mon'), EXTRACT(MONTH FROM a.date)
		ORDER BY month_num
	`, branchID, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var monthName string
		var monthNum, totalPresent, totalEmployee int
		rows.Scan(&monthName, &monthNum, &totalPresent, &totalEmployee)

		var rate float64
		if totalEmployee > 0 {
			rate = roundFloat(float64(totalPresent)/float64(totalEmployee)*100, 1)
		}

		result = append(result, map[string]interface{}{
			"month":           monthName, // Jan, Feb, Mar, ...
			"month_num":       monthNum,
			"attendance_rate": rate,
			"total_present":   totalPresent,
		})
	}
	return result, nil
}
