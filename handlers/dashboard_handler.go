package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"

	"log"

	"github.com/gofiber/fiber/v2"
)

type DashboardHandler struct {
	attendanceRepo *repository.AttendanceRepo
}

func NewDashboardHandler(ar *repository.AttendanceRepo) *DashboardHandler {
	return &DashboardHandler{
		attendanceRepo: ar,
	}
}

func (h *DashboardHandler) GetDashboard(c *fiber.Ctx) error {

	branchID := auth.GetBranchID(c)

	if branchID == 0 {
		return utils.BadRequest(
			c,
			"NO_BRANCH",
			"Admin tidak memiliki cabang yang terdaftar",
		)
	}

	stats, err := h.attendanceRepo.DashboardStats(branchID)
	if err != nil {
		return utils.InternalError(
			c,
			"Gagal mengambil statistik absensi",
		)
	}

	search := c.Query("search")
	statusFilter := c.Query("status")

	attendance, err := h.attendanceRepo.TodayByBranch(
		branchID,
		search,
		statusFilter,
	)

	if err != nil {
		log.Println("ERROR DASHBOARD:", err)

		return utils.InternalError(
			c,
			"Gagal mengambil data absensi hari ini",
		)

	}

	return utils.Success(c, fiber.Map{
		"stats":      stats,
		"attendance": attendance,
	})
}
