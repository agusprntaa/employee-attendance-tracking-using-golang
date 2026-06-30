package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
)

type QRHandler struct {
	qrRepo *repository.QRRepo
}

func NewQRHandler(qr *repository.QRRepo) *QRHandler {
	return &QRHandler{qrRepo: qr}
}

// GET /admin-cabang/qr/today
func (h *QRHandler) GetTodayQR(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	qr, err := h.qrRepo.GetTodayQR(*claims.BranchID)
if err != nil {
    return c.Status(500).JSON(fiber.Map{
        "message": err.Error(),
    })
}
	qr, err = h.qrRepo.RefreshQR(*claims.BranchID)
if err != nil {
    return c.Status(500).JSON(fiber.Map{
        "message": err.Error(),
    })
}

	return utils.Success(c, buildQRResponse(qr))
}

// POST /admin-cabang/qr/regenerate
func (h *QRHandler) RegenerateQR(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	qr, err := h.qrRepo.RefreshQR(*claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal regenerate QR Code")
	}

	return utils.Success(c, buildQRResponse(qr))
}

func buildQRResponse(qr *models.QRData) fiber.Map {
	qrContentMap := fiber.Map{
		"token":     qr.Token,
		"branch_id": qr.BranchID,
		"date":      qr.Date,
	}
	qrContentBytes, _ := json.Marshal(qrContentMap)

	refreshIn := int(time.Until(qr.ExpiresAt).Seconds())
	if refreshIn < 0 {
		refreshIn = 0
	}

	return fiber.Map{
		"token":      qr.Token,
		"branch_id":  qr.BranchID,
		"date":       qr.Date,
		"expires_at": qr.ExpiresAt,
		"expires":    "Berlaku 3 menit",
		"refresh_in": refreshIn,
		"qr_content": string(qrContentBytes),
	}
}
