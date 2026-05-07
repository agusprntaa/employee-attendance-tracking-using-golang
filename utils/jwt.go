package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var SECRET = []byte(os.Getenv("JWT_SECRET"))

// / GenerateAccessToken — include branchID supaya GET /qr/today tidak perlu query DB
func GenerateAccessToken(userID int, role string, branchID int) (string, error) {
	claims := jwt.MapClaims{
		"user_id":   userID,
		"role":      role,
		"branch_id": branchID,
		"exp":       time.Now().Add(15 * time.Minute).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(SECRET)
}

func GenerateRefreshToken(userID int) (string, time.Time, error) {
	exp := time.Now().Add(7 * 24 * time.Hour)
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     exp.Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(SECRET)
	return token, exp, err
}
