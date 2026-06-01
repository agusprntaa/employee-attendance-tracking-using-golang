package router

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/config"
	"absensi_karyawan/handlers"
	"absensi_karyawan/repository"

	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func SetupRouter(db *sql.DB, cfg *config.Config) *fiber.App {
	app := fiber.New()

	// ─── CORS support ngrok dan frontend ───────────────────────────────────────
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Authorization,Content-Type,Accept,ngrok-skip-browser-warning",
	}))

	// ─── Repositories ──────────────────────────────────────────────────────────
	authRepo := &auth.Repository{DB: db}
	empRepo := repository.NewEmployeeRepo(db)
	attendanceRepo := repository.NewAttendanceRepo(db)
	branchRepo := repository.NewBranchRepo(db)
	divRepo := repository.NewDivisionRepo(db)
	settingsRepo := repository.NewSettingsRepo(db)
	qrRepo := repository.NewQRRepo(db)

	// TAMBAHAN LEAVE REPOSITORY
	leaveRepo := repository.NewLeaveRepo(db)

	// ─── Handlers ──────────────────────────────────────────────────────────────
	dashboardH := handlers.NewDashboardHandler(attendanceRepo)
	employeeH := handlers.NewEmployeeHandler(empRepo, authRepo)
	attendanceH := handlers.NewAttendanceHandler(attendanceRepo)
	branchH := handlers.NewBranchHandler(branchRepo)
	divisionH := handlers.NewDivisionHandler(divRepo, empRepo, attendanceRepo)
	settingsH := handlers.NewSettingsHandler(settingsRepo)
	qrH := handlers.NewQRHandler(qrRepo)

	// TAMBAHAN LEAVE HANDLER
	leaveH := handlers.NewLeaveHandler(leaveRepo)

	// ─── Health Check (public) ─────────────────────────────────────────────────
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "success",
			"data": fiber.Map{
				"status":  "ok",
				"service": "backend2-admin-cabang",
			},
		})
	})

	app.Get("/tes-route", func(c *fiber.Ctx) error {
		return c.SendString("ROUTE AKTIF")
	})

	// ─── Protected Routes (wajib token + role admin) ───────────────────────────
	admin := app.Group("/admin-cabang", auth.AuthMiddleware, auth.RequireAdmin)

	// Dashboard
	admin.Get("/dashboard", dashboardH.GetDashboard)

	// QR Code
	admin.Get("/qr/today", qrH.GetTodayQR)
	admin.Post("/qr/regenerate", qrH.RegenerateQR)

	// Employees
	admin.Get("/employees", employeeH.List)
	admin.Post("/employees", employeeH.Create)
	admin.Get("/employees/:id", employeeH.GetByID)
	admin.Patch("/employees/:id", employeeH.Update)
	admin.Delete("/employees/:id", employeeH.Delete)
	admin.Patch("/employees/:id/activate", employeeH.Activate)
	admin.Patch("/employees/:id/deactivate", employeeH.Deactivate)

	// Attendance
	admin.Get("/attendance/today", attendanceH.TodayAttendance)
	admin.Get("/attendance/employee/:id", attendanceH.EmployeeHistory)

	// Reports
	admin.Get("/reports/attendance", attendanceH.AttendanceReport)

	// Branch
	admin.Get("/branch", branchH.GetBranch)
	admin.Patch("/branch", branchH.UpdateBranch)

	// Settings
	admin.Get("/settings", settingsH.GetSettings)
	admin.Patch("/settings", settingsH.UpdateSettings)

	// Schedules
	admin.Get("/schedules", divisionH.GetSchedules)

	// Divisions
	admin.Get("/divisions", divisionH.List)
	admin.Post("/divisions", divisionH.Create)
	admin.Get("/divisions/:id", divisionH.GetByID)
	admin.Patch("/divisions/:id", divisionH.Update)
	admin.Delete("/divisions/:id", divisionH.Delete)

	// ─── LEAVE MANAGEMENT ──────────────────────────────────────────────────────

	// Leave Summary
	admin.Get("/leave/summary", leaveH.GetSummary)

	// Leave Requests
	admin.Get("/leave/requests", leaveH.GetAllRequests)
	admin.Patch("/leave/:id/status", leaveH.UpdateLeaveStatus)

	// Leave Quota
	admin.Get("/leave/quota/:employee_id", leaveH.GetLeaveQuota)
	admin.Patch("/leave/quota/:employee_id", leaveH.UpdateLeaveQuota)

	// Kalender
	admin.Get("/leave/calendar", leaveH.GetCalendarDots)
	admin.Get("/leave/calendar/detail", leaveH.GetCalendarDetail)

	// Holidays
	admin.Get("/holidays", leaveH.GetAllHolidays)
	admin.Post("/holidays", leaveH.CreateHoliday)
	admin.Delete("/holidays/:id", leaveH.DeleteHoliday)
	return app
}
