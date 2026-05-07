package handlers

import (
	"absensi/middleware"
	"absensi/models"
	"absensi/repository"
	"absensi/utils"
	"net/http"
	"strings"
)

type SettingsHandler struct {
	settingsRepo *repository.SettingsRepo
}

func NewSettingsHandler(sr *repository.SettingsRepo) *SettingsHandler {
	return &SettingsHandler{settingsRepo: sr}
}

// GET /admin-cabang/settings
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	settings, err := h.settingsRepo.GetSettings(*claims.BranchID)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data settings")
		return
	}

	utils.Success(w, settings)
}

// PATCH /admin-cabang/settings
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	var req models.UpdateSettingsRequest
	if err := utils.ParseBody(r, &req); err != nil {
		utils.BadRequest(w, "INVALID_BODY", "Request body tidak valid")
		return
	}

	// Validasi
	if strings.TrimSpace(req.BranchName) == "" {
		utils.BadRequest(w, "BRANCH_NAME_REQUIRED", "Nama cabang wajib diisi")
		return
	}
	if req.RadiusMeter <= 0 {
		req.RadiusMeter = 100
	}
	if req.LateThresholdMin <= 0 {
		req.LateThresholdMin = 15
	}
	if req.CheckinCutoffMin <= 0 {
		req.CheckinCutoffMin = 120
	}
	if req.StartTime == "" {
		req.StartTime = "08:00:00"
	}
	if req.EndTime == "" {
		req.EndTime = "17:00:00"
	}
	if req.WorkDays == "" {
		req.WorkDays = "Senin,Selasa,Rabu,Kamis,Jumat"
	}

	if err := h.settingsRepo.UpdateSettings(*claims.BranchID, &req); err != nil {
		utils.InternalError(w, "Gagal menyimpan settings")
		return
	}

	utils.SuccessMessage(w, "Settings berhasil disimpan")
}
