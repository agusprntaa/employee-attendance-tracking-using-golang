package face

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"time"
)

type Service struct {
	Repo      *Repository
	Engine    FaceEngine
	DB        *sql.DB
	Threshold float64 // 0 → pakai DefaultThreshold (0.60)
}

// ─────────────────────────────────────────
// Error variables
// ─────────────────────────────────────────

var (
	ErrAlreadyCheckedIn     = errors.New("ALREADY_CHECKED_IN")
	ErrFaceNotRegistered    = errors.New("FACE_NOT_REGISTERED")
	ErrTokenInvalid         = errors.New("TOKEN_INVALID")
	ErrTokenExpired         = errors.New("TOKEN_EXPIRED")
	ErrTokenUsed            = errors.New("TOKEN_USED")
	ErrTokenNotVerified     = errors.New("TOKEN_NOT_VERIFIED")
	ErrFaceMismatch         = errors.New("FACE_MISMATCH")
	ErrMaxAttemptExceeded   = errors.New("MAX_ATTEMPT_EXCEEDED")
	ErrNoFaceDetected       = errors.New("NO_FACE_DETECTED")
	ErrMultipleFaces        = errors.New("MULTIPLE_FACES")
	ErrEngineError          = errors.New("ENGINE_ERROR")
	ErrOutsideRadius        = errors.New("OUTSIDE_RADIUS")
	ErrAlreadyAttendedEvent = errors.New("ALREADY_ATTENDED_EVENT")
	ErrQRTokenInvalid       = errors.New("QR_TOKEN_INVALID")
	ErrQRNotEventType       = errors.New("QR_NOT_EVENT_TYPE")
	ErrIncompletePoses      = errors.New("INCOMPLETE_POSES")
	ErrPoseInvalid          = errors.New("POSE_INVALID")
	ErrNotEventParticipant  = errors.New("NOT_EVENT_PARTICIPANT")
	ErrEventNotFound        = errors.New("EVENT_NOT_FOUND")
	ErrEventExpired         = errors.New("EVENT_EXPIRED")
)

// ─────────────────────────────────────────
// Helper: haversine
//
// Hitung jarak dua koordinat GPS dalam meter.
// Rumus Haversine digunakan karena akurat untuk jarak pendek (<100km).
// Sama persis dengan implementasi di attendance WFO sebelumnya.
// ─────────────────────────────────────────

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // radius bumi dalam meter
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	dPhi := (lat2 - lat1) * math.Pi / 180
	dLam := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLam/2)*math.Sin(dLam/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// ─────────────────────────────────────────
// Helper: fileHeaderToBase64
//
// Baca bytes dari multipart.FileHeader lalu encode ke base64 string.
// Python service kita terima format: {"image": "<base64 string>"}
// ─────────────────────────────────────────

func fileHeaderToBase64(fh *multipart.FileHeader) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("gagal buka file: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("gagal baca file: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// threshold — return threshold aktif (default jika tidak di-set)
func (s *Service) threshold() float64 {
	if s.Threshold > 0 {
		return s.Threshold
	}
	return DefaultThreshold
}

// ─────────────────────────────────────────
// GetOnboardingStatus
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
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("EMPLOYEE_NOT_FOUND")
	}
	return &res, err
}

// ─────────────────────────────────────────
// RegisterFace — daftarkan wajah karyawan dengan terima 5 foto sekaligus
//
// Input: map[pose]fileHeader, contoh:
//   {"front": file1, "left": file2, "right": file3, "up": file4, "down": file5}
//
// Urutan:
// 1. Validasi kelima pose ada (RequiredPoses) — kalau ada yang kurang, tolak semua
// 2. Loop tiap pose: encode base64 → DetectAndEmbed → harus tepat 1 wajah
//    Kalau salah satu pose gagal detect → batalkan semua, jangan simpan sebagian
// 3. Setelah semua 5 berhasil di-detect, baru SIMPAN kelimanya ke DB
// 4. MarkFaceRegistered — set flag true

