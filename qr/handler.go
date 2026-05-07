package qr

import (
	"absensi_karyawan/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{}

// GET /qr/today — hanya untuk admin
// Return token yang sudah include slot waktu 3 menit
func (h *Handler) GetTodayToken(c *fiber.Ctx) error {
	branchID, ok := c.Locals("branch_id").(int)
	if !ok || branchID == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Branch ID tidak ditemukan",
		})
	}

	now := time.Now()
	today := now.UTC().Format("2006-01-02")
	slotWaktu := now.Minute() / 3

	token := utils.GenerateQRToken(branchID, today, slotWaktu)

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"token":     token,
			"branch_id": branchID,
			"date":      today,
			"expire":    "Berlaku 3 menit",
		},
	})
}
