package face

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Service *Service
}

func errorMessage(err error) (int, string, string) {
	switch err {
	case ErrAlreadyCheckedIn:
		return 400, "ALREADY_CHECKED_IN", "Kamu sudah checkin hari ini"
	case ErrFaceNotRegistered:
		return 403, "FACE_NOT_REGISTERED", "Daftarkan wajah dulu sebelum bisa checkin"
	case ErrFaceNotVerified: // ✦ BARU
		return 403, "FACE_NOT_VERIFIED", "Verifikasi wajah dulu sebelum checkin"
	case ErrTokenInvalid:
		return 400, "TOKEN_INVALID", "Sesi tidak valid, tap Check In lagi"
	case ErrTokenExpired:
		return 400, "TOKEN_EXPIRED", "Waktu habis (2 menit), tap Check In lagi"
	case ErrTokenUsed:
		return 400, "TOKEN_USED", "Token sudah digunakan"
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
	default:
		return 500, "INTERNAL_ERROR", "Terjadi kesalahan server"
	}
}

// GET /employee/onboarding-status
func (h *Handler) GetOnboardingStatus(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)
	result, err := h.Service.GetOnboardingStatus(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil status onboarding",
		})
	}
	return c.JSON(fiber.Map{"status": "success", "data": result})
}

// POST /employee/face/register
func (h *Handler) RegisterFace(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	fileHeader, err := c.FormFile("face_image")
	if err != nil || fileHeader == nil {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "MISSING_FIELD",
			"message": "face_image wajib diisi",
		})
	}

	allowedExt := map[string]bool{".jpg": true, ".jpeg": true, ".png": true}
	dotIndex := strings.LastIndex(fileHeader.Filename, ".")
	if dotIndex < 0 {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "INVALID_FORMAT",
			"message": "Format file harus JPEG atau PNG",
		})
	}
	ext := strings.ToLower(fileHeader.Filename[dotIndex:])
	if !allowedExt[ext] {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "INVALID_FORMAT",
			"message": "Format file harus JPEG atau PNG",
		})
	}

	result, err := h.Service.RegisterFace(employeeID, fileHeader)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{"status": "error", "code": code, "message": msg})
	}
	return c.Status(201).JSON(fiber.Map{"status": "success", "data": result})
}

// GET /employee/face/status
func (h *Handler) GetFaceStatus(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)
	result, err := h.Service.GetFaceStatus(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"status": "error", "message": "Gagal mengambil status wajah"})
	}
	return c.JSON(fiber.Map{"status": "success", "data": result})
}

// POST /attendance/face-token — Step 1
// Generate token, face_verified=false di DB
func (h *Handler) GenerateFaceToken(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	result, err := h.Service.GenerateFaceToken(employeeID)
	if err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
			"detail":  err.Error(), // hapus di production
		})
	}
	return c.JSON(fiber.Map{"status": "success", "data": result})
}

// ✦ DIUPDATE — POST /attendance/verify-face — Step 2
//
// Verifikasi wajah saja. Jika cocok → face_verified=true di face_tokens.
// is_used tetap false — token masih bisa dipakai untuk /attendance/checkin.
//
// Body: multipart/form-data
//
//	face_token : string
//	face_image : file JPEG/PNG
//
// Response 200:
// {"face_token": "...", "confidence_score": 0.92, "verified": true}
func (h *Handler) VerifyFace(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	faceToken := strings.TrimSpace(c.FormValue("face_token"))
	if faceToken == "" {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "MISSING_FIELD",
			"message": "face_token wajib diisi",
		})
	}

	fileHeader, err := c.FormFile("face_image")
	if err != nil || fileHeader == nil {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "MISSING_FIELD",
			"message": "face_image wajib diisi",
		})
	}

	allowedExt := map[string]bool{".jpg": true, ".jpeg": true, ".png": true}
	dotIndex := strings.LastIndex(fileHeader.Filename, ".")
	if dotIndex < 0 {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "INVALID_FORMAT",
			"message": "Format file harus JPEG atau PNG",
		})
	}
	ext := strings.ToLower(fileHeader.Filename[dotIndex:])
	if !allowedExt[ext] {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "INVALID_FORMAT",
			"message": "Format file harus JPEG atau PNG",
		})
	}

	const maxSize = 5 * 1024 * 1024
	if fileHeader.Size > maxSize {
		return c.Status(400).JSON(fiber.Map{
			"status": "error", "code": "FILE_TOO_LARGE",
			"message": "Ukuran file maksimal 5MB",
		})
	}

	result, err := h.Service.VerifyFace(employeeID, faceToken, fileHeader, c.IP())
	if err != nil {
		log.Printf("[VERIFY_FACE] employeeID=%d error=%v", employeeID, err)
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{"status": "error", "code": code, "message": msg})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Wajah berhasil diverifikasi",
		"data":    result,
	})
}