func (s *Service) RegisterFace(
	employeeID int,
	files map[string]*multipart.FileHeader,
) (*FaceRegisterResponse, error) {

	// 1. Validasi kelima pose lengkap
	for _, pose := range RequiredPoses {
		if _, ok := files[pose]; !ok {
			return nil, ErrIncompletePoses
		}
	}

	// 2. Detect semua dulu, kumpulkan embedding-nya di memory
	embeddings := make(map[string][]float64)
	for _, pose := range RequiredPoses {
		fh := files[pose]

		imageBase64, err := fileHeaderToBase64(fh)
		if err != nil {
			return nil, err
		}

		embedding, faceCount, err := s.Engine.DetectAndEmbed(imageBase64)
		if err != nil {
			return nil, ErrEngineError
		}
		if faceCount == 0 {
			// ✦ DIUBAH: pakai %w supaya errors.Is(err, ErrNoFaceDetected) di handler tetap match,
			// sambil tetap nyimpen info pose mana yang gagal di pesan errornya.
			return nil, fmt.Errorf("%w: pose %s", ErrNoFaceDetected, pose)
		}
		if faceCount > 1 {
			return nil, fmt.Errorf("%w: pose %s", ErrMultipleFaces, pose)
		}

		embeddings[pose] = embedding
	}

	// 3. Semua berhasil di-detect — baru simpan ke DB
	for _, pose := range RequiredPoses {
		if err := s.Repo.SavePoseEmbedding(employeeID, pose, embeddings[pose]); err != nil {
			return nil, err
		}
	}

	// 4. Set flag face_registered = true
	if err := s.Repo.MarkFaceRegistered(employeeID); err != nil {
		return nil, err
	}

	return &FaceRegisterResponse{
		Message:        "5 foto wajah berhasil didaftarkan",
		RegisteredAt:   time.Now().Format("2006-01-02"),
		FaceRegistered: true,
		PosesSaved:     RequiredPoses,
	}, nil
}

func (s *Service) GetFaceStatus(employeeID int) (*FaceStatusResponse, error) {
	data, err := s.Repo.GetEmployeeFaceData(employeeID)
	if err != nil {
		return nil, err
	}
	if !data.FaceRegistered {
		return &FaceStatusResponse{IsRegistered: false}, nil
	}

	poses, err := s.Repo.GetRegisteredPoses(employeeID)
	if err != nil {
		return nil, err
	}

	var registeredAt string
	_ = s.DB.QueryRow(`
		SELECT COALESCE(TO_CHAR(face_registered_at, 'YYYY-MM-DD'), '')
		FROM employees WHERE id = $1
	`, employeeID).Scan(&registeredAt)

	return &FaceStatusResponse{
		IsRegistered: true,
		RegisteredAt: registeredAt,
		Poses:        poses,
	}, nil
}

// ─────────────────────────────────────────
// GenerateFaceToken
// ─────────────────────────────────────────

func (s *Service) GenerateFaceToken(employeeID int) (*FaceTokenResponse, error) {
	data, err := s.Repo.GetEmployeeFaceData(employeeID)
	if err != nil {
		return nil, err
	}
	if !data.FaceRegistered {
		return nil, ErrFaceNotRegistered
	}

	alreadyIn, err := s.Repo.HasCheckedInWFOToday(employeeID) // ← DIUBAH
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

	return &FaceTokenResponse{FaceToken: token, ExpiresIn: FaceTokenTTL}, nil
}

