package auth

import (
	"errors"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// Claims — struct yang dipakai BE2 untuk ambil data dari context
// Ditambahkan di sini supaya BE2 bisa pakai GetClaims(c)
type Claims struct {
	UserID   int    `json:"user_id"`
	Role     string `json:"role"`
	Tipe     string `json:"tipe"`
	BranchID *int   `json:"branch_id"`
}

// AuthMiddleware — validasi JWT dan simpan ke context
// Simpan DUA format:
// 1. c.Locals("user_id"), c.Locals("role"), c.Locals("branch_id") → untuk BE1
// 2. c.Locals("claims") sebagai *Claims                            → untuk BE2
func AuthMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"message": "Token tidak ditemukan",
		})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"message": "Format token tidak valid",
		})
	}

	token, err := jwt.Parse(parts[1], func(t *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {

		// DEBUG
		// log.Println("JWT ERROR:", err)

		// TOKEN EXPIRED
		if errors.Is(err, jwt.ErrTokenExpired) {
			return c.Status(401).JSON(fiber.Map{
				"status":  "error",
				"code":    "TOKEN_EXPIRED",
				"message": "Session habis, silakan login kembali",
			})
		}

		// TOKEN INVALID
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"code":    "TOKEN_INVALID",
			"message": "Token tidak valid",
		})
	}

	if !token.Valid {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"code":    "TOKEN_INVALID",
			"message": "Token tidak valid",
		})
	}

	jwtClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"message": "Token claims tidak valid",
		})
	}

	userIDFloat, ok := jwtClaims["user_id"].(float64)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"message": "Token tidak valid",
		})
	}

	userID := int(userIDFloat)
	role, _ := jwtClaims["role"].(string)
	tipe, _ := jwtClaims["tipe"].(string)

	// ── Format BE1 ────────────────────────────────────────
	c.Locals("user_id", userID)
	c.Locals("role", role)
	c.Locals("tipe", tipe)

	var branchIDPtr *int
	if branchIDFloat, ok := jwtClaims["branch_id"].(float64); ok {
		branchID := int(branchIDFloat)
		c.Locals("branch_id", branchID)
		branchIDPtr = &branchID
	}

	// ── Format BE2 — simpan sebagai *Claims ───────────────
	// Dibutuhkan oleh handlers/employee_handler.go yang pakai GetClaims(c)
	c.Locals("claims", &Claims{
		UserID:   userID,
		Role:     role,
		Tipe:     tipe,
		BranchID: branchIDPtr,
	})

	return c.Next()
}

// GetClaims — dipakai oleh handlers BE2
func GetClaims(c *fiber.Ctx) *Claims {
	claims, _ := c.Locals("claims").(*Claims)
	return claims
}

// ── Role middleware ────────────────────────────────────────

func RequireSuperAdmin(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "super_admin" {
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"code":    "FORBIDDEN",
			"message": "Hanya super admin yang bisa mengakses fitur ini",
		})
	}
	return c.Next()
}

func RequireAdmin(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)
	if role != "super_admin" && role != "admin_cabang" && role != "admin" {
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"code":    "FORBIDDEN",
			"message": "Hanya admin yang bisa mengakses fitur ini",
		})
	}
	return c.Next()
}

func RequireAdminCabangOnly(c *fiber.Ctx) error {
	role, _ := c.Locals("role").(string)

	if role == "super_admin" {
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"code":    "FORBIDDEN",
			"message": "Super admin tidak bisa generate QR. Gunakan akun admin cabang.",
		})
	}
	if role != "admin_cabang" && role != "admin" {
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"code":    "FORBIDDEN",
			"message": "Hanya admin cabang yang bisa generate QR",
		})
	}

	branchID, _ := c.Locals("branch_id").(int)
	if branchID == 0 {
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"code":    "NO_BRANCH",
			"message": "Akun ini tidak terdaftar di cabang manapun",
		})
	}
	return c.Next()
}

func GetUserID(c *fiber.Ctx) int {
	id, _ := c.Locals("user_id").(int)
	return id
}

func GetRole(c *fiber.Ctx) string {
	role, _ := c.Locals("role").(string)
	return role
}

func GetBranchID(c *fiber.Ctx) int {
	id, _ := c.Locals("branch_id").(int)
	return id
}

func RequirePusatRole(c *fiber.Ctx) error {

	role, _ := c.Locals("role").(string)
	tipe, _ := c.Locals("tipe").(string)

	// hanya untuk user tipe pusat
	if tipe != "pusat" {
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"code":    "FORBIDDEN",
			"message": "Fitur ini hanya untuk admin pusat",
		})
	}

	// role harus admin atau super_admin
	if role != "admin" && role != "super_admin" {
		return c.Status(403).JSON(fiber.Map{
			"status":  "error",
			"code":    "FORBIDDEN",
			"message": "Role tidak memiliki akses",
		})
	}

	return c.Next()
}
