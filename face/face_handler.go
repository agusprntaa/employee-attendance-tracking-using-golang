package face

import (
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Service *Service
}

// errorMessage — mapping error ke HTTP status + kode + pesan user-friendly
func errorMessage(err error) (int, string, string) {
	switch err {
	case ErrAlreadyCheckedIn:
		return 400, "ALREADY_CHECKED_IN", "Kamu sudah checkin hari ini"
	case ErrFaceNotRegistered:
		return 403, "FACE_NOT_REGISTERED", "Daftarkan wajah dulu sebelum bisa checkin"
	case ErrTokenInvalid:
		return 400, "TOKEN_INVALID", "Sesi tidak valid, tap Check In lagi"
	case ErrTokenExpired:
		return 400, "TOKEN_EXPIRED", "Waktu habis (2 menit), tap Check In lagi"
	case ErrTokenUsed:
		return 400, "TOKEN_USED", "Token sudah digunakan"
	case ErrTokenNotVerified:
		return 400, "TOKEN_NOT_VERIFIED", "Verifikasi wajah dulu sebelum checkin"
	case ErrFaceMismatch:
		return 401, "FACE_MISMATCH", "Wajah tidak dikenali, coba lagi"
	case ErrMaxAttemptExceeded:
		return 429, "MAX_ATTEMPT_EXCEEDED", "Terlalu banyak percobaan gagal, hubungi HR"
	case ErrNoFaceDetected:
		return 400, "NO_FACE_DETECTED", "Tidak ada wajah terdeteksi, foto ulang"
	case ErrMultipleFaces:
		return 400, "MULTIPLE_FACES", "Foto hanya boleh satu wajah"
	case ErrEngineError:
		return 500, "ENGINE_ERROR", "Sistem verifikasi bermasalah, coba lagi"
	case ErrOutsideRadius:
		return 400, "OUTSIDE_RADIUS", "Kamu berada di luar area yang diizinkan"
	case ErrAlreadyAttendedEvent:
		return 409, "ALREADY_ATTENDED_EVENT", "Kamu sudah absen di event ini"
	case ErrQRTokenInvalid:
		return 400, "QR_TOKEN_INVALID", "QR Code tidak valid atau sudah kadaluarsa"
	case ErrQRNotEventType:
		return 400, "QR_NOT_EVENT_TYPE", "QR Code ini bukan untuk absen event"
	case ErrIncompletePoses:
		return 400, "INCOMPLETE_POSES", "Kirim kelima foto: front, left, right, up, down"
	case ErrPoseInvalid:
		return 400, "POSE_INVALID", "Nama pose tidak dikenali"
	default:
		return 500, "INTERNAL_ERROR", "Terjadi kesalahan server"
	}
}

// validateImageFile — validasi format dan ukuran file gambar
// Dipakai di RegisterFace, VerifyFace
func validateImageFile(c *fiber.Ctx, fieldName string) error {
	fh, err := c.FormFile(fieldName)
	if err != nil || fh == nil {
		return fiber.NewError(400, "MISSING_FIELD: "+fieldName+" wajib diisi")
	}
	ext := strings.ToLower(fh.Filename[strings.LastIndex(fh.Filename, "."):])
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return fiber.NewError(400, "Format file harus JPEG atau PNG")
	}
	if fh.Size > 5*1024*1024 {
		return fiber.NewError(400, "Ukuran file maksimal 5MB")
	}
	return nil
}

// ─────────────────────────────────────────
// GET /employee/onboarding-status
// ─────────────────────────────────────────

func (h *Handler) GetOnboardingStatus(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)
	result, err := h.Service.GetOnboardingStatus(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status": "error", "message": "Gagal mengambil status onboarding",
		})
	}
	return c.JSON(fiber.Map{"status": "success", "data": result})
}

// ─────────────────────────────────────────
// POST /employee/face/register
// Body: multipart/form-data, field: face_image
// ─────────────────────────────────────────

