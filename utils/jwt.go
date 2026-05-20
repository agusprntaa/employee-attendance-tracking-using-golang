package utils

import (
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================
// KONSTANTA DURASI TOKEN
// ============================================================
//
//  Access token  : 3 menit — SAMA untuk semua role
//  Refresh token : berbeda per role
//    karyawan    : 15 menit  (900 detik)
//    admin_cabang: 12 jam    (43200 detik)
//    super_admin : 12 jam    (43200 detik)
//
// Cara kerja sesi:
//   - Setiap request kirim access token di header Authorization
//   - Kalau access token expired (3 mnt) → FE pakai refresh token
//     untuk minta access token baru ke POST /refresh
//   - Kalau refresh token expired (15 mnt / 12 jam) → harus login ulang
// ============================================================

const (
	AccessTokenDuration = 3 * time.Minute

	RefreshDurationKaryawan = 15 * time.Minute
	RefreshDurationAdmin    = 12 * time.Hour
)

// refreshDurationByRole — tentukan durasi refresh token berdasarkan role
func refreshDurationByRole(role string) time.Duration {
	switch role {
	case "admin_cabang", "admin", "super_admin":
		return RefreshDurationAdmin // 12 jam
	default:
		return RefreshDurationKaryawan // 15 menit (karyawan)
	}
}

// ============================================================
// GENERATE ACCESS TOKEN
// Secret: JWT_ACCESS_SECRET (beda dengan refresh)
// Expiry: 3 menit untuk semua role
// ============================================================

func GenerateAccessToken(userID int, role, tipe string, branchID int) (string, error) {
	return GenerateAccessTokenWithFlags(userID, role, tipe, branchID, false)
}

// GenerateAccessTokenWithFlags — versi extended, tambahkan must_change_password
// ke dalam claim. Dipakai saat login pertama kali (F4).
// Karena AT hanya 3 menit, aman untuk embed flag ini di token.
func GenerateAccessTokenWithFlags(userID int, role, tipe string, branchID int, mustChangePassword bool) (string, error) {
	secret := os.Getenv("JWT_ACCESS_SECRET")

	claims := jwt.MapClaims{
		"user_id":              userID,
		"role":                 role,
		"tipe":                 tipe,
		"branch_id":            branchID,
		"must_change_password": mustChangePassword,
		"exp":                  time.Now().Add(AccessTokenDuration).Unix(),
		"iat":                  time.Now().Unix(),
		"type":                 "access",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ============================================================
// GENERATE REFRESH TOKEN
// Secret: JWT_REFRESH_SECRET (beda dengan access)
// Expiry: 15 menit (karyawan) atau 12 jam (admin)
//
// Return: tokenString, expiresAt, error
// expiresAt → disimpan ke kolom expires_at di tabel refresh_tokens
// ============================================================

func GenerateRefreshToken(userID int, role string) (string, time.Time, error) {
	secret := os.Getenv("JWT_REFRESH_SECRET")
	duration := refreshDurationByRole(role)
	exp := time.Now().Add(duration)

	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     exp.Unix(),
		"iat":     time.Now().Unix(),
		"type":    "refresh",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	return signed, exp, err
}

// ============================================================
// HASH & COMPARE PASSWORD
// ============================================================

func HashPassword(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hashed), err
}

func CheckPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

// ============================================================
// HASH & COMPARE TOKEN (untuk refresh token di DB)
// Refresh token di-hash sebelum disimpan ke DB.
// Tujuan: kalau DB bocor, token tidak bisa langsung dipakai.
// ============================================================

func HashToken(token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash), nil
}

func CheckToken(plain, hashed string) bool {
	hash := sha256.Sum256([]byte(plain))
	return fmt.Sprintf("%x", hash) == hashed
}