// ─────────────────────────────────────────
// VerifyFace - bandingkan ke 5 embedding, ambil similarity tertinggi
// ─────────────────────────────────────────
// Urutan:
// 1. Validasi token (sama seperti sebelumnya)
// 2. Ambil 5 embedding tersimpan dari GetAllPoseEmbeddings
//    Kalau kosong (belum register sama sekali) → FACE_NOT_REGISTERED
// 3. Cek max attempts
// 4. Generate embedding baru dari foto live
// 5. Loop bandingkan embedding baru ke KELIMA embedding tersimpan
//    Simpan similarity tertinggi + nama pose-nya
// 6a. Similarity tertinggi < threshold → MISMATCH
// 6b. Similarity tertinggi >= threshold → MATCH, simpan score + pose yang match
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

	// 2. Ambil semua embedding tersimpan (harusnya 5 baris)
	storedEmbeddings, err := s.Repo.GetAllPoseEmbeddings(employeeID)
	if err != nil {
		return nil, err
	}
	if len(storedEmbeddings) == 0 {
		return nil, ErrFaceNotRegistered
	}

	// 3. Cek max attempts
	failures, err := s.Repo.CountDailyFailures(employeeID)
	if err != nil {
		return nil, err
	}
	if failures >= MaxDailyFailedAttempts {
		return nil, ErrMaxAttemptExceeded
	}

	// 4. Generate embedding dari foto live
	imageBase64, err := fileHeaderToBase64(fileHeader)
	if err != nil {
		return nil, err
	}
	newEmbedding, faceCount, err := s.Engine.DetectAndEmbed(imageBase64)
	if err != nil {
		return nil, ErrEngineError
	}
	if faceCount == 0 {
		return nil, ErrNoFaceDetected
	}
	if faceCount > 1 {
		return nil, ErrMultipleFaces
	}

	// 5. Loop bandingkan ke kelima embedding tersimpan — ambil similarity tertinggi
	var bestScore float64
	var bestPose string
	for _, stored := range storedEmbeddings {
		score, err := s.Engine.CompareEmbeddings(newEmbedding, stored.Embedding)
		if err != nil {
			return nil, ErrEngineError
		}
		if score > bestScore {
			bestScore = score
			bestPose = stored.Pose
		}
	}

	// 6a. Mismatch — similarity tertinggi pun masih di bawah threshold
	if bestScore < s.threshold() {
		_ = s.Repo.InsertVerificationLog(nil, employeeID, 0, "mismatch", bestScore, ipAddress)
		return nil, ErrFaceMismatch
	}

	// 6b. Match
	if err := s.Repo.MarkFaceVerified(ft.ID, bestScore); err != nil {
		return nil, err
	}
	_ = s.Repo.InsertVerificationLog(nil, employeeID, 0, "match", bestScore, ipAddress)

	return &VerifyFaceResponse{
		Verified:        true,
		ConfidenceScore: bestScore,
		MatchedPose:     bestPose,
		FaceToken:       faceToken,
	}, nil
}

