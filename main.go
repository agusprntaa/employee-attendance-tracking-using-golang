package main

import (
	"absensi/config"
	"absensi/database"
	"absensi/repository"
	"absensi/router"
	"log"
	"time"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// ─── Background Goroutine: Refresh QR setiap 3 menit ─────────────────────
	qrRepo := repository.NewQRRepo(db)
	go func() {
		refreshAllQR(qrRepo)
		ticker := time.NewTicker(3 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			refreshAllQR(qrRepo)
		}
	}()

	// ─── Fiber App ────────────────────────────────────────────────────────────
	app := router.SetupRouter(db, cfg)
	log.Printf("Backend 2 (Admin Cabang) running on port %s", cfg.Port)
	log.Fatal(app.Listen(":" + cfg.Port))
}

func refreshAllQR(qrRepo *repository.QRRepo) {
	branchIDs, err := qrRepo.GetAllActiveBranchIDs()
	if err != nil {
		log.Printf("[QR] Gagal ambil branch IDs: %v", err)
		return
	}
	for _, branchID := range branchIDs {
		qr, err := qrRepo.GetTodayQR(branchID)
		if err != nil {
			log.Printf("[QR] Gagal refresh QR cabang %d: %v", branchID, err)
			continue
		}
		log.Printf("[QR] Cabang %d → slot %d, expires: %s",
			branchID,
			time.Now().Minute()/3,
			qr.ExpiresAt.Format("15:04:05"),
		)
	}
}
