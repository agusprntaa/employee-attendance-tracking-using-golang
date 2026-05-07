package handlers

import (
	"absensi/middleware"
	"absensi/models"
	"absensi/repository"
	"absensi/utils"
	"encoding/json"
	"net/http"
	"time"
)

type QRHandler struct {
	qrRepo *repository.QRRepo
}

func NewQRHandler(qr *repository.QRRepo) *QRHandler {
	return &QRHandler{qrRepo: qr}
}

// GET /admin-cabang/qr/today
// Ambil QR aktif hari ini
// Frontend polling endpoint ini setiap 3 menit untuk dapat token terbaru
// lalu render qr_content jadi gambar QR
func (h *QRHandler) GetTodayQR(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	qr, err := h.qrRepo.GetTodayQR(*claims.BranchID)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil QR Code")
		return
	}

	// Kalau belum ada QR (pertama kali), generate sekarang
	if qr == nil {
		qr, err = h.qrRepo.RefreshQR(*claims.BranchID)
		if err != nil {
			utils.InternalError(w, "Gagal membuat QR Code")
			return
		}
	}

	utils.Success(w, buildQRResponse(qr))
}

// POST /admin-cabang/qr/regenerate
// Force generate token QR baru sekarang
// Dipakai jika admin mau refresh manual
func (h *QRHandler) RegenerateQR(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	qr, err := h.qrRepo.RefreshQR(*claims.BranchID)
	if err != nil {
		utils.InternalError(w, "Gagal regenerate QR Code")
		return
	}

	utils.Success(w, buildQRResponse(qr))
}

// buildQRResponse - helper buat response QR lengkap
func buildQRResponse(qr *models.QRData) map[string]interface{} {
	// qr_content = JSON string yang di-render jadi gambar QR oleh Frontend
	// Backend 1 akan parse string ini saat karyawan scan
	qrContentMap := map[string]interface{}{
		"token":     qr.Token,
		"branch_id": qr.BranchID,
		"date":      qr.Date,
	}
	qrContentBytes, _ := json.Marshal(qrContentMap)

	// Hitung sisa detik sampai expired
	refreshIn := int(time.Until(qr.ExpiresAt).Seconds())
	if refreshIn < 0 {
		refreshIn = 0
	}

	return map[string]interface{}{
		"token":      qr.Token,
		"branch_id":  qr.BranchID,
		"date":       qr.Date,
		"expires_at": qr.ExpiresAt,
		"expires":    "Berlaku 15 menit",
		"refresh_in": refreshIn, // Frontend pakai ini untuk countdown timer
		"qr_content": string(qrContentBytes), // Frontend render ini jadi gambar QR
	}
}
