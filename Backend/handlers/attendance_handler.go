package handlers

import (
	"absensi/middleware"
	"absensi/repository"
	"absensi/utils"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AttendanceHandler struct {
	attendanceRepo *repository.AttendanceRepo
}

func NewAttendanceHandler(ar *repository.AttendanceRepo) *AttendanceHandler {
	return &AttendanceHandler{attendanceRepo: ar}
}

// GET /admin-cabang/attendance/today?search=budi&status=PRESENT
func (h *AttendanceHandler) TodayAttendance(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	attendance, err := h.attendanceRepo.TodayByBranch(*claims.BranchID, search, status)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data absensi hari ini")
		return
	}
	utils.Success(w, attendance)
}

// GET /admin-cabang/attendance/employee/{id}?page=1&limit=10
func (h *AttendanceHandler) EmployeeHistory(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	empID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		utils.BadRequest(w, "INVALID_ID", "ID karyawan tidak valid")
		return
	}

	page, limit := utils.ParsePage(r)

	list, total, err := h.attendanceRepo.HistoryByEmployee(empID, *claims.BranchID, page, limit)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil riwayat absensi")
		return
	}

	utils.Success(w, map[string]interface{}{
		"data": list,
		"pagination": map[string]int{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// GET /admin-cabang/reports/attendance?start_date=2026-04-01&end_date=2026-04-30&year=2026
// Response lengkap:
// - summary        → kartu ringkasan + perbandingan minggu lalu
// - weekly_chart   → data bar chart Weekly Attendance (Mon-Fri)
// - monthly_chart  → data line chart Monthly Trend (Jan-Des)
// - daily          → data per hari
// - division       → data per divisi (Department Performance)
func (h *AttendanceHandler) AttendanceReport(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	yearStr := r.URL.Query().Get("year")

	// Default: bulan ini
	now := time.Now()
	if startDate == "" {
		startDate = now.Format("2006-01") + "-01"
	}
	if endDate == "" {
		endDate = now.Format("2006-01-02")
	}

	year := now.Year()
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			year = y
		}
	}

	// 1. Summary stats + perbandingan minggu lalu
	summary, err := h.attendanceRepo.ReportSummary(*claims.BranchID, startDate, endDate)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil summary laporan")
		return
	}

	// 2. Weekly chart (bar chart) — minggu ini
	now2 := time.Now()
	weekday := int(now2.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := now2.AddDate(0, 0, -(weekday - 1))
	sunday := monday.AddDate(0, 0, 6)
	weeklyChart, err := h.attendanceRepo.WeeklyChartData(
		*claims.BranchID,
		monday.Format("2006-01-02"),
		sunday.Format("2006-01-02"),
	)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data chart mingguan")
		return
	}

	// 3. Monthly chart (line chart) — per bulan dalam tahun ini
	monthlyChart, err := h.attendanceRepo.MonthlyChartData(*claims.BranchID, year)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data chart bulanan")
		return
	}

	// 4. Daily report
	dailyReports, err := h.attendanceRepo.ReportByDateRange(*claims.BranchID, startDate, endDate)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil laporan harian")
		return
	}

	// 5. Division report (Department Performance)
	divReports, err := h.attendanceRepo.DivisionReportWithStatus(*claims.BranchID, startDate, endDate)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil laporan divisi")
		return
	}

	utils.Success(w, map[string]interface{}{
		"summary":       summary,       // kartu ringkasan atas
		"weekly_chart":  weeklyChart,   // bar chart Weekly Attendance
		"monthly_chart": monthlyChart,  // line chart Monthly Trend
		"daily":         dailyReports,  // tabel per hari
		"division":      divReports,    // tabel Department Performance
	})
}
