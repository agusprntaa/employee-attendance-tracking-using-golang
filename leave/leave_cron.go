package leave

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// StartCronJob — jalankan cron job reset kuota cuti
// Dipanggil dari main.go saat server start
//
// Jadwal: tiap 1 Januari jam 00:01 WITA (UTC+8)
// Cron expression: "1 0 1 1 *"
//
// Cara pakai di main.go:
//   leaveRepo := &leave.Repository{DB: db}
//   leave.StartCronJob(leaveRepo)

func StartCronJob(repo *Repository) {
	c := cron.New(cron.WithLocation(witaLocation()))

	// Tiap 1 Januari jam 00:01
	c.AddFunc("1 0 1 1 *", func() {
		log.Println("[CRON] Memulai reset kuota cuti tahunan...")

		if err := repo.ResetQuotasForNewYear(); err != nil {
			log.Printf("[CRON] ERROR reset kuota: %v", err)
			return
		}

		log.Println("[CRON] Reset kuota cuti selesai")
	})

	c.Start()
	log.Println("[CRON] Cron job kuota cuti aktif — jadwal: tiap 1 Januari 00:01 WITA")
}

func witaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Makassar")
	if err != nil {
		return time.FixedZone("WITA", 8*60*60)
	}
	return loc
}
