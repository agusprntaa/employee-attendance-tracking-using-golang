package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"time"
)

// GenerateQRToken membuat token HMAC untuk QR code harian
// Format payload: "branchID|tanggal" contoh: "1|2026-04-27"
func GenerateQRToken(branchID int, date string, slotWaktu int) string {
	// Format payload sama persis dengan BE 2
	payload := fmt.Sprintf("%d|%s|%d", branchID, date, slotWaktu)
	secret := []byte(os.Getenv("HMAC_SECRET"))

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))

	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// ValidateQRToken memvalidasi token QR yang dikirim karyawan saat check-in
// Kenapa cek 2 slot?
// Kasus: token generate di menit ke-8 (slot 2)
// Di menit ke-9, slot berganti jadi 3
// Kalau karyawan scan tepat di menit ke-9, token lama (slot 2) masih diterima
// supaya tidak gagal hanya karena beda beberapa detik
//
// Alur:
// 1. Hitung slotNow = menit sekarang / 3
// 2. Hitung slotPrev = (menit sekarang - 1) / 3  ← slot sebelumnya
// 3. Generate tokenNow dari slotNow
// 4. Generate tokenPrev dari slotPrev
// 5. Cocokkan token yang dikirim dengan tokenNow ATAU tokenPrev
func ValidateQRToken(token string, branchID int, date string) bool {
	now := time.Now()

	// Slot Sekarang
	slotNow := now.Minute() / 3

	// Slot sebelumnya — untuk toleransi di detik-detik pergantian slot
	// Contoh: menit=9, slotNow=3, slotPrev=(9-1)/3=2
	// Kalau menit=0, (0-1)=-1, dibagi 3 = -1 → Go hasilkan nilai negatif
	// Tapi ini tidak masalah karena token negatif tidak akan cocok
	slotPrev := (now.Minute() - 1) / 3

	// Generate token untuk kedua slot
	tokenNow := GenerateQRToken(branchID, date, slotNow)
	tokenPrev := GenerateQRToken(branchID, date, slotPrev)

	// pakai hmac.Equal untuk mencegah timing attack
	return hmac.Equal([]byte(token), []byte(tokenNow)) ||
		hmac.Equal([]byte(token), []byte(tokenPrev))
}
