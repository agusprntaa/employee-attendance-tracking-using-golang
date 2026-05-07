package employee

import (
	"absensi_karyawan/utils"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	Repo *Repository
}

// ─────────────────────────────────────────
// GET /employee/profile
// Data profil karyawan yang sedang login
// ─────────────────────────────────────────
func (h *Handler) GetProfile(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	profile, err := h.Repo.GetProfile(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil profil",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   profile,
	})
}

// ─────────────────────────────────────────
// PATCH /employee/change-password
// Ubah password karyawan sendiri
// ─────────────────────────────────────────
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Format request tidak valid",
		})
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Password lama dan baru wajib diisi",
		})
	}

	if len(req.NewPassword) < 6 {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Password baru minimal 6 karakter",
		})
	}

	// Ambil password hash saat ini dari DB
	currentHash, err := h.Repo.GetPasswordHash(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal memverifikasi password",
		})
	}

	// Verifikasi password lama
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.OldPassword)); err != nil {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"code":    "WRONG_OLD_PASSWORD",
			"message": "Password lama tidak sesuai",
		})
	}

	// Hash password baru
	newHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal memproses password baru",
		})
	}

	// Update ke DB
	if err := h.Repo.UpdatePassword(employeeID, newHash); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menyimpan password baru",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Password berhasil diubah",
	})
}
