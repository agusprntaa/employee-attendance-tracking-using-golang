package middleware

import (
	"absensi/config"
	"absensi/utils"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Claims struct {
	UserID   int    `json:"user_id"`
	Role     string `json:"role"`
	BranchID *int   `json:"branch_id"`
}

var db *sql.DB

func SetDB(database *sql.DB) {
	db = database
}

// AuthMiddleware - validasi JWT dari Backend 1
func AuthMiddleware(cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return utils.Unauthorized(c, "Token tidak ditemukan")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return utils.Unauthorized(c, "Format token tidak valid")
		}

		claims, err := parseJWT(parts[1])
		if err != nil {
			return utils.Unauthorized(c, "Token tidak valid atau sudah expired")
		}

		// Kalau branch_id tidak ada di token, ambil dari DB
		if claims.BranchID == nil && db != nil {
			var branchID int
			err := db.QueryRow(`
				SELECT branch_id FROM employees 
				WHERE id = $1 AND branch_id IS NOT NULL
			`, claims.UserID).Scan(&branchID)
			if err == nil {
				claims.BranchID = &branchID
			}
		}

		// Simpan claims ke context
		c.Locals("claims", claims)
		return c.Next()
	}
}

// RequireAdminCabang - hanya admin_cabang dan admin
func RequireAdminCabang(c *fiber.Ctx) error {
	claims := GetClaims(c)
	if claims == nil {
		return utils.Unauthorized(c, "Unauthorized")
	}
	if claims.Role != "admin_cabang" && claims.Role != "admin" {
		return utils.Forbidden(c, "Hanya admin cabang yang bisa mengakses fitur ini")
	}
	return c.Next()
}

func GetClaims(c *fiber.Ctx) *Claims {
	claims, ok := c.Locals("claims").(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// parseJWT - parse JWT token dari Backend 1
func parseJWT(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}

	var rawClaims map[string]interface{}
	if err := json.Unmarshal(payload, &rawClaims); err != nil {
		return nil, errors.New("invalid payload json")
	}

	// Cek expiry
	if exp, ok := rawClaims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, errors.New("token expired")
		}
	}

	claims := &Claims{}

	if v, ok := rawClaims["user_id"].(float64); ok {
		claims.UserID = int(v)
	}
	if v, ok := rawClaims["role"].(string); ok {
		claims.Role = v
	}
	if v, ok := rawClaims["branch_id"].(float64); ok {
		id := int(v)
		claims.BranchID = &id
	}

	return claims, nil
}
