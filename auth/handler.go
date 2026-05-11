package auth

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Service *Service
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	// Service.Login sekarang return (accessToken, refreshToken, user, error)
	access, refresh, user, err := h.Service.Login(body.Username, body.Password)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}

	// Kembalikan token + data user sekaligus
	// Frontend butuh role & tipe untuk redirect ke halaman yang sangat benar
	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"token":         access,
			"refresh_token": refresh,
			"user": fiber.Map{
				"id":        user.ID,
				"name":      user.Name,
				"role":      user.Role,         // "super_admin" / "admin_cabang" / "karyawan"
				"tipe":      user.EmployeeType, // "pusat" / "cabang"
				"branch_id": user.BranchID,
			},
		},
	})
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	newAccess, err := h.Service.Refresh(body.RefreshToken)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"token": newAccess,
		},
	})
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	h.Service.Logout(body.RefreshToken)

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "logout successful",
	})
}

func (h *Handler) CreateUser(c *fiber.Ctx) error {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		Tipe     string `json:"tipe"`
		BranchID int    `json:"branch_id"`

		DivisionID int `json:"division_id"` // opsional
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	// Validasi field wajib
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

	// Ambil role dan branch_id pembuat dari JWT
	// Ini diset oleh AuthMiddleware saat request masuk
	requesterRole, _ := c.Locals("role").(string)
	requesterBranchID, _ := c.Locals("branch_id").(int)

	switch requesterRole {
	case "super_admin":
		// super_admin bisa buat semua role kecuali super_admin lagi
		if body.Role == "super_admin" {
			return c.Status(403).JSON(fiber.Map{
				"status":  "error",
				"message": "Tidak bisa membuat akun super admin baru",
			})
		}
		// Kalau buat admin_cabang, branch_id wajib diisi
		if body.Role == "admin_cabang" && body.BranchID == 0 {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"message": "Branch ID wajib diisi saat membuat admin cabang",
			})
		}

	case "admin_cabang", "admin":
		// admin_cabang hanya bisa buat karyawan
		if body.Role != "karyawan" {
			return c.Status(403).JSON(fiber.Map{
				"status":  "error",
				"code":    "FORBIDDEN",
				"message": "Admin cabang hanya bisa membuat akun karyawan",
			})
		}
		// branch_id otomatis dari token admin yang sedang login
		// tidak perlu diisi di body
		body.BranchID = requesterBranchID
		body.Tipe = "cabang" // karyawan yang dibuat admin_cabang selalu tipe cabang

	default:
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"message": "Tidak memiliki akses untuk membuat user",
		})
	}

	// Set default tipe jika kosong
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
		// Cek apakah username sudah dipakai
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
	})
}
