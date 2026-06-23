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
	Threshold float64
}

var (
	ErrAlreadyCheckedIn   = errors.New("ALREADY_CHECKED_IN")
	ErrFaceNotRegistered  = errors.New("FACE_NOT_REGISTERED")
	ErrFaceNotVerified    = errors.New("FACE_NOT_VERIFIED") // ✦ BARU — token belum di-verify-face
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
// RegisterFace — upload foto referensi wajah (tidak berubah)
// ─────────────────────────────────────────

func (s *Service) RegisterFace(
	employeeID int,
	fileHeader *multipart.FileHeader,
) (*FaceRegisterResponse, error) {
	const maxSize = 5 * 1024 * 1024
	if fileHeader.Size > maxSize {
		return nil, fmt.Errorf("ATTACHMENT_TOO_LARGE")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("gagal buka file: %w", err)
	}
	defer file.Close()

	imageBytes := make([]byte, fileHeader.Size)
	if _, err := file.Read(imageBytes); err != nil {
		return nil, fmt.Errorf("gagal baca file: %w", err)
	}

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

	uploadDir := "uploads/faces"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("gagal buat folder: %w", err)
	}

	filename := fmt.Sprintf("%d_%d.jpg", employeeID, time.Now().Unix())
	savePath := filepath.Join(uploadDir, filename)

	if err := os.WriteFile(savePath, imageBytes, 0644); err != nil {
		return nil, fmt.Errorf("gagal simpan file: %w", err)
	}

	if err := s.Repo.UpdateFaceReference(employeeID, savePath); err != nil {
		_ = os.Remove(savePath)
		return nil, err
	}

	return &FaceRegisterResponse{
		Message:        "Foto wajah berhasil didaftarkan",
		RegisteredAt:   time.Now().Format("2006-01-02"),
		FaceRegistered: true,
	}, nil
}

