package face

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

type Service struct {
	Repo      *Repository
	Engine    FaceEngine
	DB        *sql.DB
	Threshold float64 // default 0.80 jika 0
}

// ─────────────────────────────────────────
// Error variables — pola sama dengan leave & auth
// ─────────────────────────────────────────

var (
	ErrAlreadyCheckedIn   = errors.New("ALREADY_CHECKED_IN")
	ErrFaceNotRegistered  = errors.New("FACE_NOT_REGISTERED")
	ErrTokenInvalid       = errors.New("TOKEN_INVALID")
	ErrTokenExpired       = errors.New("TOKEN_EXPIRED")
	ErrTokenUsed          = errors.New("TOKEN_USED")
	ErrFaceMismatch       = errors.New("FACE_MISMATCH")
	ErrMaxAttemptExceeded = errors.New("MAX_ATTEMPT_EXCEEDED")
	ErrNoFaceDetected     = errors.New("NO_FACE_DETECTED")
	ErrMultipleFaces      = errors.New("MULTIPLE_FACES")
	ErrEngineError        = errors.New("ENGINE_ERROR")
)

// ─────────────────────────────────────────
// RegisterFace — upload foto referensi wajah
//
// Dipanggil saat onboarding (face_registered = false)
// atau kapan saja karyawan ingin update foto.
//
// Urutan:
// 1. Validasi ukuran file (max 5MB)
// 2. Baca bytes → kirim ke engine
//    - 0 wajah  → NO_FACE_DETECTED
//    - >1 wajah → MULTIPLE_FACES
// 3. Simpan file ke uploads/faces/
// 4. UPDATE employees: face_reference_path + face_registered = true
//
// Tidak ada tabel employee_faces terpisah.
// Satu karyawan = satu referensi aktif = satu kolom di employees.
// ─────────────────────────────────────────

func (s *Service) RegisterFace(
	employeeID int,
	fileHeader *multipart.FileHeader,
) (*FaceRegisterResponse, error) {

	// 1. Validasi ukuran max 5MB
	const maxSize = 5 * 1024 * 1024
	if fileHeader.Size > maxSize {
		return nil, fmt.Errorf("ATTACHMENT_TOO_LARGE")
	}

	// 2. Baca bytes gambar
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("gagal buka file: %w", err)
	}
	defer file.Close()

	imageBytes := make([]byte, fileHeader.Size)
	if _, err := file.Read(imageBytes); err != nil {
		return nil, fmt.Errorf("gagal baca file: %w", err)
	}

	// 3. Validasi jumlah wajah via engine
	faceCount, err := s.Engine.DetectFace(imageBytes)
	if err != nil {
		return nil, ErrEngineError
	}
	if faceCount == 0 {
		return nil, ErrNoFaceDetected
	}
	if faceCount > 1 {
		return nil, ErrMultipleFaces
	}

	// 4. Simpan file ke disk
	// Path: uploads/faces/{employeeID}_{unix_timestamp}.jpg
	// Konsisten dengan pola attachment di leave module
	uploadDir := "uploads/faces"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("gagal buat folder: %w", err)
	}

	filename := fmt.Sprintf("%d_%d.jpg", employeeID, time.Now().Unix())
	savePath := filepath.Join(uploadDir, filename)

	if err := os.WriteFile(savePath, imageBytes, 0644); err != nil {
		return nil, fmt.Errorf("gagal simpan file: %w", err)
	}

	// 5. UPDATE employees — simpan path dan tandai face_registered = true
	if err := s.Repo.UpdateFaceReference(employeeID, savePath); err != nil {
		_ = os.Remove(savePath) // hapus file jika DB gagal
		return nil, err
	}

	return &FaceRegisterResponse{
		Message:        "Foto wajah berhasil didaftarkan",
		RegisteredAt:   time.Now().Format("2006-01-02"),
		FaceRegistered: true,
	}, nil
}

