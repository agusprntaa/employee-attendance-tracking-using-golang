package auth

import (
	"absensi_karyawan/utils"
	"errors"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	Repo *Repository
}

// User struct — tidak berubah
type User struct {
	ID           int
	Name         string
	Username     string
	Role         string
	EmployeeType string
	BranchID     int
}

// ============================================================
// Login
//
// PERUBAHAN:
//   - Pakai FindUserWithPasswordFlag supaya dapat must_change_password
//   - GenerateRefreshToken sekarang terima role (untuk expiry beda)
//   - SaveRefreshToken sekarang terima lebih banyak param
//   - Return tambah mustChangePassword bool
// ============================================================

func (s *Service) Login(username, password string) (string, string, *User, bool, error) {
	// Ambil user + flag must_change_password
	user, hashed, mustChange, err := s.Repo.FindUserWithPasswordFlag(username)
	if err != nil {
		return "", "", nil, false, errors.New("user not found")
	}

	// Validasi password
	if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) != nil {
		return "", "", nil, false, errors.New("wrong password")
	}

	// Generate access token (3 menit, semua role sama)
	// Embed mustChange ke claim supaya middleware bisa cek tanpa query DB
	access, err := utils.GenerateAccessTokenWithFlags(user.ID, user.Role, user.EmployeeType, user.BranchID, mustChange)
	if err != nil {
		return "", "", nil, false, err
	}

	// Generate refresh token (expiry berdasarkan role)
	// karyawan → 15 menit | admin/super_admin → 12 jam
	refresh, exp, err := utils.GenerateRefreshToken(user.ID, user.Role)
	if err != nil {
		return "", "", nil, false, err
	}

	// Simpan refresh token (di-hash) ke DB
	// Pakai UpsertRefreshToken untuk single-session (hapus token lama dulu)
	// Ganti ke SaveRefreshToken kalau mau multi-device
	err = s.Repo.UpsertRefreshToken(
		user.ID,
		refresh,
		user.Role,
		user.EmployeeType,
		user.BranchID,
		exp,
	)
	if err != nil {
		return "", "", nil, false, err
	}

	return access, refresh, user, mustChange, nil
}

// ============================================================
// Refresh
//
// PERUBAHAN:
//   - Decode JWT refresh token dulu → dapat user_id & role
//   - Gunakan JWT_REFRESH_SECRET (bukan JWT_SECRET lama)
//   - Validasi token dengan compare hash di DB
//   - Generate access token baru (refresh token TIDAK diganti)
// ============================================================

func (s *Service) Refresh(rawRefreshToken string) (string, error) {
	// 1. Decode JWT refresh token untuk dapat user_id
	//    (tanpa verify expiry dulu — kita cek DB sebagai sumber kebenaran)
	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")

	parsed, err := jwt.Parse(rawRefreshToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(refreshSecret), nil
	})

	if err != nil {
		// Token expired / invalid signature → sesi habis
		return "", errors.New("refresh token expired atau tidak valid, silakan login kembali")
	}

	if !parsed.Valid {
		return "", errors.New("refresh token tidak valid")
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("token claims tidak valid")
	}

	// Pastikan ini benar-benar refresh token, bukan access token
	if tokenType, _ := claims["type"].(string); tokenType != "refresh" {
		return "", errors.New("token type tidak valid")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return "", errors.New("token tidak valid")
	}
	userID := int(userIDFloat)

	// 2. Cari token yang cocok di DB (compare hash)
	role, tipe, branchID, err := s.Repo.ValidateAndGetToken(userID, rawRefreshToken)
	if err != nil {
		// Token tidak ada di DB → kemungkinan sudah logout atau dicuri
		return "", errors.New("sesi tidak ditemukan, silakan login kembali")
	}

	// 3. Generate access token baru
	newAccess, err := utils.GenerateAccessToken(userID, role, tipe, branchID)
	if err != nil {
		return "", err
	}

	return newAccess, nil
}

// ============================================================
// Logout
//
// PERUBAHAN:
//   - Perlu userID sekarang untuk cari token di DB
//   - Decode JWT refresh token untuk dapat userID tanpa hit DB dulu
// ============================================================

func (s *Service) Logout(rawRefreshToken string) {
	if rawRefreshToken == "" {
		return
	}

	// Decode tanpa validasi expiry untuk dapat user_id saja
	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	parsed, err := jwt.Parse(rawRefreshToken, func(t *jwt.Token) (interface{}, error) {
		return []byte(refreshSecret), nil
	}, jwt.WithoutClaimsValidation())

	if err != nil || !parsed.Valid {
		return
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return
	}

	s.Repo.DeleteRefreshToken(rawRefreshToken, int(userIDFloat))
}

// ============================================================
// CreateUser
//
// PERUBAHAN:
//   - must_change_password = true otomatis dari repository
//   - Tidak ada perubahan di sini, logikanya di repo
// ============================================================

func (s *Service) CreateUser(username, password, name, role, tipe string, branchID, divisionID int) error {
	exists, err := s.Repo.UsernameExists(username)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("username already taken")
	}

	hashed, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	return s.Repo.CreateUser(username, hashed, name, role, tipe, branchID, divisionID)
}

