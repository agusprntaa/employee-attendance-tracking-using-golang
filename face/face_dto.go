package face

// ─────────────────────────────────────────
// Konstanta
// ─────────────────────────────────────────

const (
	DefaultThreshold       = 0.80 // confidence score minimum
	MaxDailyFailedAttempts = 5    // max percobaan gagal per hari
	FaceTokenTTL           = 120  // detik (2 menit)
)

// ─────────────────────────────────────────
// ONBOARDING
// ─────────────────────────────────────────

// OnboardingStatusResponse — response GET /employee/onboarding-status
//
// FE pakai ini setelah login untuk routing:
//   must_change_password = true  → halaman ubah password
//   face_registered      = false → halaman tambah wajah
//   profile_completed    = false → halaman lengkapi biodata
//   semua sudah selesai          → masuk dashboard
type OnboardingStatusResponse struct {
	MustChangePassword bool `json:"must_change_password"`
	FaceRegistered     bool `json:"face_registered"`
	ProfileCompleted   bool `json:"profile_completed"`
}

// ─────────────────────────────────────────
// FACE TOKEN
// ─────────────────────────────────────────

// FaceTokenResponse — response POST /attendance/face-token
type FaceTokenResponse struct {
	FaceToken string `json:"face_token"` // hex 64 karakter
	ExpiresIn int    `json:"expires_in"` // selalu 120 detik
}

// ─────────────────────────────────────────
// FACE REGISTER
// ─────────────────────────────────────────

// FaceRegisterResponse — response POST /employee/face/register
type FaceRegisterResponse struct {
	Message        string `json:"message"`
	RegisteredAt   string `json:"registered_at"`
	FaceRegistered bool   `json:"face_registered"` // selalu true
}

// FaceStatusResponse — response GET /employee/face/status
type FaceStatusResponse struct {
	IsRegistered bool   `json:"is_registered"`
	RegisteredAt string `json:"registered_at,omitempty"`
}

// ─────────────────────────────────────────
// CHECKIN VERIFY
// ─────────────────────────────────────────

// CheckinVerifyResponse — response POST /attendance/checkin-verify
type CheckinVerifyResponse struct {
	AttendanceID    int     `json:"attendance_id"`
	EmployeeID      int     `json:"employee_id"`
	Date            string  `json:"date"`
	CheckinTime     string  `json:"checkin_time"`
	FaceVerified    bool    `json:"face_verified"`
	ConfidenceScore float64 `json:"confidence_score"`
}
