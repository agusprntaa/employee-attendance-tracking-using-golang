package attendance

import (
	"time"
)

// timeOfDay gabungkan tanggal dari `base` dengan jam dari `t`
// Dipakai untuk bangun work_start dan work_end hari ini
//
// Contoh:
//
//	base = 2026-04-29 09:15:00 UTC  (waktu sekarang)
//	t    = 0000-01-01 08:00:00 UTC  (work_start dari DB)
//	hasil = 2026-04-29 08:00:00 UTC
func timeOfDay(base time.Time, t time.Time) time.Time {
	return time.Date(
		base.Year(), base.Month(), base.Day(),
		t.Hour(), t.Minute(), 0, 0, time.UTC,
	)
}

// minutesDuration konversi int menit ke time.Duration
func minutesDuration(minutes int) time.Duration {
	return time.Duration(minutes) * time.Minute
}
