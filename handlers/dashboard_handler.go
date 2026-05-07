package handlers

import (
	"absensi/middleware"
	"absensi/repository"
	"absensi/utils"

	"github.com/gofiber/fiber/v2"
)

type DashboardHandler struct {
	attendanceRepo *repository.AttendanceRepo
}

func NewDashboardHandler(ar *repository.AttendanceRepo) *DashboardHandler {
	return &DashboardHandler{attendanceRepo: ar}
}

// GET /admin-cabang/dashboard
func (h *DashboardHandler) GetDashboard(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}
	branchID := *claims.BranchID

	stats, err := h.attendanceRepo.DashboardStats(branchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil statistik absensi")
	}

	search := c.Query("search")
	statusFilter := c.Query("status")

	attendance, err := h.attendanceRepo.TodayByBranch(branchID, search, statusFilter)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data absensi hari ini")
	}

	return utils.Success(c, fiber.Map{
		"stats":      stats,
		"attendance": attendance,
	})
}
