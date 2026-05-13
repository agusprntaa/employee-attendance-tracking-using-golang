package utils

import "time"

// ServerTime — waktu server sekarang
// PENTING: pakai Local() bukan UTC() supaya konsisten dengan BE2
// BE2 pakai time.Now() tanpa UTC — kita ikuti supaya slot waktu sama
func ServerTime() time.Time {
	return time.Now()
}

// TodayDate — tanggal hari ini format "2006-01-02"
// Harus konsisten dengan BE2 yang pakai time.Now().Format(...)
// Kalau BE2 pakai local time, kita juga harus local time
func TodayDate() string {
	return time.Now().Format("2006-01-02")
}

func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}
