package attendance

import "time"

// ────────────────────────────────────────────
// REQUEST structs (dari frontend ke backend)
// ────────────────────────────────────────────

// CheckInRequest dipakai untuk KEDUA mode WFO dan WFA
// WorkType wajib diisi: "WFO" atau "WFA"
type CheckInRequest struct {
	WorkType  string  `json:"work_type"`  // "WFO" atau "WFA"
	Lat       float64 `json:"lat"`        // GPS latitude karyawan (WFO wajib)
	Lon       float64 `json:"lon"`        // GPS longitude karyawan (WFO wajib)
	Accuracy  float64 `json:"accuracy"`   // akurasi GPS dalam meter (WFO wajib)
	QRToken   string  `json:"qr_token"`   // token dari scan QR (WFO wajib)
	BranchID  int     `json:"branch_id"`  // ID cabang dari QR (WFO wajib)
	WFAReason string  `json:"wfa_reason"` // alasan WFA minimal 20 karakter (WFA wajib)
}

// CheckOutRequest dipakai saat karyawan checkout
// EarlyLeaveReason wajib diisi jika pulang sebelum work_end
type CheckOutRequest struct {
	EarlyLeaveReason string `json:"early_leave_reason"`
}

// ChangePasswordRequest untuk ubah password karyawan
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ────────────────────────────────────────────
// RESPONSE structs KARYAWAN (dari backend ke frontend)
// ────────────────────────────────────────────

// AttendanceResponse data absensi satu record
type AttendanceResponse struct {
	ID               int        `json:"id"`
	Date             string     `json:"date"`
	WorkType         string     `json:"work_type"`
	Status           string     `json:"status"`
	CheckIn          *time.Time `json:"check_in"`
	CheckOut         *time.Time `json:"check_out"`
	LateMinutes      int        `json:"late_minutes"`
	IsAutoCheckout   bool       `json:"is_auto_checkout"`
	WFAReason        string     `json:"wfa_reason,omitempty"`
	EarlyLeaveReason string     `json:"early_leave_reason,omitempty"`
	DistanceMeter    float64    `json:"distance_meter,omitempty"`
}

// TodayResponse dipakai untuk GET /attendance/today
// Frontend butuh ini untuk tampilan dashboard dan status tombol check-in/out
type TodayResponse struct {
	HasCheckedIn  bool                `json:"has_checked_in"`
	HasCheckedOut bool                `json:"has_checked_out"`
	Attendance    *AttendanceResponse `json:"attendance"` // null jika belum check-in
}

// ProfileResponse untuk GET /employee/profile
type ProfileResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	Tipe         string `json:"tipe"`
	DivisionName string `json:"division_name"`
	BranchName   string `json:"branch_name"`
}

type HistoryResponse struct {
	Data       []*AttendanceResponse `json:"data"`
	Total      int                   `json:"total"`
	Page       int                   `json:"page"`
	Limit      int                   `json:"limit"`
	TotalPages int                   `json:"total_pages"`
}
