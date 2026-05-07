package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"absensi_karyawan/attendance"
	"absensi_karyawan/auth"
	"absensi_karyawan/database"
	"absensi_karyawan/employee"
	// "absensi_karyawan/employee"  ← comment dulu
	// "absensi_karyawan/qr"        ← comment dulu
)

func main() {
	app := fiber.New()
	app.Use(cors.New())

	db := database.ConnectDB()

	// Auth
	authRepo := &auth.Repository{DB: db}
	authService := &auth.Service{Repo: authRepo}
	authHandler := &auth.Handler{Service: authService}

	// Attendance
	attendanceRepo := &attendance.Repository{DB: db}
	attendanceService := &attendance.Service{Repo: attendanceRepo}
	attendanceHandler := &attendance.Handler{Service: attendanceService}

	// Employee — comment dulu sampai siap
	employeeRepo := &employee.Repository{DB: db}
	employeeHandler := &employee.Handler{Repo: employeeRepo}

	loginLimiter := limiter.New(limiter.Config{
		Max:        15,
		Expiration: 15 * time.Minute,

		LimitReached: func(c *fiber.Ctx) error {
			retry := c.GetRespHeader("Retry-After")

			return c.Status(429).JSON(fiber.Map{
				"error":       "Terlalu banyak request",
				"retry_after": retry, // dalam detik
				"message":     "Coba lagi dalam " + retry + " detik",
			})
		},
	})

	// Public routes
	app.Post("/login", loginLimiter, authHandler.Login)
	app.Post("/refresh", authHandler.Refresh)
	app.Post("/logout", authHandler.Logout)

	// Protected routes
	api := app.Group("/", auth.AuthMiddleware)

	api.Post("/admin/create-user", auth.AdminOnly, authHandler.CreateUser)

	// ── Employee (karyawan sendiri) ───────────────────
	// GET  /employee/profile         → profil sendiri
	// PATCH /employee/change-password → ubah password
	api.Get("/employee/profile", employeeHandler.GetProfile)
	api.Patch("/employee/change-password", employeeHandler.ChangePassword)

	// Attendance routes — ini yang mau ditest
	api.Post("/attendance/checkin", attendanceHandler.CheckIn)
	api.Patch("/attendance/checkout", attendanceHandler.CheckOut)
	api.Get("/attendance/today", attendanceHandler.GetToday)
	api.Get("/attendance/history", attendanceHandler.GetHistory)

	// Employee routes — comment dulu
	// api.Get("/employee/profile",           employeeHandler.GetProfile)
	// api.Patch("/employee/change-password", employeeHandler.ChangePassword)

	// QR routes — comment dulu
	// api.Get("/qr/today", auth.AdminOnly, qrHandler.GetTodayToken)

	log.Println("Server running on http://localhost:3000")
	log.Fatal(app.Listen(":3000"))
}
