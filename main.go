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
	"absensi_karyawan/face"
	"absensi_karyawan/leave"
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

	app.Use(func(c *fiber.Ctx) error {
		c.Set("ngrok-skip-browser-warning", "true")
		return c.Next()
	})

	// =====================================================
	// DATABASE
	// =====================================================

	db := database.ConnectDB()

	// =====================================================
	// FACE MODULE
	// =====================================================

	faceRepo := &face.Repository{
		DB: db,
	}

	faceEngine := face.NewInsightFaceEngine(
		"http://localhost:3000",
	)

	// alternatif AWS:
	// faceEngine := face.NewAWSEngine("ap-southeast-1")

	faceService := &face.Service{
		Repo:      faceRepo,
		Engine:    faceEngine,
		DB:        db,
		Threshold: 0.80,
	}

	faceHandler := &face.Handler{
		Service: faceService,
	}

	// =====================================================
	// STATIC FILE SERVING
	// Foto profil  → /uploads/photos/<filename>
	// Attachment   → /uploads/attachments/<filename>
	// =====================================================

	app.Static("/uploads", "./uploads") // ← TAMBAHAN

	// =====================================================
	// CONFIG
	// =====================================================

	cfg := &config.Config{}
	_ = cfg

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
		Repo:     employeeRepo,
		AuthRepo: authRepo,
	}

	app.Get(
		"/employee/profile/photo/view/:filename",
		employeeHandler.ViewPhoto,
	)

	// =====================================================
	// LEAVE MODULE (KARYAWAN)
	// =====================================================

	leaveRepo := &leave.Repository{DB: db}
	leaveService := &leave.Service{Repo: leaveRepo}
	leaveHandler := &leave.Handler{Service: leaveService}

	// Jalankan cron job reset kuota tiap 1 Januari jam 00:01 WITA
	leave.StartCronJob(leaveRepo)

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
		authRepo,
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
		Max:        30,
		Expiration: 30 * time.Minute,
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

	// ── Employee ─────────────────────────────────────────

	api.Get(
		"/employee/profile",
		employeeHandler.GetProfile,
	)

	api.Patch(
		"/employee/change-password",
		employeeHandler.ChangePassword,
	)

	api.Patch(
		"/employee/profile",
		employeeHandler.UpdateProfile,
	)

	api.Post(
		"/employee/profile/photo",
		employeeHandler.UploadPhoto,
	)

	api.Delete(
		"/employee/profile/photo",
		employeeHandler.DeletePhoto,
	)

	// ── Face / Onboarding ────────────────────────────

	api.Get(
		"/employee/onboarding-status",
		faceHandler.GetOnboardingStatus,
	)

	api.Get(
		"/employee/face/status",
		faceHandler.GetFaceStatus,
	)

	api.Post(
		"/employee/face/register",
		faceHandler.RegisterFace,
	)

	api.Post(
		"/attendance/face-token",
		faceHandler.GenerateFaceToken,
	)

	api.Post(
		"/attendance/checkin-verify",
		faceHandler.VerifyAndCheckin,
	)

	// ── Attendance ───────────────────────────────────────

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

	// ── Leave (Cuti) ─────────────────────────────────────

	api.Get(
		"/employee/leave/types",
		leaveHandler.GetLeaveTypes,
	)

	api.Get(
		"/employee/leave/quota",
		leaveHandler.GetMyQuota,
	)

	api.Post(
		"/employee/leave/request",
		leaveHandler.RequestLeave,
	)

	api.Get(
		"/employee/leave/history",
		leaveHandler.GetMyHistory,
	)

	api.Patch(
		"/employee/leave/:id/cancel",
		leaveHandler.CancelLeave,
	)

	api.Get(
		"/employee/leave/holidays",
		leaveHandler.GetHolidays,
	)

	api.Get(
		"/employee/leave/notifications",
		leaveHandler.GetNotifications,
	)

	api.Patch(
		"/employee/leave/notifications/read-all",
		leaveHandler.MarkAllNotificationsRead,
	)

	api.Patch(
		"/employee/leave/notifications/:id/read",
		leaveHandler.MarkNotificationRead,
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

	router.SetupGlobalAdminRoutes(app, db)

	// =====================================================
	// RUN SERVER
	// =====================================================

	log.Println("Server running on http://localhost:3000")

	log.Fatal(app.Listen(":3000"))
}
