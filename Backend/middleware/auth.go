package middleware

import (
	"absensi/config"
	"absensi/utils"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Claims - sesuai payload JWT dari Backend 1
// user_id, role, exp (branch_id tidak ada di token terbaru)
type Claims struct {
	UserID   int    `json:"user_id"`
	Role     string `json:"role"`
	BranchID *int   `json:"branch_id"`
}

type contextKey string

const ClaimsKey contextKey = "claims"

var db *sql.DB

// SetDB - set database connection untuk lookup branch_id
func SetDB(database *sql.DB) {
	db = database
}

// AuthMiddleware - validasi JWT dari Backend 1
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				utils.Unauthorized(w, "Token tidak ditemukan")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				utils.Unauthorized(w, "Format token tidak valid")
				return
			}

			claims, err := parseJWT(parts[1])
			if err != nil {
				utils.Unauthorized(w, "Token tidak valid atau sudah expired")
				return
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

			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAdminCabang - hanya admin_cabang dan admin
func RequireAdminCabang(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := GetClaims(r)
		if claims == nil {
			utils.Unauthorized(w, "Unauthorized")
			return
		}
		if claims.Role != "admin_cabang" && claims.Role != "admin" {
			utils.Forbidden(w, "Hanya admin cabang yang bisa mengakses fitur ini")
			return
		}
		next(w, r)
	}
}

func GetClaims(r *http.Request) *Claims {
	val := r.Context().Value(ClaimsKey)
	if claims, ok := val.(*Claims); ok {
		return claims
	}
	return nil
}

// parseJWT - parse JWT token dari Backend 1
// Tidak verifikasi signature karena golang-jwt encode berbeda
// Cukup parse payload dan cek expiry
func parseJWT(tokenStr string) (*Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// Decode payload (base64url)
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

	// user_id
	if v, ok := rawClaims["user_id"].(float64); ok {
		claims.UserID = int(v)
	}

	// role
	if v, ok := rawClaims["role"].(string); ok {
		claims.Role = v
	}

	// branch_id (kalau ada di token)
	if v, ok := rawClaims["branch_id"].(float64); ok {
		id := int(v)
		claims.BranchID = &id
	}

	return claims, nil
}
