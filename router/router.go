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

	// ─── Main Mux ──────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Health check (public, tanpa auth)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		utils.Success(w, map[string]string{"status": "ok", "service": "backend2-admin-cabang"})
	})

	// ─── Auth Middleware ────────────────────────────────────────────────────────
	authMw := middleware.AuthMiddleware(cfg)

	// ─── Dashboard ─────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/dashboard", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(dashboardH.GetDashboard)(w, r)
	})))

	// ─── QR Code ───────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/qr/today", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(qrH.GetTodayQR)(w, r)
	})))

	mux.Handle("/admin-cabang/qr/regenerate", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(qrH.RegenerateQR)(w, r)
	})))

	// ─── Employees ─────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/employees", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(employeeH.List)(w, r)
		case http.MethodPost:
			middleware.RequireAdminCabang(employeeH.Create)(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/admin-cabang/employees/", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})))

	// ─── Attendance ────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/attendance/today", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(attendanceH.TodayAttendance)(w, r)
	})))

	mux.Handle("/admin-cabang/attendance/employee/", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(attendanceH.EmployeeHistory)(w, r)
	})))

	// ─── Reports ───────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/reports/attendance", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(attendanceH.AttendanceReport)(w, r)
	})))

	// ─── Branch ────────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/branch", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(branchH.GetBranch)(w, r)
		case http.MethodPatch:
			middleware.RequireAdminCabang(branchH.UpdateBranch)(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	// ─── Settings ──────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/settings", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(settingsH.GetSettings)(w, r)
		case http.MethodPatch:
			middleware.RequireAdminCabang(settingsH.UpdateSettings)(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	// ─── Schedules ─────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/schedules", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		middleware.RequireAdminCabang(divisionH.GetSchedules)(w, r)
	})))

	// ─── Divisions ─────────────────────────────────────────────────────────────
	mux.Handle("/admin-cabang/divisions", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequireAdminCabang(divisionH.List)(w, r)
		case http.MethodPost:
			middleware.RequireAdminCabang(divisionH.Create)(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/admin-cabang/divisions/", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})))

	// ─── Wrap semua dengan CORS ─────────────────────────────────────────────────
	// Support ngrok dan semua origin frontend
	return middleware.CORS(mux)
}