// GetFaceStatus — tidak berubah
func (s *Service) GetFaceStatus(employeeID int) (*FaceStatusResponse, error) {
	_, registered, err := s.Repo.GetFaceReference(employeeID)
	if err != nil {
		return nil, err
	}
	if !registered {
		return &FaceStatusResponse{IsRegistered: false}, nil
	}

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
// GenerateFaceToken — Step 1
// Generate token, face_verified=false
// Tidak berubah dari sebelumnya
// ─────────────────────────────────────────

func (s *Service) GenerateFaceToken(employeeID int) (*FaceTokenResponse, error) {
	_, registered, err := s.Repo.GetFaceReference(employeeID)
	if err != nil {
		return nil, err
	}
	if !registered {
		return nil, ErrFaceNotRegistered
	}

	alreadyIn, err := s.Repo.HasCheckedInToday(employeeID)
	if err != nil {
		return nil, err
	}
	if alreadyIn {
		return nil, ErrAlreadyCheckedIn
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("gagal generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	if err := s.Repo.InsertFaceToken(employeeID, token); err != nil {
		return nil, err
	}

	return &FaceTokenResponse{
		FaceToken: token,
		ExpiresIn: FaceTokenTTL,
	}, nil
}

// ─────────────────────────────────────────
// VerifyFace — Step 2
//
// Verifikasi wajah saja, BELUM checkin.
// Jika cocok → UPDATE face_tokens SET face_verified=true
// Token is_used tetap false → masih bisa dipakai untuk checkin
//
// Urutan:
// 1. Validasi token (ada? milik user? belum expired? belum dipakai?)
// 2. Load foto referensi
// 3. Cek MAX_ATTEMPT
// 4. Compare wajah
// 5. Jika match → MarkFaceVerified (face_verified=true, is_used tetap false)
// ─────────────────────────────────────────

func (s *Service) VerifyFace(
	employeeID int,
	faceToken string,
	fileHeader *multipart.FileHeader,
	ipAddress string,
) (*VerifyFaceResponse, error) {

	// 1. Validasi token
	ft, err := s.Repo.GetFaceToken(faceToken, employeeID)
	if err != nil {
		return nil, err
	}
	if ft == nil {
		return nil, ErrTokenInvalid
	}
	if ft.IsUsed {
		return nil, ErrTokenUsed
	}
	if time.Now().After(ft.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// 2. Load foto referensi
	referencePath, registered, err := s.Repo.GetFaceReference(employeeID)
	if err != nil {
		return nil, err
	}
	if !registered || referencePath == "" {
		return nil, ErrFaceNotRegistered
	}

	// 3. Cek MAX_ATTEMPT sebelum proses gambar
	failures, err := s.Repo.CountDailyFailures(employeeID)
	if err != nil {
		return nil, err
	}
	if failures >= MaxDailyFailedAttempts {
		return nil, ErrMaxAttemptExceeded
	}

	// 4. Baca bytes gambar
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("gagal buka file: %w", err)
	}
	defer file.Close()

	imageBytes := make([]byte, fileHeader.Size)
	if _, err := file.Read(imageBytes); err != nil {
		return nil, fmt.Errorf("gagal baca file: %w", err)
	}

	// 5. Compare wajah
	threshold := s.Threshold
	if threshold == 0 {
		threshold = DefaultThreshold
	}

	score, err := s.Engine.Compare(imageBytes, referencePath)
	if err != nil {
		return nil, ErrEngineError
	}

	// MISMATCH — log dan return error
	if score < threshold {
		_ = s.Repo.InsertVerificationLog(nil, employeeID, 0, "mismatch", score, ipAddress)
		return nil, ErrFaceMismatch
	}

	// MATCH — update token face_verified=true, is_used tetap false
	// is_used baru jadi true setelah /attendance/checkin dipanggil
	if err := s.Repo.MarkFaceVerified(ft.ID, score); err != nil {
		return nil, fmt.Errorf("gagal update face verified: %w", err)
	}

	// Log match (tanpa attendance_id — belum checkin)
	_ = s.Repo.InsertVerificationLog(nil, employeeID, 0, "match", score, ipAddress)

	return &VerifyFaceResponse{
		FaceToken:       faceToken,
		ConfidenceScore: score,
		Verified:        true,
	}, nil
}

// ─────────────────────────────────────────
// CheckinWithFaceToken — Step 3
//
// Dipanggil dari attendance handler yang sudah ada.
// Tugasnya hanya validasi face_token sudah face_verified=true
// lalu mark is_used=true setelah checkin berhasil.
//
// CARA INTEGRASI:
// Di attendance/service.go tambahkan call ke FaceRepo.IsFaceVerified
// sebelum proses checkin. Atau tambahkan fungsi ini dan panggil
// dari attendance handler.
// ─────────────────────────────────────────

// ValidateFaceTokenForCheckin — dipanggil attendance service
// Cek: token ada? face_verified=true? belum expired? belum used?
// Return: face token ID untuk di-consume setelah checkin berhasil
func (s *Service) ValidateFaceTokenForCheckin(employeeID int, faceToken string) (int, float64, error) {
	ft, err := s.Repo.GetFaceToken(faceToken, employeeID)
	if err != nil {
		return 0, 0, err
	}
	if ft == nil {
		return 0, 0, ErrTokenInvalid
	}
	if ft.IsUsed {
		return 0, 0, ErrTokenUsed
	}
	if time.Now().After(ft.ExpiresAt) {
		return 0, 0, ErrTokenExpired
	}
	// ✦ Cek utama: face harus sudah diverifikasi
	if !ft.FaceVerified {
		return 0, 0, ErrFaceNotVerified
	}
	return ft.ID, ft.ConfidenceScore, nil
}

// ConsumeToken — mark is_used=true setelah attendance berhasil dibuat
func (s *Service) ConsumeToken(faceToken string) error {
	return s.Repo.ConsumeFaceToken(faceToken)
}

// ─────────────────────────────────────────
// GetOnboardingStatus — tidak berubah
// ─────────────────────────────────────────

func (s *Service) GetOnboardingStatus(employeeID int) (*OnboardingStatusResponse, error) {
	var res OnboardingStatusResponse
	err := s.DB.QueryRow(`
		SELECT must_change_password, face_registered, profile_completed
		FROM employees WHERE id = $1
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
