package utils

import "time"

// WITA timezone Asia/Makassar (UTC+8)
var WITA *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Makassar")
	if err != nil {
		panic(err)
	}
	WITA = loc
}

// NowWITA return waktu sekarang dalam WITA.
//
// PENTING — kolom check_in/check_out di DB adalah TIMESTAMP WITHOUT TIME ZONE.
// lib/pq untuk kolom jenis ini TIDAK melakukan konversi timezone — ia simpan
// nilai numerik apa adanya yang ada di time.Time.
//
// Jadi jika kita kirim 12:37 WITA → DB simpan 12:37 (tanpa offset).
// Saat dibaca kembali → lib/pq return 12:37 UTC → .In(WITA) = 20:37 ❌
//
// Solusi: kirim UTC ke DB, baca kembali sebagai UTC, konversi ke WITA hanya
// di layer response (recordToResponse).
func NowWITA() time.Time {
	return time.Now()
}

// TodayDate return tanggal hari ini format YYYY-MM-DD dalam WITA.
// Tetap pakai WITA agar tanggal absensi sesuai hari kerja lokal.
func TodayDate() string {
	return time.Now().In(WITA).Format("2006-01-02")
}
