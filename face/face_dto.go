package face

// ─────────────────────────────────────────
// Konstanta
// ─────────────────────────────────────────

const (
	// threshold diturunkan dari 0.80 ke 0.60
	// InsightFace buffalo_l menggunakan cosine similarity (bukan persentase seperti AWS).
	// Nilai 0.60 adalah batas aman untuk kondisi lighting/sudut berbeda.
	// AWS Rekognition pakai skala 0-100, InsightFace pakai 0.0-1.0 — beda skala!
	DefaultThreshold       = 0.60
	MaxDailyFailedAttempts = 10
	FaceTokenTTL           = 120 // detik
)

// Daftar pose yang wajib dikirim saat registrasi — urutan ini dipakai validasi
var RequiredPoses = []string{"front", "left", "right", "up", "down"}

// ─────────────────────────────────────────
// ONBOARDING
// ─────────────────────────────────────────

type OnboardingStatusResponse struct {
	MustChangePassword bool `json:"must_change_password"`
	FaceRegistered     bool `json:"face_registered"`
	ProfileCompleted   bool `json:"profile_completed"`
}

// ─────────────────────────────────────────
// FACE TOKEN
// ─────────────────────────────────────────

type FaceTokenResponse struct {
	FaceToken string `json:"face_token"`
	ExpiresIn int    `json:"expires_in"`
}

// ─────────────────────────────────────────
// FACE REGISTER
// ─────────────────────────────────────────

type FaceRegisterResponse struct {
	Message        string   `json:"message"`
	RegisteredAt   string   `json:"registered_at"`
	FaceRegistered bool     `json:"face_registered"`
	PosesSaved     []string `json:"poses_saved"` // ["front","left","right","up","down"]
}

type FaceStatusResponse struct {
	IsRegistered bool     `json:"is_registered"`
	RegisteredAt string   `json:"registered_at,omitempty"`
	Poses        []string `json:"poses"` // pose yang sudah tersimpan
}

// ─────────────────────────────────────────
// VERIFY FACE (step terpisah sebelum checkin)
// ─────────────────────────────────────────

type VerifyFaceResponse struct {
	Verified        bool    `json:"verified"`
	ConfidenceScore float64 `json:"confidence_score"`
	MatchedPose     string  `json:"matched_pose"` // pose dengan similarity tertinggi
	FaceToken       string  `json:"face_token"`   // token yang sama, dikembalikan untuk step checkin
}

// ─────────────────────────────────────────
// CHECKIN (step final setelah verify)
// ─────────────────────────────────────────

// CheckinResponse — response POST /attendance/checkin
// checkin sekarang endpoint terpisah, terima face_token yang sudah verified
type CheckinResponse struct {
	AttendanceID int    `json:"attendance_id"`
	EmployeeID   int    `json:"employee_id"`
	Date         string `json:"date"`
	CheckinTime  string `json:"check_in"`
	LateMinutes  int    `json:"late_minutes"`
	Status       string `json:"status"`
	CheckinType  string `json:"checkin_type"`
}

// ─────────────────────────────────────────
// CHECKIN QR EVENT
// ─────────────────────────────────────────

// CheckinQRRequest — body POST /attendance/checkin-qr
// endpoint khusus absen QR event luar kantor
type CheckinQRRequest struct {
	FaceToken string  `json:"face_token"`
	QRToken   string  `json:"qr_token"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// CheckinQRResponse — response POST /attendance/checkin-qr
type CheckinQRResponse struct {
	AttendanceID int    `json:"attendance_id"`
	EventName    string `json:"event_name"`
	Date         string `json:"date"`
	CheckinTime  string `json:"check_in"`
}

// ─────────────────────────────────────────
// EVENT FACE TOKEN
// ─────────────────────────────────────────

// EventFaceTokenRequest — body POST /attendance/event/face-token
type EventFaceTokenRequest struct {
	EventID int `json:"event_id"`
}

// EventListItem — item di GET /attendance/events/active-today
type EventListItem struct {
	EventID          int    `json:"event_id"`
	Name             string `json:"name"`
	Location         string `json:"location"`
	Date             string `json:"date"`
	StartTime        string `json:"start_time,omitempty"`
	EndTime          string `json:"end_time,omitempty"`
	AlreadyCheckedIn bool   `json:"already_checked_in"`
}
