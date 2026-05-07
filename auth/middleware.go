package auth

import (
	"absensi_karyawan/utils"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(fiber.Map{"error": "missing token"})
	}

	// split "Bearer token"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(401).JSON(fiber.Map{"error": "invalid token format"})
	}

	token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
		return utils.SECRET, nil
	})

	if err != nil || !token.Valid {
		return c.Status(401).JSON(fiber.Map{"error": "invalid token"})
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.SendStatus(401)
	}

	// JWT Simpan number sebagai float64 - konversi ke int
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "invalid token claims"})
	}

	// simpan ke context
	c.Locals("user_id", int(userIDFloat))
	c.Locals("role", claims["role"])

	// branch_id juga disimpan di JWT supaya tidak perlu query DB lagi
	// Dibutuhkan oleh GET /qr/today dan attendance endpoints
	if branchIDFloat, ok := claims["branch_id"].(float64); ok {
		c.Locals("branch_id", int(branchIDFloat))
	}

	return c.Next()
}

func AdminOnly(c *fiber.Ctx) error {
	role, ok := c.Locals("role").(string)
	if !ok {
		return c.Status(403).JSON(fiber.Map{"error": "forbidden"})
	}

	if role != "super_admin" && role != "admin_cabang" && role != "admin" {
		return c.Status(403).JSON(fiber.Map{"error": "admin only"})
	}

	return c.Next()
}
