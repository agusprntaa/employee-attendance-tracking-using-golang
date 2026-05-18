package utils

import (
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func getJWTSecret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

// GenerateAccessToken
func GenerateAccessToken(userID int, role string, tipe string, branchID int) (string, error) {

	now := NowWITA()

	claims := jwt.MapClaims{
		"user_id":   userID,
		"role":      role,
		"tipe":      tipe,
		"branch_id": branchID,
		"exp":       now.Add(15 * time.Minute).Unix(),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString(getJWTSecret())
}

func GenerateRefreshToken(userID int) (string, time.Time, error) {

	now := NowWITA()

	exp := now.Add(7 * 24 * time.Hour)

	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     exp.Unix(),
	}

	token, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString(getJWTSecret())

	return token, exp, err
}