// ─────────────────────────────────────────
// Checkin
// ─────────────────────────────────────────
func (s *Service) Checkin(
	employeeID int,
	faceToken string,
	latitude, longitude float64,
	ipAddress string,
) (*CheckinResponse, error) {

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
	if !ft.FaceVerified {
		return nil, ErrTokenNotVerified
	}

	empData, err := s.Repo.GetEmployeeFaceData(employeeID)
	if err != nil {
		return nil, err
	}

	branchLat, branchLon, radiusMeter, err := s.Repo.GetBranchLocation(empData.BranchID)
	if err != nil {
		return nil, err
	}
	distance := haversineMeters(latitude, longitude, branchLat, branchLon)
	if distance > float64(radiusMeter) {
		return nil, ErrOutsideRadius
	}

	now := time.Now()
	lateMinutes := 0
	status := "ON_TIME"
	workStart := time.Date(now.Year(), now.Month(), now.Day(), 8, 30, 0, 0, now.Location())
	if now.After(workStart) {
		lateMinutes = int(now.Sub(workStart).Minutes())
		status = "LATE"
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	attendanceID, err := s.Repo.InsertFaceCheckin(
		tx, employeeID, empData.BranchID,
		latitude, longitude, distance,
		lateMinutes, status, ft.ConfidenceScore,
	)
	if err != nil {
		return nil, err
	}

	if err := s.Repo.MarkTokenConsumed(tx, ft.ID); err != nil {
		return nil, err
	}

	if err := s.Repo.InsertVerificationLog(tx, employeeID, attendanceID, "checkin", ft.ConfidenceScore, ipAddress); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &CheckinResponse{
		AttendanceID: attendanceID,
		EmployeeID:   employeeID,
		Date:         now.Format("2006-01-02"),
		CheckinTime:  now.Format("15:04:05"),
		LateMinutes:  lateMinutes,
		Status:       status,
		CheckinType:  "face_geo",
	}, nil
}

// CheckinQREvent — TIDAK BERUBAH
func (s *Service) CheckinQREvent(
	employeeID int,
	req CheckinQRRequest,
	ipAddress string,
) (*CheckinQRResponse, error) {

	// 1. Validasi face_token — harus token bertipe EVENT & sudah verified
	ft, err := s.Repo.GetFaceToken(req.FaceToken, employeeID)
	if err != nil {
		return nil, err
	}
	if ft == nil || ft.EventID == nil {
		return nil, ErrTokenInvalid
	}
	if ft.IsUsed {
		return nil, ErrTokenUsed
	}
	if time.Now().After(ft.ExpiresAt) {
		return nil, ErrTokenExpired
	}
	if !ft.FaceVerified {
		return nil, ErrTokenNotVerified
	}

	// 2. Validasi QR token
	qt, err := s.Repo.GetQRTokenByValue(req.QRToken)
	if err != nil {
		return nil, err
	}
	if qt == nil {
		return nil, ErrQRTokenInvalid
	}
	if qt.TokenType != "event" {
		return nil, ErrQRNotEventType
	}
	if time.Now().After(qt.ExpiresAt) {
		return nil, ErrQRTokenInvalid
	}
	// QR yang discan HARUS untuk event yang sama dengan face_token
	if qt.EventID != *ft.EventID {
		return nil, ErrQRTokenInvalid
	}

	// 3. Validasi geolocation
	event, err := s.Repo.GetEventData(qt.EventID)
	if err != nil {
		return nil, err
	}
	distance := haversineMeters(req.Latitude, req.Longitude, event.Latitude, event.Longitude)
	if distance > float64(event.RadiusMeter) {
		return nil, ErrOutsideRadius
	}

	// 4. Defense-in-depth: cek belum pernah absen (garis akhir tetap unique index DB)
	attended, err := s.Repo.HasAttendedEvent(employeeID, event.ID)
	if err != nil {
		return nil, err
	}
	if attended {
		return nil, ErrAlreadyAttendedEvent
	}

	// 5. Insert + consume token, dalam 1 transaksi
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	attendanceID, err := s.Repo.InsertEventCheckin(
		tx, employeeID, event.ID, event.BranchID,
		req.Latitude, req.Longitude, distance, ft.ConfidenceScore,
	)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.MarkTokenConsumed(tx, ft.ID); err != nil {
		return nil, err
	}
	if err := s.Repo.InsertVerificationLog(tx, employeeID, attendanceID, "checkin_event", ft.ConfidenceScore, ipAddress); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &CheckinQRResponse{
		AttendanceID: attendanceID,
		EventName:    event.Name,
		Date:         time.Now().Format("2006-01-02"),
		CheckinTime:  time.Now().Format("15:04:05"),
	}, nil
}

func (s *Service) GenerateEventFaceToken(employeeID, eventID int) (*FaceTokenResponse, error) {
	// 1. Karyawan harus punya wajah terdaftar (sama seperti WFO)
	data, err := s.Repo.GetEmployeeFaceData(employeeID)
	if err != nil {
		return nil, err
	}
	if !data.FaceRegistered {
		return nil, ErrFaceNotRegistered
	}

	// 2. Whitelist check
	isParticipant, err := s.Repo.IsEventParticipant(employeeID, eventID)
	if err != nil {
		return nil, err
	}
	if !isParticipant {
		return nil, ErrNotEventParticipant
	}

	// 3. Event masih aktif hari ini
	event, err := s.Repo.GetEventData(eventID)
	if err == sql.ErrNoRows {
		return nil, ErrEventNotFound
	}
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if event.Date.Format("2006-01-02") != now.Format("2006-01-02") || now.After(event.ExpiresAt) {
		return nil, ErrEventExpired
	}

	// 4. Belum pernah absen event ini
	attended, err := s.Repo.HasAttendedEvent(employeeID, eventID)
	if err != nil {
		return nil, err
	}
	if attended {
		return nil, ErrAlreadyAttendedEvent
	}

	// 5. Generate token, simpan dengan event_id
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("gagal generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	if err := s.Repo.InsertEventFaceToken(employeeID, eventID, token); err != nil {
		return nil, err
	}

	return &FaceTokenResponse{FaceToken: token, ExpiresIn: FaceTokenTTL}, nil
}

// ─────────────────────────────────────────
// GetActiveEventsToday
// ─────────────────────────────────────────

func (s *Service) GetActiveEventsToday(employeeID int) ([]EventListItem, error) {
	events, err := s.Repo.GetActiveEventsForEmployee(employeeID)
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []EventListItem{}
	}
	return events, nil
}
