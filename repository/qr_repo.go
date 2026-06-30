package repository

import (
	"absensi_karyawan/models"
	"absensi_karyawan/utils"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"time"
)

type QRRepo struct {
	db *sql.DB
}

func NewQRRepo(db *sql.DB) *QRRepo {
	return &QRRepo{db: db}
}

// generateQRToken - generate token HMAC sama persis dengan Backend 1
// Format payload: "branchID|tanggal|slotWaktu"
// contoh: "1|2026-05-04|2" (slot = menit / 3)
func generateQRToken(branchID int, date string, slotWaktu int) string {
	payload := fmt.Sprintf("%d|%s|%d", branchID, date, slotWaktu)
	secret := []byte(os.Getenv("HMAC_SECRET"))
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// GetTodayQR - generate token QR untuk slot 3 menit sekarang
// Selaras dengan Backend 1 yang pakai slotWaktu = menit / 3
func (r *QRRepo) GetTodayQR(branchID int) (*models.QRData, error) {
	now := utils.NowWITA()
	today := now.Format("2006-01-02")
	slotWaktu := now.Minute() / 3

	token := generateQRToken(branchID, today, slotWaktu)

	// Hitung expires — sampai akhir slot ini (3 menit)
	// Slot berganti setiap kelipatan 3 menit
	// Contoh: menit=8 (slot 2) → expires menit ke-9 (slot 3 mulai)
	minutesUntilNextSlot := 3 - (now.Minute() % 3)
	expiresAt := now.Add(time.Duration(minutesUntilNextSlot) * time.Minute).Truncate(time.Minute)

	// Simpan ke tabel qr_tokens
	// hapus QR WFO lama untuk branch & tanggal yang sama
	_, err := r.db.Exec(`
    DELETE FROM qr_tokens
    WHERE branch_id = $1
      AND date = $2
      AND token_type = 'wfo'
`, branchID, today)

	if err != nil {
    	return nil, err
}

// insert QR WFO baru
		_, err = r.db.Exec(`
    INSERT INTO qr_tokens (
        token,
        branch_id,
        date,
        expires_at,
        token_type
    )
    VALUES (
        $1,
        $2,
        $3::date,
        $4,
        'wfo'
    )
`, token, branchID, today, expiresAt)

if err != nil {
    return nil, err
}

	return &models.QRData{
		Token:     token,
		BranchID:  branchID,
		Date:      today,
		ExpiresAt: expiresAt,
	}, nil
}

// RefreshQR - generate token untuk slot sekarang
func (r *QRRepo) RefreshQR(branchID int) (*models.QRData, error) {
	return r.GetTodayQR(branchID)
}

// GetAllActiveBranchIDs - ambil semua branch_id yang punya admin aktif
func (r *QRRepo) GetAllActiveBranchIDs() ([]int, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT branch_id 
		FROM employees 
		WHERE role IN ('admin_cabang', 'admin') 
		  AND status = 'active'
		  AND branch_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}
