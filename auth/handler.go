package auth

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Service *Service
}

// ============================================================
// Login
//
// PERUBAHAN:
//   - Service.Login sekarang return 5 nilai (tambah mustChangePassword)
//   - Response tambah field must_change_password
//   - FE wajib cek field ini: kalau true → redirect ke /change-password
// ============================================================

func (h *Handler) Login(c *fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	access, refresh, user, mustChangePassword, err := h.Service.Login(body.Username, body.Password)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"token":         access,
			"refresh_token": refresh,
			// expires_in: FE pakai ini untuk set timer auto-refresh
			// Nilainya 180 detik (3 menit), FE refresh di detik ke ~150
			"expires_in":           180,
			"must_change_password": mustChangePassword,
			"user": fiber.Map{
				"id":        user.ID,
				"name":      user.Name,
				"role":      user.Role,
				"tipe":      user.EmployeeType,
				"branch_id": user.BranchID,
			},
		},
	})
}

// ============================================================
// Refresh
// Tidak berubah dari sisi handler — perubahan ada di service
// ============================================================

func (h *Handler) Refresh(c *fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	if body.RefreshToken == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "refresh_token wajib diisi",
		})
	}

	newAccess, err := h.Service.Refresh(body.RefreshToken)
	if err != nil {
		// Refresh gagal = sesi habis, FE harus redirect ke login
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"code":    "SESSION_EXPIRED",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"token":      newAccess,
			"expires_in": 180, // FE update timer countdown
		},
	})
}

// ============================================================
// Logout
//
// PERUBAHAN:
//   - Sekarang endpoint ini perlu refresh_token di body
//     supaya bisa hapus token yang spesifik dari DB
// ============================================================

func (h *Handler) Logout(c *fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "invalid request",
		})
	}

	h.Service.Logout(body.RefreshToken)

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "logout successful",
	})
}

// ============================================================
// CreateUser — tidak berubah
// ============================================================

func (h *Handler) CreateUser(c *fiber.Ctx) error {
	var body struct {
		Username   string `json:"username"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		Name       string `json:"name"`
		Role       string `json:"role"`
		Tipe       string `json:"tipe"`
		BranchID   int    `json:"branch_id"`
		DivisionID int    `json:"division_id"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	if body.Username == "" || body.Password == "" || body.Name == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Username, password, dan nama wajib diisi",
		})
	}

	if len(body.Password) < 6 {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Password minimal 6 karakter",
		})
	}

	requesterRole, _ := c.Locals("role").(string)
	requesterBranchID, _ := c.Locals("branch_id").(int)

	switch requesterRole {
	case "super_admin":
		if body.Role == "super_admin" {
			return c.Status(403).JSON(fiber.Map{
				"status":  "error",
				"message": "Tidak bisa membuat akun super admin baru",
			})
		}
		if body.Role == "admin_cabang" && body.BranchID == 0 {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"message": "Branch ID wajib diisi saat membuat admin cabang",
			})
		}

	case "admin_cabang", "admin":
		if body.Role != "karyawan" {
			return c.Status(403).JSON(fiber.Map{
				"status":  "error",
				"code":    "FORBIDDEN",
				"message": "Admin cabang hanya bisa membuat akun karyawan",
			})
		}
		body.BranchID = requesterBranchID
		body.Tipe = "cabang"

	default:
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"message": "Tidak memiliki akses untuk membuat user",
		})
	}

	if body.Tipe == "" {
		if body.Role == "admin_cabang" || body.Role == "karyawan" {
			body.Tipe = "cabang"
		} else {
			body.Tipe = "pusat"
		}
	}

	err := h.Service.CreateUser(
		body.Username,
		body.Password,
		body.Name,
		body.Role,
		body.Tipe,
		body.BranchID,
		body.DivisionID,
	)
	if err != nil {
		if err.Error() == "username already taken" {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"code":    "USERNAME_TAKEN",
				"message": "Username sudah digunakan",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal membuat user: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "user created",
		// Informasikan ke admin bahwa password sementara sudah di-set
		// dan karyawan harus ganti saat login pertama
		"note": "Karyawan wajib mengganti password saat login pertama",
	})
}
