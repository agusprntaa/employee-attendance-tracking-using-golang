package models

import "time"

// ─── EMPLOYEE ────────────────────────────────────────────────────────────────
// Tabel: id, username, password, role, tipe, division_id, branch_id, status, created_at

type Employee struct {
	ID         int       `json:"id"`
	Username   string    `json:"username"`
	Role       string    `json:"role"`
	Tipe       string    `json:"tipe"`
	DivisionID *int      `json:"division_id,omitempty"`
	BranchID   *int      `json:"branch_id,omitempty"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type EmployeeDetail struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	Tipe         string    `json:"tipe"`
	Status       string    `json:"status"`
	DivisionID   *int      `json:"division_id,omitempty"`
	DivisionName string    `json:"division_name,omitempty"`
	BranchID     *int      `json:"branch_id,omitempty"`
	BranchName   string    `json:"branch_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type CreateEmployeeRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Role       string `json:"role"`
	Tipe       string `json:"tipe"`
	DivisionID *int   `json:"division_id"`
}

type UpdateEmployeeRequest struct {
	Role       string `json:"role"`
	Status     string `json:"status"`
	DivisionID *int   `json:"division_id"`
}

// ─── BRANCH ──────────────────────────────────────────────────────────────────
// Tabel: id, name, address, latitude, longitude, radius_meter
//        + kolom baru: auto_refresh_qr, require_admin_approval,
//                      admin_email, email_notifications,
//                      late_arrival_alerts, weekly_reports

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
// Tabel: id, name, work_days, work_start, work_end, late_tolerance_min, checkin_cutoff_min

type Division struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	WorkDays         string `json:"work_days"`
	WorkStart        string `json:"work_start"`
	WorkEnd          string `json:"work_end"`
	LateToleanceMin  int    `json:"late_tolerance_min"`
	CheckinCutoffMin int    `json:"checkin_cutoff_min"`
}

type CreateDivisionRequest struct {
	Name             string `json:"name"`
	WorkDays         string `json:"work_days"`
	WorkStart        string `json:"work_start"`
	WorkEnd          string `json:"work_end"`
	LateToleanceMin  int    `json:"late_tolerance_min"`
	CheckinCutoffMin int    `json:"checkin_cutoff_min"`
}

// ─── ATTENDANCE ──────────────────────────────────────────────────────────────
// Tabel: id, employee_id, date, check_in, check_out, check_in_lat, check_in_lon,
//        distance_meter, status, late_minutes, work_mode, work_type,
//        wfa_reason, early_leave_reason, is_auto_checkout, branch_id

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

// ─── DASHBOARD STATS ─────────────────────────────────────────────────────────

type DashboardStats struct {
	TotalEmployee int `json:"total_employee"`
	Present       int `json:"present"`
	Late          int `json:"late"`
	WFA           int `json:"wfa"`
	Absent        int `json:"absent"`
}

// ─── QR CODE ─────────────────────────────────────────────────────────────────
// Tabel: id, token, branch_id, date, expires_at, created_at

type QRData struct {
	Token     string    `json:"token"`
	BranchID  int       `json:"branch_id"`
	Date      string    `json:"date"`
	ExpiresAt time.Time `json:"expires_at"`
}

type QRResponse struct {
	Token      string `json:"token"`
	BranchID   int    `json:"branch_id"`
	Date       string `json:"date"`
	ExpiresAt  string `json:"expires_at"`
	QRContent  string `json:"qr_content"`  // JSON string → Frontend render jadi gambar QR
	RefreshIn  int    `json:"refresh_in"`  // detik sampai refresh berikutnya
}

// ─── SETTINGS ────────────────────────────────────────────────────────────────
// Gabungan dari tabel branches (semua kolom) + divisions

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
	// Branch Information
	BranchName  string  `json:"branch_name"`
	Address     string  `json:"address"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	RadiusMeter int     `json:"radius_meter"`

	// Working Hours
	DivisionID       *int   `json:"division_id"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	LateThresholdMin int    `json:"late_threshold_min"`
	CheckinCutoffMin int    `json:"checkin_cutoff_min"`
	WorkDays         string `json:"work_days"`

	// Security → disimpan ke tabel branches
	AutoRefreshQR        bool `json:"auto_refresh_qr"`
	RequireAdminApproval bool `json:"require_admin_approval"`

	// Notifications → disimpan ke tabel branches
	AdminEmail         string `json:"admin_email"`
	EmailNotifications bool   `json:"email_notifications"`
	LateArrivalAlerts  bool   `json:"late_arrival_alerts"`
	WeeklyReports      bool   `json:"weekly_reports"`
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

// ReportSummary - kartu ringkasan di bagian atas halaman Reports
type ReportSummary struct {
	// Minggu ini
	AverageAttendanceRate float64 `json:"average_attendance_rate"`
	TotalPresent          int     `json:"total_present"`
	LateArrivals          int     `json:"late_arrivals"`

	// Perbandingan dengan minggu lalu (persen, + naik / - turun)
	AttendanceRateChange float64 `json:"attendance_rate_change"`
	TotalPresentChange   float64 `json:"total_present_change"`
	LateArrivalsChange   float64 `json:"late_arrivals_change"`
}

// FullReportResponse - response lengkap halaman Reports
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
