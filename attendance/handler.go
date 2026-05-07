package attendance

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Service *Service
}

// errorMessage mapping error code ke pesan yang ramah untuk user
func errorMessage(err error) (int, string, string) {
	switch err {
	case ErrAlreadyCheckedIn:
		return 400, "ALREADY_CHECKED_IN", "Kamu sudah check-in hari ini"
	case ErrAlreadyCheckedOut:
		return 400, "ALREADY_CHECKED_OUT", "Kamu sudah check-out hari ini"
	case ErrCutoffExceeded:
		return 400, "CUTOFF_EXCEEDED", "Waktu check-in sudah melewati batas"
	case ErrGPSAccuracyLow:
		return 400, "GPS_ACCURACY_LOW", "Akurasi GPS terlalu rendah, coba pindah ke tempat terbuka"
	case ErrQRInvalid:
		return 400, "QR_INVALID", "QR Code tidak valid atau sudah kadaluarsa"
	case ErrBranchMismatch:
		return 400, "BRANCH_MISMATCH", "QR Code bukan milik cabang kamu"
	case ErrOutOfRadius:
		return 400, "OUT_OF_RADIUS", "Kamu berada di luar radius kantor"
	case ErrWFAReasonTooShort:
		return 400, "WFA_REASON_TOO_SHORT", "Alasan WFA minimal 20 karakter"
	case ErrNotCheckedIn:
		return 400, "NOT_CHECKED_IN", "Kamu belum check-in hari ini"
	case ErrEarlyLeaveReason:
		return 400, "EARLY_LEAVE_REASON_REQUIRED", "Alasan pulang cepat wajib diisi"
	default:
		return 500, "INTERNAL_ERROR", "Terjadi kesalahan, coba lagi"
	}
}

// ─────────────────────────────────────────
// POST /attendance/checkin
// ─────────────────────────────────────────
func (h *Handler) CheckIn(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var req CheckInRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "INVALID_REQUEST",
			"message": "Format request tidak valid",
		})
	}

	record, err := h.Service.CheckIn(employeeID, req)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"status":  "success",
		"message": "Check-in berhasil",
		"data":    recordToResponse(record),
	})
}

// ─────────────────────────────────────────
// PATCH /attendance/checkout
// ─────────────────────────────────────────
func (h *Handler) CheckOut(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var req CheckOutRequest
	// Body boleh kosong jika tidak early leave
	c.BodyParser(&req)

	record, err := h.Service.CheckOut(employeeID, req)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Check-out berhasil",
		"data":    recordToResponse(record),
	})
}

// ─────────────────────────────────────────
// GET /attendance/today
// Dipakai dashboard karyawan untuk cek status hari ini
// ─────────────────────────────────────────
func (h *Handler) GetToday(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	result, err := h.Service.GetToday(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil data absensi",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}

// ─────────────────────────────────────────
// GET /attendance/history?page=1&limit=10
// Riwayat absensi karyawan sendiri
// ─────────────────────────────────────────
func (h *Handler) GetHistory(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	result, err := h.Service.GetHistory(employeeID, page, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil riwayat absensi",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}
