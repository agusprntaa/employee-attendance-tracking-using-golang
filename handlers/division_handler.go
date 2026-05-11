package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type DivisionHandler struct {
	divRepo        *repository.DivisionRepo
	empRepo        *repository.EmployeeRepo
	attendanceRepo *repository.AttendanceRepo
}

func NewDivisionHandler(dr *repository.DivisionRepo, er *repository.EmployeeRepo, ar *repository.AttendanceRepo) *DivisionHandler {
	return &DivisionHandler{divRepo: dr, empRepo: er, attendanceRepo: ar}
}

// GET /admin-cabang/schedules
func (h *DivisionHandler) GetSchedules(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	if startDate == "" || endDate == "" {
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := now.AddDate(0, 0, -(weekday - 1))
		sunday := monday.AddDate(0, 0, 6)
		startDate = monday.Format("2006-01-02")
		endDate = sunday.Format("2006-01-02")
	}

	schedules, err := h.attendanceRepo.WeeklySchedule(*claims.BranchID, startDate, endDate)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data jadwal")
	}

	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	weekLabel := fmt.Sprintf("%s %d - %d, %d",
		start.Format("January"),
		start.Day(),
		end.Day(),
		start.Year(),
	)

	return utils.Success(c, fiber.Map{
		"week":       weekLabel,
		"start_date": startDate,
		"end_date":   endDate,
		"schedules":  schedules,
	})
}

// GET /admin-cabang/divisions
func (h *DivisionHandler) List(c *fiber.Ctx) error {
	divisions, err := h.divRepo.List()
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data divisi")
	}
	return utils.Success(c, divisions)
}

// GET /admin-cabang/divisions/:id
func (h *DivisionHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID tidak valid")
	}
	div, err := h.divRepo.GetByID(id)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data divisi")
	}
	if div == nil {
		return utils.NotFound(c, "Divisi tidak ditemukan")
	}
	return utils.Success(c, div)
}

// POST /admin-cabang/divisions
func (h *DivisionHandler) Create(c *fiber.Ctx) error {
	var req models.CreateDivisionRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}
	if strings.TrimSpace(req.Name) == "" {
		return utils.BadRequest(c, "NAME_REQUIRED", "Nama divisi wajib diisi")
	}
	if req.WorkStart == "" {
		req.WorkStart = "08:00:00"
	}
	if req.WorkEnd == "" {
		req.WorkEnd = "17:00:00"
	}
	if req.WorkDays == "" {
		req.WorkDays = "1,2,3,4,5"
	}
	if req.LateToleanceMin == 0 {
		req.LateToleanceMin = 15
	}
	if req.CheckinCutoffMin == 0 {
		req.CheckinCutoffMin = 120
	}
	id, err := h.divRepo.Create(&req)
	if err != nil {
		return utils.InternalError(c, "Gagal membuat divisi")
	}
	return utils.Created(c, fiber.Map{
		"id":      id,
		"message": "Divisi berhasil dibuat",
	})
}

// PATCH /admin-cabang/divisions/:id
func (h *DivisionHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID tidak valid")
	}
	var req models.CreateDivisionRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}
	div, err := h.divRepo.GetByID(id)
	if err != nil || div == nil {
		return utils.NotFound(c, "Divisi tidak ditemukan")
	}
	if err := h.divRepo.Update(id, &req); err != nil {
		return utils.InternalError(c, "Gagal update divisi")
	}
	return utils.SuccessMessage(c, "Divisi berhasil diupdate")
}

// DELETE /admin-cabang/divisions/:id
func (h *DivisionHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID tidak valid")
	}
	if err := h.divRepo.Delete(id); err != nil {
		return utils.InternalError(c, "Gagal hapus divisi")
	}
	return utils.SuccessMessage(c, "Divisi berhasil dihapus")
}