// GetFaceStatus — cek status pendaftaran wajah karyawan
// FE pakai ini untuk tampilkan banner di halaman face register
func (s *Service) GetFaceStatus(employeeID int) (*FaceStatusResponse, error) {
	_, registered, err := s.Repo.GetFaceReference(employeeID)
	if err != nil {
		return nil, err
	}
	if !registered {
		return &FaceStatusResponse{IsRegistered: false}, nil
	}

	// Ambil registered_at dari DB untuk info tambahan
	var registeredAt string
	_ = s.Repo.DB.QueryRow(`
		SELECT COALESCE(TO_CHAR(face_registered_at, 'YYYY-MM-DD'), '')
		FROM employees WHERE id = $1
	`, employeeID).Scan(&registeredAt)

	return &FaceStatusResponse{
		IsRegistered: true,
		RegisteredAt: registeredAt,
	}, nil
}

// ─────────────────────────────────────────
// GenerateFaceToken — generate token sebelum kamera dibuka
//
// Dipanggil FE saat karyawan tap CHECK IN.
// Kamera TIDAK boleh dibuka sebelum token ini ada.
//
// Urutan validasi:
// 1. face_registered = true? → kalau belum → FACE_NOT_REGISTERED
//    (cek ini dulu karena lebih murah dari cek checkin)
// 2. Sudah checkin hari ini? → ALREADY_CHECKED_IN
// 3. Generate token crypto/rand 32 byte (hex = 64 char)
// 4. Simpan ke face_tokens TTL 2 menit
//
// Kenapa must_change_password TIDAK dicek di sini?
// Sudah ditangani middleware MustChangePassword dari auth package.
// Semua route yang butuh password sudah diganti tidak bisa diakses
// kecuali lewat /change-password dulu.
// ─────────────────────────────────────────

func (s *Service) GenerateFaceToken(employeeID int) (*FaceTokenResponse, error) {
	// 1. Cek face sudah terdaftar
	_, registered, err := s.Repo.GetFaceReference(employeeID)
	if err != nil {
		return nil, err
	}
	if !registered {
		return nil, ErrFaceNotRegistered
	}

	// 2. Cek sudah checkin hari ini
	alreadyIn, err := s.Repo.HasCheckedInToday(employeeID)
	if err != nil {
		return nil, err
	}
	if alreadyIn {
		return nil, ErrAlreadyCheckedIn
	}

	// 3. Generate token aman dengan crypto/rand
	// Jangan pakai math/rand — tidak aman untuk token autentikasi
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("gagal generate token: %w", err)
	}
	token := hex.EncodeToString(b) // 64 karakter hex

	// 4. Simpan ke DB (trigger DB auto-cleanup token expired lama)
	if err := s.Repo.InsertFaceToken(employeeID, token); err != nil {
		return nil, err
	}

	return &FaceTokenResponse{
		FaceToken: token,
		ExpiresIn: FaceTokenTTL, // 120 detik
	}, nil
}

// ─────────────────────────────────────────
// VerifyAndCheckin — verifikasi wajah + simpan attendance
//
// Ini fungsi paling kritis di seluruh face module.
// Urutan TIDAK BOLEH diubah:
//
// 1. Validasi token
//    - Tidak ada di DB        → TOKEN_INVALID
//    - is_used = true         → TOKEN_USED
//    - Sudah expired          → TOKEN_EXPIRED
//
// 2. Load foto referensi dari employees.face_reference_path
//    - Kosong / NULL          → FACE_NOT_REGISTERED
//
// 3. Cek percobaan gagal hari ini (SEBELUM proses gambar)
//    - Sudah ≥ 5              → MAX_ATTEMPT_EXCEEDED
//
// 4. Baca bytes gambar dari file upload
//
// 5. Panggil face engine → confidence score
//    - Engine error           → ENGINE_ERROR (JANGAN log sebagai mismatch)
//
// 6a. score < threshold:
//    - Log mismatch (tanpa tx)
//    - Return FACE_MISMATCH
//    - Token TETAP valid sampai expired (karyawan bisa coba lagi)
//
// 6b. score >= threshold:
//    BEGIN TRANSACTION
//    - INSERT attendances (face_verified=true, confidence_score)
//    - UPDATE face_tokens SET is_used=true
//    - INSERT face_verification_logs (result=match)
//    COMMIT
//    (jika salah satu gagal → ROLLBACK semua)
// ─────────────────────────────────────────

