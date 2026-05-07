package auth

import (
	"absensi_karyawan/utils"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	Repo *Repository
}

// User struct untuk dikirim ke handler (dan ke frontend)
type User struct {
	ID           int
	Name         string
	Username     string
	Role         string
	EmployeeType string
	BranchID     int
}

func (s *Service) Login(username, password string) (string, string, *User, error) {
	user, hashed, err := s.Repo.FindUser(username)
	if err != nil {
		return "", "", nil, errors.New("user not found")
	}

	// bandingkan password
	if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password)) != nil {
		return "", "", nil, errors.New("wrong password")
	}

	// generate tokens
	access, err := utils.GenerateAccessToken(user.ID, user.Role, user.BranchID)
	if err != nil {
		return "", "", nil, err
	}

	refresh, exp, err := utils.GenerateRefreshToken(user.ID)
	if err != nil {
		return "", "", nil, err
	}

	// simpan refresh token ke DB
	s.Repo.SaveRefreshToken(user.ID, refresh, exp)

	return access, refresh, user, nil
}

func (s *Service) Refresh(oldToken string) (string, error) {
	// Sekarang ValidateRefreshToken return 4 nilai — userID, role, branchID, error
	userID, role, branchID, err := s.Repo.ValidateRefreshToken(oldToken)
	if err != nil {
		return "", errors.New("invalid refresh token")
	}
	// Generate token baru dengan role + branchID yang benar dari DB
	newAccess, err := utils.GenerateAccessToken(userID, role, branchID)
	if err != nil {
		return "", err
	}

	return newAccess, nil
}

func (s *Service) Logout(refreshToken string) {
	s.Repo.DeleteRefreshToken(refreshToken)
}

func (s *Service) CreateUser(username, password, name, role, tipe string) error {
	hashed, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	return s.Repo.CreateUser(username, hashed, name, role, tipe)
}
