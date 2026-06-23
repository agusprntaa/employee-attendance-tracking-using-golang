package attendance

import (
	"absensi_karyawan/face"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Service     *Service
	FaceService *face.Service
}

func requiresFaceToken(req CheckInRequest) bool {
	workType := strings.ToUpper(strings.TrimSpace(req.WorkType))
	return workType != "WFA"
}

// errorMessage mapping error ke HTTP status + kode + pesan user-friendly
func errorMessage(err error) (int, string, string) {
	switch err {
	case ErrAlreadyCheckedIn:
		return 400, "ALREADY_CHECKED_IN", "Kamu sudah check-in hari ini"
	case ErrAlreadyCheckedOut:
		return 400, "ALREADY_CHECKED_OUT", "Kamu sudah check-out hari ini"
	case ErrCutoffExceeded:
		return 400, "CUTOFF_EXCEEDED", "Waktu check-in sudah melewati batas maksimal"
	case ErrGPSAccuracyLow:
		return 400, "GPS_ACCURACY_LOW", "Akurasi GPS terlalu rendah, coba pindah ke tempat terbuka"
	case ErrOutOfRadius:
		return 400, "OUT_OF_RADIUS", "Kamu berada di luar radius kantor"
	case ErrWFAReasonTooShort:
		return 400, "WFA_REASON_TOO_SHORT", "Alasan WFA minimal 20 karakter"
	case ErrNotCheckedIn:
		return 400, "NOT_CHECKED_IN", "Kamu belum check-in hari ini"
	case ErrEarlyLeaveReason:
		return 400, "EARLY_LEAVE_REASON_REQUIRED", "Alasan pulang cepat wajib diisi"
	case ErrNotWorkDay:
		return 400, "NOT_WORK_DAY", "Hari ini bukan hari kerja untuk divisimu"
	case ErrEmployeeDataIncomplete:
		return 422, "EMPLOYEE_DATA_INCOMPLETE", "Data karyawan tidak lengkap, hubungi admin untuk mengatur divisi dan cabang"
	case face.ErrFaceNotVerified:
		return 403, "FACE_NOT_VERIFIED", "Verifikasi wajah dulu sebelum checkin"
	case face.ErrTokenExpired:
		return 400, "TOKEN_EXPIRED", "Waktu habis, mulai ulang dari Check In"
	case face.ErrTokenUsed:
		return 400, "TOKEN_USED", "Token sudah digunakan"
	case face.ErrTokenInvalid:
		return 400, "TOKEN_INVALID", "Token tidak valid"
	case ErrEventNotFound:
		return 404, "EVENT_NOT_FOUND", "QR event tidak valid atau sudah expired"
	case ErrEventNotInvited:
		return 403, "EVENT_NOT_INVITED", "Kamu tidak terdaftar di event ini"
	case ErrAlreadyCheckedInEvent:
		return 400, "ALREADY_CHECKED_IN_EVENT", "Kamu sudah absen di event ini"
	default:
		return 500, "INTERNAL_ERROR", "Terjadi kesalahan server, coba lagi"
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
			"message": "invalid request",
		})
	}
	log.Printf("CHECKIN REQUEST: %+v\n", req)

	if requiresFaceToken(req) {
		if req.FaceToken == "" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"code":    "MISSING_FACE_TOKEN",
				"message": "face_token wajib diisi untuk check-in WFO",
			})
		}

		_, _, err := h.FaceService.ValidateFaceTokenForCheckin(employeeID, req.FaceToken)
		if err != nil {
			switch err {
			case face.ErrFaceNotVerified:
				return c.Status(403).JSON(fiber.Map{
					"status":  "error",
					"code":    "FACE_NOT_VERIFIED",
					"message": "Verifikasi wajah dulu sebelum checkin",
				})
			case face.ErrTokenExpired:
				return c.Status(400).JSON(fiber.Map{
					"status":  "error",
					"code":    "TOKEN_EXPIRED",
					"message": "Waktu habis, mulai ulang dari Check In",
				})
			case face.ErrTokenUsed:
				return c.Status(400).JSON(fiber.Map{
					"status":  "error",
					"code":    "TOKEN_USED",
					"message": "Token sudah digunakan",
				})
			default:
				return c.Status(400).JSON(fiber.Map{
					"status":  "error",
					"code":    "TOKEN_INVALID",
					"message": "Token tidak valid",
				})
			}
		}
	}

	// Proses checkin — service akan validasi QR, GPS, jam masuk,
	// sekaligus consume token via FaceRepo.ConsumeFaceToken
	record, err := h.Service.CheckIn(employeeID, req)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
		})
	}

	if req.FaceToken != "" {
		_ = h.FaceService.ConsumeToken(req.FaceToken)
	}

	return c.Status(201).JSON(fiber.Map{
		"status":  "success",
		"message": "Check-in berhasil",
		"data":    recordToResponse(record),
	})
}

func (h *Handler) CheckInEvent(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var body struct {
		FaceToken   string `json:"face_token"`
		EventQRCode string `json:"event_qr_code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"status": "error", "message": "Format request tidak valid"})
	}
	if body.FaceToken == "" || body.EventQRCode == "" {
		return c.Status(400).JSON(fiber.Map{"status": "error", "code": "MISSING_FIELD", "message": "face_token dan event_qr_code wajib diisi"})
	}

	// Validasi face token sudah face_verified=true
	_, confidenceScore, err := h.FaceService.ValidateFaceTokenForCheckin(employeeID, body.FaceToken)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
		})
	}

	result, err := h.Service.CheckinQREvent(employeeID, body.EventQRCode, confidenceScore)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
		})
	}

	_ = h.FaceService.ConsumeToken(body.FaceToken)

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Check-in event berhasil",
		"data":    result,
	})
}

// ─────────────────────────────────────────
// PATCH /attendance/checkout
// ─────────────────────────────────────────

func (h *Handler) CheckOut(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var req CheckOutRequest
	c.BodyParser(&req) // body boleh kosong jika bukan early leave

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
// ─────────────────────────────────────────

func (h *Handler) GetToday(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	result, err := h.Service.GetToday(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil data absensi hari ini",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}

// ─────────────────────────────────────────
// GET /attendance/history?page=1&limit=10
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
