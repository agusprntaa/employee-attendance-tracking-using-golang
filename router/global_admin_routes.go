package router

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/handlers"
	"absensi_karyawan/repository"

	"database/sql"

	"github.com/gofiber/fiber/v2"
)

func SetupGlobalAdminRoutes(app *fiber.App, db *sql.DB) {

	globalAdminRepo := repository.NewGlobalAdminRepository(db)
	globalAdminHandler := handlers.NewGlobalAdminHandler(globalAdminRepo)

	// PREFIX API
	global := app.Group("/api/global")

	// middleware
	global.Use(auth.AuthMiddleware)
	global.Use(auth.RequirePusatRole)

	// dashboard
	global.Get("/dashboard", globalAdminHandler.GetDashboard)

	// employees
	global.Get("/employees", globalAdminHandler.GetAllEmployees)
	global.Get("/employees/:id", globalAdminHandler.GetEmployeeDetail)

	// attendance
	global.Get("/attendance/today", globalAdminHandler.GetTodayAttendance)
	global.Get("/attendance/analytics", globalAdminHandler.GetAttendanceAnalytics)

	// branches
	global.Get("/branches", globalAdminHandler.GetAllBranches)
	global.Get("/branches/:id", globalAdminHandler.GetBranchDetail)

	global.Post("/branches", globalAdminHandler.CreateBranch)
	global.Put("/branches/:id", globalAdminHandler.UpdateBranch)
	global.Delete("/branches/:id", globalAdminHandler.DeleteBranch)
}
