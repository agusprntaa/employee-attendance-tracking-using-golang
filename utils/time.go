package utils

import "time"

// ServerTime return waktu server sekarang dalam UTC
// SELALU pakai fungsi ini, jangan pakai time.Now() langsung di handler
// Kenapa? Supaya waktu konsisten dari server, tidak bisa dimanipulasi client
//
// Contoh penggunaan:
//   now := utils.ServerTime()
//   → simpan ke DB sebagai UTC
//   → FE konversi ke WIB (UTC+7) untuk tampilan
func ServerTime() time.Time {
	return time.Now().UTC()
}

// TodayDate return tanggal hari ini format "2006-01-02"
// Dipakai untuk query attendance WHERE date = TodayDate()
func TodayDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

// FormatDate format time.Time ke string "2006-01-02"
func FormatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}
