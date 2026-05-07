package handlers

import (
	"absensi/middleware"
	"absensi/models"
	"absensi/repository"
	"absensi/utils"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type DivisionHandler struct {
	divRepo        *repository.DivisionRepo
	empRepo        *repository.EmployeeRepo
	attendanceRepo *repository.AttendanceRepo
}

func NewDivisionHandler(dr *repository.DivisionRepo, er *repository.EmployeeRepo, ar *repository.AttendanceRepo) *DivisionHandler {
	return &DivisionHandler{divRepo: dr, empRepo: er, attendanceRepo: ar}
}

// GET /admin-cabang/schedules?start_date=2026-04-21&end_date=2026-04-27
// Sesuai UI: tabel per karyawan per hari (Senin - Minggu), tanpa shift
func (h *DivisionHandler) GetSchedules(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	// Ambil parameter minggu, default minggu ini
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if startDate == "" || endDate == "" {
		// Default: minggu ini (Senin - Minggu)
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
		utils.InternalError(w, "Gagal mengambil data jadwal")
		return
	}

	// Format label minggu untuk UI: "April 21 - 27, 2026"
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	weekLabel := fmt.Sprintf("%s %d - %d, %d",
		start.Format("January"),
		start.Day(),
		end.Day(),
		start.Year(),
	)

	utils.Success(w, map[string]interface{}{
		"week":       weekLabel,
		"start_date": startDate,
		"end_date":   endDate,
		"schedules":  schedules,
	})
}

// GET /admin-cabang/divisions
func (h *DivisionHandler) List(w http.ResponseWriter, r *http.Request) {
	divisions, err := h.divRepo.List()
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data divisi")
		return
	}
	utils.Success(w, divisions)
}

// GET /admin-cabang/divisions/{id}
func (h *DivisionHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := parseDivisionID(r.URL.Path)
	if err != nil {
		utils.BadRequest(w, "INVALID_ID", "ID tidak valid")
		return
	}
	div, err := h.divRepo.GetByID(id)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data divisi")
		return
	}
	if div == nil {
		utils.NotFound(w, "Divisi tidak ditemukan")
		return
	}
	utils.Success(w, div)
}

// POST /admin-cabang/divisions
func (h *DivisionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateDivisionRequest
	if err := utils.ParseBody(r, &req); err != nil {
		utils.BadRequest(w, "INVALID_BODY", "Request body tidak valid")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		utils.BadRequest(w, "NAME_REQUIRED", "Nama divisi wajib diisi")
		return
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
		utils.InternalError(w, "Gagal membuat divisi")
		return
	}
	utils.Created(w, map[string]interface{}{
		"id":      id,
		"message": "Divisi berhasil dibuat",
	})
}

// PATCH /admin-cabang/divisions/{id}
func (h *DivisionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseDivisionID(r.URL.Path)
	if err != nil {
		utils.BadRequest(w, "INVALID_ID", "ID tidak valid")
		return
	}
	var req models.CreateDivisionRequest
	if err := utils.ParseBody(r, &req); err != nil {
		utils.BadRequest(w, "INVALID_BODY", "Request body tidak valid")
		return
	}
	div, err := h.divRepo.GetByID(id)
	if err != nil || div == nil {
		utils.NotFound(w, "Divisi tidak ditemukan")
		return
	}
	if err := h.divRepo.Update(id, &req); err != nil {
		utils.InternalError(w, "Gagal update divisi")
		return
	}
	utils.SuccessMessage(w, "Divisi berhasil diupdate")
}

// DELETE /admin-cabang/divisions/{id}
func (h *DivisionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseDivisionID(r.URL.Path)
	if err != nil {
		utils.BadRequest(w, "INVALID_ID", "ID tidak valid")
		return
	}
	if err := h.divRepo.Delete(id); err != nil {
		utils.InternalError(w, "Gagal hapus divisi")
		return
	}
	utils.SuccessMessage(w, "Divisi berhasil dihapus")
}

func parseDivisionID(path string) (int, error) {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	return strconv.Atoi(parts[len(parts)-1])
}
