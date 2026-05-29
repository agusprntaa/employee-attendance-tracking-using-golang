package leave

// ─────────────────────────────────────────
// Jenis cuti yang tersedia
// Hanya Cuti Tahunan yang memotong kuota
// ─────────────────────────────────────────

var ValidLeaveTypes = map[string]bool{
	"Cuti Tahunan":    true,
	"Cuti Sakit":      true,
	"Izin Pribadi":    true,
	"Cuti Melahirkan": true,
}

// LeaveTypeDeductsQuota — cek apakah jenis cuti memotong kuota
// Hanya Cuti Tahunan yang memotong kuota dari 12 jatah per tahun
func LeaveTypeDeductsQuota(leaveType string) bool {
	return leaveType == "Cuti Tahunan"
}

// ─────────────────────────────────────────
// REQUEST DTO
// ─────────────────────────────────────────

// LeaveRequestDTO — karyawan ajukan cuti
// Dikirim sebagai multipart/form-data karena ada attachment
type LeaveRequestDTO struct {
	LeaveType string `form:"leave_type"` // wajib: salah satu dari ValidLeaveTypes
	StartDate string `form:"start_date"` // format: "YYYY-MM-DD"
	EndDate   string `form:"end_date"`   // format: "YYYY-MM-DD"
	Reason    string `form:"reason"`     // minimal 10 karakter
	// Attachment diambil dari c.FormFile("attachment") di handler — tidak ada di sini
}

// ─────────────────────────────────────────
// RESPONSE DTO
// ─────────────────────────────────────────

// QuotaResponse — response GET /employee/leave/quota
type QuotaResponse struct {
	Year      int  `json:"year"`
	Total     int  `json:"total"`
	Used      int  `json:"used"`
	Remaining int  `json:"remaining"` // dihitung: total - used
	CanApply  bool `json:"can_apply"` // true jika remaining > 0 dan sudah 1 tahun kerja
}

// LeaveItem — satu baris riwayat cuti
type LeaveItem struct {
	ID             int    `json:"id"`
	LeaveType      string `json:"leave_type"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	TotalDays      int    `json:"total_days"`
	Reason         string `json:"reason"`
	Status         string `json:"status"` // pending / approved / rejected / cancelled
	Note           string `json:"note,omitempty"`
	AttachmentPath string `json:"attachment_path,omitempty"` // path file attachment
	CreatedAt      string `json:"created_at"`
}

// LeaveHistoryResponse — response GET /employee/leave/history
type LeaveHistoryResponse struct {
	Data       []LeaveItem `json:"data"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	TotalPages int         `json:"total_pages"`
}

// HolidayItem — satu baris hari libur
type HolidayItem struct {
	ID   int    `json:"id"`
	Date string `json:"date"`
	Name string `json:"name"`
}

// ─────────────────────────────────────────
// NOTIFICATION DTO
// ─────────────────────────────────────────

type NotificationItem struct {
	ID          int    `json:"id"`
	LeaveType   string `json:"leave_type"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Status      string `json:"status"`         // pending / approved / rejected
	Title       string `json:"title"`          // contoh: "Pengajuan cuti disetujui"
	Description string `json:"description"`    // contoh: "Cuti Tahunan 3–5 Jun telah disetujui HR."
	Note        string `json:"note,omitempty"` // catatan dari admin jika ditolak
	IsRead      bool   `json:"is_read"`        // false = ada dot biru di FE
	UpdatedAt   string `json:"updated_at"`     // kapan status terakhir berubah
}

// NotificationResponse — response GET /employee/leave/notifications
type NotificationResponse struct {
	Data        []NotificationItem `json:"data"`
	Total       int                `json:"total"`
	Page        int                `json:"page"`
	Limit       int                `json:"limit"`
	TotalPages  int                `json:"total_pages"`
	UnreadCount int                `json:"unread_count"` // untuk badge angka di icon notif
}
