package handlers

import (
	"absensi/middleware"
	"absensi/repository"
	"absensi/utils"
	"net/http"
)

type DashboardHandler struct {
	attendanceRepo *repository.AttendanceRepo
}

func NewDashboardHandler(ar *repository.AttendanceRepo) *DashboardHandler {
	return &DashboardHandler{attendanceRepo: ar}
}

// GET /admin-cabang/dashboard
// Response: stats (total, present, late, wfa, absent) + daftar absensi hari ini
func (h *DashboardHandler) GetDashboard(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}
	branchID := *claims.BranchID

	// Statistik ringkasan hari ini
	stats, err := h.attendanceRepo.DashboardStats(branchID)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil statistik absensi")
		return
	}

	// Tabel absensi hari ini (bisa di-filter)
	search := r.URL.Query().Get("search")
	statusFilter := r.URL.Query().Get("status")

	attendance, err := h.attendanceRepo.TodayByBranch(branchID, search, statusFilter)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data absensi hari ini")
		return
	}

	utils.Success(w, map[string]interface{}{
		"stats":      stats,
		"attendance": attendance,
	})
}