func (h *Handler) RegisterFace(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	files := make(map[string]*multipart.FileHeader)

	for _, pose := range RequiredPoses {
		fh, err := c.FormFile(pose)
		if err != nil || fh == nil {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"code":    "MISSING_FIELD",
				"message": fmt.Sprintf("Field '%s' wajib diisi dengan foto", pose),
			})
		}

		ext := strings.ToLower(fh.Filename[strings.LastIndex(fh.Filename, "."):])
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"code":    "INVALID_FORMAT",
				"message": fmt.Sprintf("Format file '%s' harus JPEG atau PNG", pose),
			})
		}
		if fh.Size > 5*1024*1024 {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"code":    "FILE_TOO_LARGE",
				"message": fmt.Sprintf("Ukuran file '%s' maksimal 5MB", pose),
			})
		}

		files[pose] = fh
	}

	result, err := h.Service.RegisterFace(employeeID, files)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status": "error", "code": code, "message": msg,
		})
	}

	return c.Status(201).JSON(fiber.Map{"status": "success", "data": result})
}

// ─────────────────────────────────────────
// GET /employee/face/status
// ─────────────────────────────────────────

func (h *Handler) GetFaceStatus(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)
	result, err := h.Service.GetFaceStatus(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status": "error", "message": "Gagal mengambil status wajah",
		})
	}
	return c.JSON(fiber.Map{"status": "success", "data": result})
}

// ─────────────────────────────────────────
// POST /attendance/face-token
// Tidak ada body — hanya JWT
// ─────────────────────────────────────────

func (h *Handler) GenerateFaceToken(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)
	result, err := h.Service.GenerateFaceToken(employeeID)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status": "error", "code": code, "message": msg,
		})
	}
	return c.JSON(fiber.Map{"status": "success", "data": result})
}

// ─────────────────────────────────────────
// POST /attendance/verify-face
// Body: multipart/form-data
//   face_token : string
//   face_image : file JPEG/PNG
// ─────────────────────────────────────────

func (h *Handler) VerifyFace(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	faceToken := strings.TrimSpace(c.FormValue("face_token"))
	if faceToken == "" {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "MISSING_FIELD", "message": "face_token wajib diisi",
		})
	}

	if err := validateImageFile(c, "face_image"); err != nil {
		fe := err.(*fiber.Error)
		return c.Status(fe.Code).JSON(fiber.Map{
			"status": "error", "message": fe.Message,
		})
	}
	fh, _ := c.FormFile("face_image")

	result, err := h.Service.VerifyFace(employeeID, faceToken, fh, c.IP())
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status": "error", "code": code, "message": msg,
		})
	}
	return c.JSON(fiber.Map{"status": "success", "data": result})
}

// ─────────────────────────────────────────
// POST /attendance/checkin
// Body: JSON
//   face_token : string (yang sudah face_verified = true)
//   latitude   : float64
//   longitude  : float64
// ─────────────────────────────────────────

func (h *Handler) Checkin(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var body struct {
		FaceToken string  `json:"face_token"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "message": "Body tidak valid",
		})
	}
	if body.FaceToken == "" || body.Latitude == 0 || body.Longitude == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "face_token, latitude, dan longitude wajib diisi",
		})
	}

	result, err := h.Service.Checkin(
		employeeID, body.FaceToken,
		body.Latitude, body.Longitude,
		c.IP(),
	)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status": "error", "code": code, "message": msg,
		})
	}
	return c.Status(201).JSON(fiber.Map{
		"status": "success", "message": "Checkin berhasil", "data": result,
	})
}

// ─────────────────────────────────────────
// POST /attendance/checkin-qr
// Body: JSON
//   qr_token  : string (dari scan QR event)
//   latitude  : float64
//   longitude : float64
// ─────────────────────────────────────────

func (h *Handler) CheckinQREvent(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var req CheckinQRRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "message": "Body tidak valid",
		})
	}
	if req.QRToken == "" || req.Latitude == 0 || req.Longitude == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "qr_token, latitude, dan longitude wajib diisi",
		})
	}

	result, err := h.Service.CheckinQREvent(employeeID, req, c.IP())
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status": "error", "code": code, "message": msg,
		})
	}
	return c.Status(201).JSON(fiber.Map{
		"status": "success", "message": "Absen event berhasil", "data": result,
	})
}
