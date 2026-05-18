package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"os"
)

// GenerateQRToken — generate HMAC token
// Format payload: "branchID|tanggal|slotWaktu"
// contoh: "1|2026-05-12|4"
//
// PENTING: pakai time.Now() bukan time.Now().UTC()
// supaya konsisten dengan BE2 yang juga pakai time.Now()
func GenerateQRToken(branchID int, date string, slotWaktu int) string {
	payload := fmt.Sprintf("%d|%s|%d", branchID, date, slotWaktu)
	secret := []byte(os.Getenv("HMAC_SECRET"))

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))

	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ValidateQRToken — validasi token dengan toleransi 1 slot sebelumnya
//
// Kenapa cek 2 slot?
// Token dibuat di menit ke-8 (slot 2), berganti di menit ke-9 (slot 3)
// Kalau karyawan scan tepat di menit ke-9, token slot 2 masih diterima
func ValidateQRToken(token string, branchID int, date string) bool {
	// Pakai local time — sama dengan BE2
	now := NowWITA()

	slotNow := now.Minute() / 3

	slotPrev := slotNow - 1
	if slotPrev < 0 {
		slotPrev = 19 // 60 menit / 3
	}

	tokenNow := GenerateQRToken(branchID, date, slotNow)
	tokenPrev := GenerateQRToken(branchID, date, slotPrev)

	// Debug log — hapus setelah confirmed working
	secret := os.Getenv("HMAC_SECRET")
	log.Printf("=== QR VALIDATE DEBUG ===")
	log.Printf("Local time    : %s", now.Format("2006-01-02 15:04:05"))
	log.Printf("Menit         : %d → slot now=%d, slot prev=%d", now.Minute(), slotNow, slotPrev)
	log.Printf("Branch ID     : %d", branchID)
	log.Printf("Date          : %s", date)
	log.Printf("HMAC_SECRET   : len=%d, kosong=%v", len(secret), secret == "")
	log.Printf("Payload now   : %d|%s|%d", branchID, date, slotNow)
	log.Printf("Token diterima: %s", token)
	log.Printf("Token(now)    : %s", tokenNow)
	log.Printf("Token(prev)   : %s", tokenPrev)
	log.Printf("Match now     : %v", hmac.Equal([]byte(token), []byte(tokenNow)))
	log.Printf("Match prev    : %v", hmac.Equal([]byte(token), []byte(tokenPrev)))
	log.Printf("=========================")

	return hmac.Equal([]byte(token), []byte(tokenNow)) ||
		hmac.Equal([]byte(token), []byte(tokenPrev))
}
