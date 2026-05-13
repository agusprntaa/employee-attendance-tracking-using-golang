package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	// CORE
	"absensi_karyawan/auth"
	"absensi_karyawan/config"
	"absensi_karyawan/database"
	"absensi_karyawan/router"

	// BE1
	"absensi_karyawan/attendance"
	"absensi_karyawan/employee"

	// BE2
	"absensi_karyawan/handlers"
	"absensi_karyawan/repository"
)

func main() {

	// =====================================================
	// APP
	// =====================================================

	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:5173",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, ngrok-skip-browser-warning",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	// =====================================================
	// DATABASE
	// =====================================================

	db := database.ConnectDB()

	// =====================================================
	// CONFIG
	// =====================================================

	cfg := &config.Config{}
	_ = cfg // sementara jika belum dipakai

	// =====================================================
	// AUTH MODULE
	// =====================================================

	authRepo := &auth.Repository{
		DB: db,
	}

	authService := &auth.Service{
		Repo: authRepo,
	}

	authHandler := &auth.Handler{
		Service: authService,
	}

	// =====================================================
	// ATTENDANCE MODULE (KARYAWAN)
	// =====================================================

	attendanceRepo := &attendance.Repository{
		DB: db,
	}

	attendanceService := &attendance.Service{
		Repo: attendanceRepo,
	}

	attendanceHandler := &attendance.Handler{
		Service: attendanceService,
	}

	// =====================================================
	// EMPLOYEE MODULE (KARYAWAN)
	// =====================================================

	employeeRepo := &employee.Repository{
		DB: db,
	}

	employeeHandler := &employee.Handler{
		Repo: employeeRepo,
	}

	// =====================================================
	// BE2 REPOSITORIES (ADMIN PANEL)
	// =====================================================

	empRepo := repository.NewEmployeeRepo(db)

	attendanceRepo2 := repository.NewAttendanceRepo(db)

	branchRepo := repository.NewBranchRepo(db)

	divRepo := repository.NewDivisionRepo(db)

	settingsRepo := repository.NewSettingsRepo(db)

	qrRepo := repository.NewQRRepo(db)

	// =====================================================
	// BE2 HANDLERS (ADMIN PANEL)
	// =====================================================

	dashboardH := handlers.NewDashboardHandler(
		attendanceRepo2,
	)

	employeeH2 := handlers.NewEmployeeHandler(
		empRepo,
	)

	attendanceH2 := handlers.NewAttendanceHandler(
		attendanceRepo2,
	)

	branchH := handlers.NewBranchHandler(
		branchRepo,
	)

	divisionH := handlers.NewDivisionHandler(
		divRepo,
		empRepo,
		attendanceRepo2,
	)

	settingsH := handlers.NewSettingsHandler(
		settingsRepo,
	)

	qrH := handlers.NewQRHandler(
		qrRepo,
	)

	// =====================================================
	// LIMITER
	// =====================================================

	loginLimiter := limiter.New(limiter.Config{
		Max:        15,
		Expiration: 15 * time.Minute,
	})

	// =====================================================
	// PUBLIC ROUTES
	// =====================================================

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	app.Post("/login", loginLimiter, authHandler.Login)

	app.Post("/refresh", authHandler.Refresh)

	app.Post("/logout", authHandler.Logout)

	// =====================================================
	// PROTECTED ROUTES (WAJIB LOGIN)
	// =====================================================

	api := app.Group(
		"/",
		auth.AuthMiddleware,
	)

	// =====================================================
	// KARYAWAN ROUTES
	// Semua role yang sudah login bisa akses
	// =====================================================

	// Employee
	api.Get(
		"/employee/profile",
		employeeHandler.GetProfile,
	)

	api.Patch(
		"/employee/change-password",
		employeeHandler.ChangePassword,
	)

	// Attendance
	api.Post(
		"/attendance/checkin",
		attendanceHandler.CheckIn,
	)

	api.Patch(
		"/attendance/checkout",
		attendanceHandler.CheckOut,
	)

	api.Get(
		"/attendance/today",
		attendanceHandler.GetToday,
	)

	api.Get(
		"/attendance/history",
		attendanceHandler.GetHistory,
	)

	// =====================================================
	// ADMIN CABANG ROUTES
	// super_admin + admin_cabang
	// =====================================================

	admin := api.Group(
		"/admin-cabang",
		auth.RequireAdmin,
	)

	// Dashboard
	admin.Get(
		"/dashboard",
		dashboardH.GetDashboard,
	)

	// Employees
	admin.Get(
		"/employees",
		employeeH2.List,
	)

	admin.Post(
		"/employees",
		employeeH2.Create,
	)

	admin.Get(
		"/employees/:id",
		employeeH2.GetByID,
	)

	admin.Patch(
		"/employees/:id",
		employeeH2.Update,
	)

	admin.Delete(
		"/employees/:id",
		employeeH2.Delete,
	)

	admin.Patch(
		"/employees/:id/activate",
		employeeH2.Activate,
	)

	admin.Patch(
		"/employees/:id/deactivate",
		employeeH2.Deactivate,
	)

	// Attendance Monitoring
	admin.Get(
		"/attendance/today",
		attendanceH2.TodayAttendance,
	)

	admin.Get(
		"/attendance/employee/:id",
		attendanceH2.EmployeeHistory,
	)

	// Reports
	admin.Get(
		"/reports/attendance",
		attendanceH2.AttendanceReport,
	)

	// Branch
	admin.Get(
		"/branch",
		branchH.GetBranch,
	)

	admin.Patch(
		"/branch",
		branchH.UpdateBranch,
	)

	// Settings
	admin.Get(
		"/settings",
		settingsH.GetSettings,
	)

	admin.Patch(
		"/settings",
		settingsH.UpdateSettings,
	)

	// Divisions
	admin.Get(
		"/divisions",
		divisionH.List,
	)

	admin.Post(
		"/divisions",
		divisionH.Create,
	)

	admin.Get(
		"/divisions/:id",
		divisionH.GetByID,
	)

	admin.Patch(
		"/divisions/:id",
		divisionH.Update,
	)

	admin.Delete(
		"/divisions/:id",
		divisionH.Delete,
	)

	// Schedule
	admin.Get(
		"/schedules",
		divisionH.GetSchedules,
	)

	// =====================================================
	// QR ROUTES
	// HANYA admin_cabang
	// super_admin ditolak
	// =====================================================

	qr := api.Group(
		"/qr",
		auth.RequireAdminCabangOnly,
	)

	qr.Get(
		"/today",
		qrH.GetTodayQR,
	)

	qr.Post(
		"/regenerate",
		qrH.RegenerateQR,
	)

	// =====================================================
	// SUPER ADMIN ROUTES
	// Hanya super_admin
	// =====================================================

	superAdmin := api.Group(
		"/super-admin",
		auth.RequireSuperAdmin,
	)

	superAdmin.Post(
		"/create-admin",
		authHandler.CreateUser,
	)

	// OPTIONAL:
	// superAdmin.Get("/branches", branchH.ListBranches)
	// superAdmin.Post("/branches", branchH.CreateBranch)

	router.SetupGlobalAdminRoutes(app, db)

	// =====================================================
	// RUN SERVER
	// =====================================================

	log.Println("Server running on http://localhost:3000")

	log.Fatal(app.Listen(":3000"))
}
