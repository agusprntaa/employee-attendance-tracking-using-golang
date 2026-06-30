package models

import "time"

// ─── EMPLOYEE ────────────────────────────────────────────────────────────────

type Employee struct {
	ID         int       `json:"id"`
	Username   string    `json:"username"`
	Name       string    `json:"full_name"` // tambah Name
	Role       string    `json:"role"`
	Tipe       string    `json:"tipe"`
	DivisionID *int      `json:"division_id,omitempty"`
	BranchID   *int      `json:"branch_id,omitempty"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type EmployeeDetail struct {
	ID        int    `json:"id"`
	FullName  string `json:"full_name"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
	BirthDate string `json:"birth_date"`
	PhotoURL  string `json:"photo_url"`

	Role   string `json:"role"`
	Tipe   string `json:"tipe"`
	Status string `json:"status"`

	DivisionID   *int   `json:"division_id"`
	DivisionName string `json:"division_name"`

	BranchID   int    `json:"branch_id"`
	BranchName string `json:"branch_name"`

	CreatedAt time.Time `json:"created_at"`
}

type CreateEmployeeRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Name       string `json:"full_name"`
	Role       string `json:"role"`
	Tipe       string `json:"tipe"`
	DivisionID *int   `json:"division_id"`
}

type UpdateEmployeeRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Name       string `json:"full_name"`
	Role       string `json:"role"`
	Tipe       string `json:"tipe"`
	DivisionID *int   `json:"division_id"`
	Status     string `json:"status"`
}

// ─── BRANCH ──────────────────────────────────────────────────────────────────

type Branch struct {
	ID                   int     `json:"id"`
	Name                 string  `json:"name"`
	Address              string  `json:"address"`
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	RadiusMeter          int     `json:"radius_meter"`
	AutoRefreshQR        bool    `json:"auto_refresh_qr"`
	RequireAdminApproval bool    `json:"require_admin_approval"`
	AdminEmail           string  `json:"admin_email"`
	EmailNotifications   bool    `json:"email_notifications"`
	LateArrivalAlerts    bool    `json:"late_arrival_alerts"`
	WeeklyReports        bool    `json:"weekly_reports"`
}

type UpdateBranchRequest struct {
	Name        string  `json:"name"`
	Address     string  `json:"address"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	RadiusMeter int     `json:"radius_meter"`
}

// ─── DIVISION ────────────────────────────────────────────────────────────────

type Division struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	WorkDays         string `json:"work_days"`
	WorkStart        string `json:"work_start"`
	WorkEnd          string `json:"work_end"`
	LateToleranceMin int    `json:"late_tolerance_min"`
	CheckinCutoffMin int    `json:"checkin_cutoff_min"`
}

type CreateDivisionRequest struct {
	Name             string `json:"name"`
	WorkDays         string `json:"work_days"`
	WorkStart        string `json:"work_start"`
	WorkEnd          string `json:"work_end"`
	LateToleranceMin int    `json:"late_tolerance_min"`
	CheckinCutoffMin int    `json:"checkin_cutoff_min"`
}

// ─── ATTENDANCE ──────────────────────────────────────────────────────────────

type Attendance struct {
	ID               int        `json:"id"`
	EmployeeID       int        `json:"employee_id"`
	EmployeeUsername string     `json:"employee_username,omitempty"`
	Date             string     `json:"date"`
	WorkMode         string     `json:"work_mode,omitempty"`
	WorkType         string     `json:"work_type"`
	Status           string     `json:"status"`
	CheckIn          *time.Time `json:"check_in"`
	CheckOut         *time.Time `json:"check_out"`
	CheckInLat       *float64   `json:"check_in_lat,omitempty"`
	CheckInLon       *float64   `json:"check_in_lon,omitempty"`
	LateMinutes      int        `json:"late_minutes"`
	DistanceMeter    *float64   `json:"distance_meter,omitempty"`
	IsAutoCheckout   bool       `json:"is_auto_checkout"`
	WFAReason        string     `json:"wfa_reason,omitempty"`
	EarlyLeaveReason string     `json:"early_leave_reason,omitempty"`
	BranchID         *int       `json:"branch_id,omitempty"`
}

// ─── DASHBOARD ───────────────────────────────────────────────────────────────

type DashboardStats struct {
	TotalEmployee int `json:"total_employee"`
	Present       int `json:"present"`
	Late          int `json:"late"`
	WFA           int `json:"wfa"`
	Absent        int `json:"absent"`
}

// ─── QR CODE ─────────────────────────────────────────────────────────────────

type QRData struct {
	Token     string    `json:"token"`
	BranchID  int       `json:"branch_id"`
	Date      string    `json:"date"`
	ExpiresAt time.Time `json:"expires_at"`
}

type QRResponse struct {
	Token     string `json:"token"`
	BranchID  int    `json:"branch_id"`
	Date      string `json:"date"`
	ExpiresAt string `json:"expires_at"`
	QRContent string `json:"qr_content"`
	RefreshIn int    `json:"refresh_in"`
}

// ─── SETTINGS ────────────────────────────────────────────────────────────────

type SettingsResponse struct {
	BranchInformation BranchInformation `json:"branch_information"`
	WorkingHours      WorkingHours      `json:"working_hours"`
	Security          SecuritySettings  `json:"security"`
	Notifications     NotifSettings     `json:"notifications"`
}

type BranchInformation struct {
	BranchName  string  `json:"branch_name"`
	Address     string  `json:"address"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	RadiusMeter int     `json:"radius_meter"`
}