func (s *Service) VerifyAndCheckin(
	employeeID int,
	faceToken string,
	fileHeader *multipart.FileHeader,
	ipAddress string,
) (*CheckinVerifyResponse, error) {

	// 1. Validasi token — urutan cek penting
	ft, err := s.Repo.GetFaceToken(faceToken, employeeID)
	if err != nil {
		return nil, err
	}
	if ft == nil {
		// Token tidak ada di DB atau bukan milik employee ini
		return nil, ErrTokenInvalid
	}
	if ft.IsUsed {
		return nil, ErrTokenUsed
	}
	if time.Now().After(ft.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// 2. Load foto referensi dari kolom employees
	referencePath, registered, err := s.Repo.GetFaceReference(employeeID)
	if err != nil {
		return nil, err
	}
	if !registered || referencePath == "" {
		return nil, ErrFaceNotRegistered
	}

	// 3. Cek percobaan gagal hari ini SEBELUM proses gambar
	// Jangan buang resource engine jika sudah pasti ditolak
	failures, err := s.Repo.CountDailyFailures(employeeID)
	if err != nil {
		return nil, err
	}
	if failures >= MaxDailyFailedAttempts {
		return nil, ErrMaxAttemptExceeded
	}

	// 4. Baca bytes gambar dari file upload FE
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("gagal buka file: %w", err)
	}
	defer file.Close()

	imageBytes := make([]byte, fileHeader.Size)
	if _, err := file.Read(imageBytes); err != nil {
		return nil, fmt.Errorf("gagal baca file: %w", err)
	}

	// 5. Panggil face engine
	threshold := s.Threshold
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	score, err := s.Engine.Compare(imageBytes, referencePath)
	if err != nil {
		// Engine error BUKAN kesalahan karyawan — jangan log sebagai mismatch
		return nil, ErrEngineError
	}

	// 6a. MISMATCH
	if score < threshold {
		// Log gagal tanpa transaksi — tidak ada attendance yang dibuat
		_ = s.Repo.InsertVerificationLog(nil, employeeID, 0, "mismatch", score, ipAddress)
		return nil, ErrFaceMismatch
	}

	// 6b. MATCH — semua operasi dalam satu transaksi
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("gagal mulai transaksi: %w", err)
	}

	var txErr error
	defer func() {
		if txErr != nil {
			tx.Rollback()
		}
	}()

	// INSERT attendance
	attendanceID, txErr := s.Repo.InsertAttendanceWithFace(tx, employeeID, score)
	if txErr != nil {
		return nil, fmt.Errorf("gagal simpan attendance: %w", txErr)
	}

	// Invalidate token — tidak bisa dipakai lagi
	txErr = s.Repo.MarkTokenUsed(tx, ft.ID)
	if txErr != nil {
		return nil, fmt.Errorf("gagal invalidate token: %w", txErr)
	}

	// Log sukses dalam transaksi yang sama
	txErr = s.Repo.InsertVerificationLog(tx, employeeID, attendanceID, "match", score, ipAddress)
	if txErr != nil {
		return nil, fmt.Errorf("gagal simpan log: %w", txErr)
	}

	// Commit semua
	txErr = tx.Commit()
	if txErr != nil {
		return nil, fmt.Errorf("gagal commit: %w", txErr)
	}

	return &CheckinVerifyResponse{
		AttendanceID:    attendanceID,
		EmployeeID:      employeeID,
		Date:            time.Now().Format("2006-01-02"),
		CheckinTime:     time.Now().Format("15:04:05"),
		FaceVerified:    true,
		ConfidenceScore: score,
	}, nil
}

func (s *Service) GetOnboardingStatus(
	employeeID int,
) (*OnboardingStatusResponse, error) {

	var res OnboardingStatusResponse

	err := s.DB.QueryRow(`
		SELECT
			must_change_password,
			face_registered,
			profile_completed
		FROM employees
		WHERE id = $1
	`, employeeID).Scan(
		&res.MustChangePassword,
		&res.FaceRegistered,
		&res.ProfileCompleted,
	)

	if err != nil {

		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("EMPLOYEE_NOT_FOUND")
		}

		return nil, err
	}

	return &res, nil
}
