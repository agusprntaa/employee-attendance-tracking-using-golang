package router

import (
	"absensi/config"
	"absensi/handlers"
	"absensi/middleware"
	"absensi/repository"
	"absensi/utils"
	"database/sql"
	"net/http"
)

func SetupRouter(db *sql.DB, cfg *config.Config) http.Handler {
	// Set DB ke middleware untuk lookup branch_id
	middleware.SetDB(db)

	// ─── Repositories ──────────────────────────────────────────────────────────
	empRepo        := repository.NewEmployeeRepo(db)
	attendanceRepo := repository.NewAttendanceRepo(db)
	branchRepo     := repository.NewBranchRepo(db)
	divRepo        := repository.NewDivisionRepo(db)
	settingsRepo   := repository.NewSettingsRepo(db)
	qrRepo         := repository.NewQRRepo(db)

	// ─── Handlers ──────────────────────────────────────────────────────────────
	dashboardH  := handlers.NewDashboardHandler(attendanceRepo)
	employeeH   := handlers.NewEmployeeHandler(empRepo)
	attendanceH := handlers.NewAttendanceHandler(attendanceRepo)
	branchH     := handlers.NewBranchHandler(branchRepo)
	divisionH   := handlers.NewDivisionHandler(divRepo, empRepo, attendanceRepo)
	settingsH   := handlers.NewSettingsHandler(settingsRepo)
	qrH         := handlers.NewQRHandler(qrRepo)

	// ─── Public Mux (tanpa auth) ───────────────────────────────────────────────
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		utils.Success(w, map[string]string{"status": "ok", "service": "backend2-admin-cabang"})
	})

	// ─── Protected Mux (dengan auth) ───────────────────────────────────────────
	protectedMux := http.NewServeMux()

	// Dashboard
	protectedMux.HandleFunc("/admin-cabang/dashboard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(dashboardH.GetDashboard)(w, r)
	})

	// QR Code
	protectedMux.HandleFunc("/admin-cabang/qr/today", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(qrH.GetTodayQR)(w, r)
	})
	protectedMux.HandleFunc("/admin-cabang/qr/regenerate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(qrH.RegenerateQR)(w, r)
	})

	// Employees
	protectedMux.HandleFunc("/admin-cabang/employees", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(employeeH.List)(w, r)
		case http.MethodPost:
			middleware.RequireAdminCabang(employeeH.Create)(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	protectedMux.HandleFunc("/admin-cabang/employees/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(employeeH.GetByID)(w, r)
		case http.MethodPatch:
			middleware.RequireAdminCabang(employeeH.Update)(w, r)
		case http.MethodDelete:
			middleware.RequireAdminCabang(employeeH.Deactivate)(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Attendance
	protectedMux.HandleFunc("/admin-cabang/attendance/today", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(attendanceH.TodayAttendance)(w, r)
	})
	protectedMux.HandleFunc("/admin-cabang/attendance/employee/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(attendanceH.EmployeeHistory)(w, r)
	})

	// Reports
	protectedMux.HandleFunc("/admin-cabang/reports/attendance", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(attendanceH.AttendanceReport)(w, r)
	})

	// Branch
	protectedMux.HandleFunc("/admin-cabang/branch", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(branchH.GetBranch)(w, r)
		case http.MethodPatch:
			middleware.RequireAdminCabang(branchH.UpdateBranch)(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Settings
	protectedMux.HandleFunc("/admin-cabang/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(settingsH.GetSettings)(w, r)
		case http.MethodPatch:
			middleware.RequireAdminCabang(settingsH.UpdateSettings)(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Schedules
	protectedMux.HandleFunc("/admin-cabang/schedules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(divisionH.GetSchedules)(w, r)
	})

	// Divisions
	protectedMux.HandleFunc("/admin-cabang/divisions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(divisionH.List)(w, r)
		case http.MethodPost:
			middleware.RequireAdminCabang(divisionH.Create)(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	protectedMux.HandleFunc("/admin-cabang/divisions/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(divisionH.GetByID)(w, r)
		case http.MethodPatch:
			middleware.RequireAdminCabang(divisionH.Update)(w, r)
		case http.MethodDelete:
			middleware.RequireAdminCabang(divisionH.Delete)(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// ─── Gabungkan public + protected ──────────────────────────────────────────
	authMw := middleware.AuthMiddleware(cfg)
	mainMux := http.NewServeMux()
	mainMux.Handle("/health", middleware.CORS(publicMux))
	mainMux.Handle("/admin-cabang/", middleware.CORS(authMw(protectedMux)))

	return mainMux
}
