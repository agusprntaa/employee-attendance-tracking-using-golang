package models

import "time"

// =============================================
// GLOBAL ADMIN DASHBOARD STATS
// =============================================

type GlobalDashboardStats struct {
	TotalEmployees int     `json:"total_employees"`
	PresentToday   int     `json:"present_today"`
	AttendanceRate float64 `json:"attendance_rate"`
	TotalBranches  int     `json:"total_branches"`
	RateChange     float64 `json:"rate_change"`
}

type BranchAttendanceBar struct {
	BranchName string `json:"branch_name"`
	Present    int    `json:"present"`
	Capacity   int    `json:"capacity"`
}

type WorkModeDistribution struct {
	WFOCount   int     `json:"wfo_count"`
	WFACount   int     `json:"wfa_count"`
	WFOPercent float64 `json:"wfo_percent"`
	WFAPercent float64 `json:"wfa_percent"`
}

type BranchPerformanceRow struct {
	Branch         string  `json:"branch"`
	TotalEmployees int     `json:"total_employees"`
	Present        int     `json:"present"`
	Absent         int     `json:"absent"`
	Rate           float64 `json:"rate"`
	WFO            int     `json:"wfo"`
	WFA            int     `json:"wfa"`
	Status         string  `json:"status"`
}

// =============================================
// EMPLOYEE LIST (klik Total Employees)
// =============================================

type GlobalEmployeeItem struct {
	EmployeeID  string    `json:"employee_id"`
	FullName    string    `json:"full_name"`
	Position    string    `json:"position"`
	Branch      string    `json:"branch"`
	Status      string    `json:"status"`
	CreatedDate time.Time `json:"created_date"`
}

// Employee Detail (klik icon mata di employee list)
type GlobalEmployeeDetail struct {
	EmployeeID     string    `json:"employee_id"`
	FullName       string    `json:"full_name"`
	Username       string    `json:"username"`
	Role           string    `json:"role"`
	Tipe           string    `json:"tipe"`
	Position       string    `json:"position"`
	Division       string    `json:"division"`
	Branch         string    `json:"branch"`
	BranchAddress  string    `json:"branch_address"`
	Status         string    `json:"status"`
	CreatedDate    time.Time `json:"created_date"`
	TotalPresent   int       `json:"total_present"`
	TotalAbsent    int       `json:"total_absent"`
	TotalLate      int       `json:"total_late"`
	AttendanceRate float64   `json:"attendance_rate"`
}

// =============================================
// TODAY ATTENDANCE OVERVIEW (klik Present Today)
// =============================================

type TodayAttendanceOverviewResponse struct {
	TotalPresent   int                     `json:"total_present"`
	TotalAbsent    int                     `json:"total_absent"`
	TotalEmployees int                     `json:"total_employees"`
	Branches       []BranchAttendanceToday `json:"branches"`
}

type BranchAttendanceToday struct {
	BranchName     string  `json:"branch_name"`
	TotalEmployees int     `json:"total_employees"`
	PresentToday   int     `json:"present_today"`
	Absent         int     `json:"absent"`
	AttendanceRate float64 `json:"attendance_rate"`
	Status         string  `json:"status"`
}

// =============================================
// ATTENDANCE ANALYTICS (klik Attendance Rate)
// =============================================

type AttendanceTrendPoint struct {
	Label      string  `json:"label"`
	Attendance float64 `json:"attendance"`
}

type BranchComparisonItem struct {
	BranchName string  `json:"branch_name"`
	Rate       float64 `json:"rate"`
}

// =============================================
// BRANCH MANAGEMENT (klik Total Branches)
// =============================================

type GlobalBranchItem struct {
	BranchID       string     `json:"branch_id"`
	BranchName     string     `json:"branch_name"`
	City           string     `json:"city"`
	Address        string     `json:"address"`
	TotalEmployees int        `json:"total_employees"`
	Status         string     `json:"status"`
	CreatedDate    *time.Time `json:"created_date"`
}

// Branch Detail (klik icon mata di branch list)
type GlobalBranchDetail struct {
	BranchID             string     `json:"branch_id"`
	BranchName           string     `json:"branch_name"`
	City                 string     `json:"city"`
	Address              string     `json:"address"`
	Latitude             float64    `json:"latitude"`
	Longitude            float64    `json:"longitude"`
	RadiusMeter          int        `json:"radius_meter"`
	AutoRefreshQR        bool       `json:"auto_refresh_qr"`
	RequireAdminApproval bool       `json:"require_admin_approval"`
	AdminEmail           string     `json:"admin_email"`
	EmailNotifications   bool       `json:"email_notifications"`
	LateArrivalAlerts    bool       `json:"late_arrival_alerts"`
	WeeklyReports        bool       `json:"weekly_reports"`
	TotalEmployees       int        `json:"total_employees"`
	ActiveEmployees      int        `json:"active_employees"`
	InactiveEmployees    int        `json:"inactive_employees"`
	TodayPresent         int        `json:"today_present"`
	TodayAbsent          int        `json:"today_absent"`
	AttendanceRate30d    float64    `json:"attendance_rate_30d"`
	Status               string     `json:"status"`
	CreatedDate          *time.Time `json:"created_date"`
}
