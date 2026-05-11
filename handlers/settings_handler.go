package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type SettingsHandler struct {
	settingsRepo *repository.SettingsRepo
}

func NewSettingsHandler(sr *repository.SettingsRepo) *SettingsHandler {
	return &SettingsHandler{settingsRepo: sr}
}

// GET /admin-cabang/settings
func (h *SettingsHandler) GetSettings(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	settings, err := h.settingsRepo.GetSettings(*claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data settings")
	}

	return utils.Success(c, settings)
}

// PATCH /admin-cabang/settings
func (h *SettingsHandler) UpdateSettings(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	var req models.UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	if strings.TrimSpace(req.BranchName) == "" {
		return utils.BadRequest(c, "BRANCH_NAME_REQUIRED", "Nama cabang wajib diisi")
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
		req.WorkDays = "1,2,3,4,5"
	}

	if err := h.settingsRepo.UpdateSettings(*claims.BranchID, &req); err != nil {
		return utils.InternalError(c, "Gagal menyimpan settings")
	}

	return utils.SuccessMessage(c, "Settings berhasil disimpan")
}