type WorkingHours struct {
	DivisionID       *int   `json:"division_id,omitempty"`
	DivisionName     string `json:"division_name,omitempty"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	LateThresholdMin int    `json:"late_threshold_min"`
	CheckinCutoffMin int    `json:"checkin_cutoff_min"`
	WorkDays         string `json:"work_days"`
}

type SecuritySettings struct {
	AutoRefreshQR        bool `json:"auto_refresh_qr"`
	RequireAdminApproval bool `json:"require_admin_approval"`
}

type NotifSettings struct {
	AdminEmail         string `json:"admin_email"`
	EmailNotifications bool   `json:"email_notifications"`
	LateArrivalAlerts  bool   `json:"late_arrival_alerts"`
	WeeklyReports      bool   `json:"weekly_reports"`
}

type UpdateSettingsRequest struct {
	BranchName           string  `json:"branch_name"`
	Address              string  `json:"address"`
	Latitude             float64 `json:"latitude"`
	Longitude            float64 `json:"longitude"`
	RadiusMeter          int     `json:"radius_meter"`
	DivisionID           *int    `json:"division_id"`
	StartTime            string  `json:"start_time"`
	EndTime              string  `json:"end_time"`
	LateThresholdMin     int     `json:"late_threshold_min"`
	CheckinCutoffMin     int     `json:"checkin_cutoff_min"`
	WorkDays             string  `json:"work_days"`
	AutoRefreshQR        bool    `json:"auto_refresh_qr"`
	RequireAdminApproval bool    `json:"require_admin_approval"`
	AdminEmail           string  `json:"admin_email"`
	EmailNotifications   bool    `json:"email_notifications"`
	LateArrivalAlerts    bool    `json:"late_arrival_alerts"`
	WeeklyReports        bool    `json:"weekly_reports"`
}

// ─── REPORT ──────────────────────────────────────────────────────────────────

type AttendanceReport struct {
	Date         string `json:"date"`
	TotalPresent int    `json:"total_present"`
	TotalLate    int    `json:"total_late"`
	TotalWFA     int    `json:"total_wfa"`
	TotalAbsent  int    `json:"total_absent"`
}

type DivisionReport struct {
	Status         string  `json:"status"`
	DivisionName   string  `json:"division_name"`
	TotalEmployees int     `json:"total_employees"`
	AttendanceRate float64 `json:"attendance_rate"`
}

// ─── PAGINATION ──────────────────────────────────────────────────────────────

type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

// ─── REPORT SUMMARY ──────────────────────────────────────────────────────────

type ReportSummary struct {
	AverageAttendanceRate float64 `json:"average_attendance_rate"`
	TotalPresent          int     `json:"total_present"`
	LateArrivals          int     `json:"late_arrivals"`
	AttendanceRateChange  float64 `json:"attendance_rate_change"`
	TotalPresentChange    float64 `json:"total_present_change"`
	LateArrivalsChange    float64 `json:"late_arrivals_change"`
}

type FullReportResponse struct {
	Summary  ReportSummary      `json:"summary"`
	Daily    []AttendanceReport `json:"daily"`
	Division []DivisionReport   `json:"division"`
}

// ─── SCHEDULE ────────────────────────────────────────────────────────────────

type EmployeeSchedule struct {
	EmployeeID       int    `json:"employee_id"`
	EmployeeUsername string `json:"employee_username"`
	Division         string `json:"division"`
	Monday           string `json:"monday"`
	Tuesday          string `json:"tuesday"`
	Wednesday        string `json:"wednesday"`
	Thursday         string `json:"thursday"`
	Friday           string `json:"friday"`
	Saturday         string `json:"saturday"`
	Sunday           string `json:"sunday"`
}

type ScheduleResponse struct {
	Week      string             `json:"week"`
	StartDate string             `json:"start_date"`
	EndDate   string             `json:"end_date"`
	Schedules []EmployeeSchedule `json:"schedules"`
}

//─── LEAVE (CUTI) ────────────────────────────────────────────────────────────

// LeaveQuota — kuota cuti karyawan per tahun
type LeaveQuota struct {
	ID         int `json:"id"`
	EmployeeID int `json:"employee_id"`
	Year       int `json:"year"`
	Total      int `json:"total"`
	Used       int `json:"used"`
	Remaining  int `json:"remaining"` // computed: total - used
}

// LeaveRequest — pengajuan cuti karyawan
type LeaveRequest struct {
	ID           int     `json:"id"`
	EmployeeID   int     `json:"employee_id"`
	EmployeeName string  `json:"employee_name,omitempty"`
	DivisionName string  `json:"division_name,omitempty"`
	LeaveType    string  `json:"leave_type"` // cuti_tahunan | cuti_sakit | cuti_pribadi | cuti_melahirkan
	StartDate    string  `json:"start_date"`
	EndDate      string  `json:"end_date"`
	TotalDays    int     `json:"total_days,omitempty"`
	Reason       string  `json:"reason"`
	Status       string  `json:"status"` // pending | approved | rejected
	Note         *string `json:"note,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// PublicHoliday — hari libur nasional
type PublicHoliday struct {
	ID          int    `json:"id"`
	Date        string `json:"date"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// LeaveSummary — statistik kartu atas UI
type LeaveSummary struct {
	TotalRequests int `json:"total_requests"`
	Pending       int `json:"pending"`
	Approved      int `json:"approved"`
	Rejected      int `json:"rejected"`
}

// UpdateLeaveStatusRequest — untuk approve/reject pengajuan cuti
type UpdateLeaveStatusRequest struct {
	Status string  `json:"status"` // "approved" atau "rejected"
	Note   *string `json:"note"`   // opsional, catatan admin
}

// UpdateLeaveQuotaRequest — set kuota manual oleh admin
type UpdateLeaveQuotaRequest struct {
	Total int `json:"total"`
}

// CreateHolidayRequest — tambah hari libur nasional
type CreateHolidayRequest struct {
	Date        string `json:"date"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
