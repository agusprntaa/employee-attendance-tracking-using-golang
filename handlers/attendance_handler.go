package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

type AttendanceHandler struct {
	attendanceRepo *repository.AttendanceRepo
}

func NewAttendanceHandler(ar *repository.AttendanceRepo) *AttendanceHandler {
	return &AttendanceHandler{attendanceRepo: ar}
}

// GET /admin-cabang/attendance/today
func (h *AttendanceHandler) TodayAttendance(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	search := c.Query("search")
	status := c.Query("status")

	attendance, err := h.attendanceRepo.TodayByBranch(*claims.BranchID, search, status)
	if err != nil {
		fmt.Println("TODAY ATTENDANCE ERROR:", err)
		return utils.InternalError(c, "Gagal mengambil data absensi hari ini")
	}
	return utils.Success(c, attendance)
}

// GET /admin-cabang/attendance/employee/:id
func (h *AttendanceHandler) EmployeeHistory(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	empID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	page, limit := utils.ParsePage(c)

	list, total, err := h.attendanceRepo.HistoryByEmployee(empID, *claims.BranchID, page, limit)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil riwayat absensi")
	}

	return utils.Success(c, fiber.Map{
		"data": list,
		"pagination": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

// GET /admin-cabang/reports/attendance
func (h *AttendanceHandler) AttendanceReport(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	yearStr := c.Query("year")

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

	summary, err := h.attendanceRepo.ReportSummary(*claims.BranchID, startDate, endDate)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil summary laporan")
	}

	// Weekly chart — minggu ini
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -(weekday - 1))
	sunday := monday.AddDate(0, 0, 6)
	weeklyChart, err := h.attendanceRepo.WeeklyChartData(
		*claims.BranchID,
		monday.Format("2006-01-02"),
		sunday.Format("2006-01-02"),
	)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data chart mingguan")
	}

	monthlyChart, err := h.attendanceRepo.MonthlyChartData(*claims.BranchID, year)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data chart bulanan")
	}

	dailyReports, err := h.attendanceRepo.ReportByDateRange(*claims.BranchID, startDate, endDate)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil laporan harian")
	}

	divReports, err := h.attendanceRepo.DivisionReportWithStatus(*claims.BranchID, startDate, endDate)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil laporan divisi")
	}

	return utils.Success(c, fiber.Map{
		"summary":       summary,
		"weekly_chart":  weeklyChart,
		"monthly_chart": monthlyChart,
		"daily":         dailyReports,
		"division":      divReports,
	})
}
