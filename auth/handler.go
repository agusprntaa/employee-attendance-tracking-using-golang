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
				"id":   user.ID,
				"name": user.Name,
				"role": user.Role,         // "super_admin" / "admin_cabang" / "karyawan"
				"tipe": user.EmployeeType, // "pusat" / "cabang"
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
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Role     string `json:"role"`
		Tipe     string `json:"tipe"`
	}

	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
	}

	err := h.Service.CreateUser(body.Email, body.Password, body.Name, body.Role, body.Tipe)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "user created",
	})
}
