package handlers

// File: handlers/leave_handler.go

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type LeaveHandler struct {
	leaveRepo *repository.LeaveRepo
}

func NewLeaveHandler(lr *repository.LeaveRepo) *LeaveHandler {
	return &LeaveHandler{leaveRepo: lr}
}

// ─── SUMMARY (kartu statistik atas) ──────────────────────────────────────────
// GET /admin-cabang/leave/summary

func (h *LeaveHandler) GetSummary(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	data, err := h.leaveRepo.GetSummaryByBranch(*claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil ringkasan cuti")
	}

	return utils.Success(c, data)
}

// ─── GET ALL LEAVE REQUESTS ───────────────────────────────────────────────────
// GET /admin-cabang/leave/requests
// Query params opsional: status (pending|approved|rejected), page, limit

func (h *LeaveHandler) GetAllRequests(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	page, limit := utils.ParsePage(c)
	status := c.Query("status")

	if status != "" && status != "pending" && status != "approved" && status != "rejected" {
		return utils.BadRequest(c, "INVALID_STATUS", "Status harus pending, approved, atau rejected")
	}

	data, total, err := h.leaveRepo.GetAllRequestsByBranch(*claims.BranchID, page, limit, status)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data pengajuan cuti")
	}

	return utils.Success(c, fiber.Map{
		"data": data,
		"pagination": models.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// ─── APPROVE / REJECT LEAVE ───────────────────────────────────────────────────
// PATCH /admin-cabang/leave/:id/status
// Body: { "status": "approved" | "rejected", "note": "opsional" }

func (h *LeaveHandler) UpdateLeaveStatus(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID pengajuan tidak valid")
	}

	var req models.UpdateLeaveStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.Status != "approved" && req.Status != "rejected" {
		return utils.BadRequest(c, "INVALID_STATUS", "Status harus 'approved' atau 'rejected'")
	}

	existing, err := h.leaveRepo.GetRequestByID(id, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data pengajuan cuti")
	}
	if existing == nil {
		return utils.NotFound(c, "Pengajuan cuti tidak ditemukan")
	}

	if existing["status"] != "pending" {
		return utils.BadRequest(c, "ALREADY_PROCESSED", "Pengajuan ini sudah diproses sebelumnya")
	}

	if err := h.leaveRepo.UpdateLeaveStatus(id, req.Status, req.Note); err != nil {
		return utils.InternalError(c, "Gagal mengupdate status pengajuan cuti")
	}

	pesan := "Pengajuan cuti berhasil disetujui"
	if req.Status == "rejected" {
		pesan = "Pengajuan cuti berhasil ditolak"
	}
	return utils.SuccessMessage(c, pesan)
}

// ─── LIHAT KUOTA KARYAWAN ─────────────────────────────────────────────────────
// GET /admin-cabang/leave/quota/:employee_id
// Query param opsional: year (default tahun ini)

func (h *LeaveHandler) GetLeaveQuota(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	employeeID, err := strconv.Atoi(c.Params("employee_id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	year := c.QueryInt("year", time.Now().Year())

	quota, err := h.leaveRepo.GetQuotaByEmployee(employeeID, year)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil kuota cuti")
	}
	if quota == nil {
		return utils.NotFound(c, "Data kuota cuti tidak ditemukan untuk karyawan ini")
	}

	return utils.Success(c, quota)
}

// ─── SET KUOTA MANUAL ─────────────────────────────────────────────────────────
// PATCH /admin-cabang/leave/quota/:employee_id
// Body: { "total": 12 }
// Query param opsional: year (default tahun ini)

func (h *LeaveHandler) UpdateLeaveQuota(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	employeeID, err := strconv.Atoi(c.Params("employee_id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	var req models.UpdateLeaveQuotaRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	if req.Total < 0 {
		return utils.BadRequest(c, "INVALID_TOTAL", "Total kuota tidak boleh negatif")
	}

	year := c.QueryInt("year", time.Now().Year())

	if err := h.leaveRepo.UpsertQuota(employeeID, year, req.Total); err != nil {
		return utils.InternalError(c, "Gagal mengupdate kuota cuti")
	}

	return utils.Success(c, fiber.Map{
		"employee_id": employeeID,
		"year":        year,
		"total":       req.Total,
		"pesan":       "Kuota cuti berhasil diupdate",
	})
}

// ─── LIHAT SEMUA HARI LIBUR ───────────────────────────────────────────────────
// GET /admin-cabang/holidays
// Query param opsional: year

func (h *LeaveHandler) GetAllHolidays(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	year := c.QueryInt("year", 0)

	data, err := h.leaveRepo.GetAllHolidays(year)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data hari libur")
	}

	return utils.Success(c, data)
}

// ─── TAMBAH HARI LIBUR ────────────────────────────────────────────────────────
// POST /admin-cabang/holidays
// Body: { "name": "Hari Kemerdekaan", "date": "2026-08-17", "description": "opsional" }

// ─── TAMBAHAN HARI LIBUR ────────────────────────────────────────────────────────
// POST /admin-cabang/holidays

func (h *LeaveHandler) CreateHoliday(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	// Sesuai dengan UI Anda, struct ini hanya menangkap 3 data dari FE
	var req models.CreateHolidayRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	req.Date = strings.TrimSpace(req.Date)
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	if req.Date == "" {
		return utils.BadRequest(c, "DATE_REQUIRED", "Tanggal wajib diisi")
	}
	if req.Name == "" {
		return utils.BadRequest(c, "NAME_REQUIRED", "Nama hari libur wajib diisi")
	}
	if _, err := time.Parse("2006-01-02", req.Date); err != nil {
		return utils.BadRequest(c, "INVALID_DATE", "Format tanggal harus YYYY-MM-DD, contoh: 2026-08-17")
	}

	// STRATEGI BACKEND: Karena UI tidak mengirim kategori, kita kunci/set otomatis di sini!
	categoryOtomatis := "khusus"

	// Oper variabel 'categoryOtomatis' ke repository
	id, err := h.leaveRepo.CreateHoliday(req.Date, req.Name, req.Description, categoryOtomatis)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return utils.BadRequest(c, "DATE_DUPLICATE", "Tanggal ini sudah terdaftar sebagai hari libur")
		}
		return utils.InternalError(c, "Gagal menambahkan hari libur")
	}

	return utils.Created(c, fiber.Map{
		"id":          id,
		"date":        req.Date,
		"name":        req.Name,
		"description": req.Description,
		"category":    categoryOtomatis, // Beritahu FE kalau ini sukses masuk sebagai kategori khusus
	})
}

// ─── HAPUS HARI LIBUR ─────────────────────────────────────────────────────────
// DELETE /admin-cabang/holidays/:id

func (h *LeaveHandler) DeleteHoliday(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID hari libur tidak valid")
	}

	deleted, err := h.leaveRepo.DeleteHoliday(id)
	if err != nil {
		return utils.InternalError(c, "Gagal menghapus hari libur")
	}
	if !deleted {
		return utils.NotFound(c, "Hari libur tidak ditemukan")
	}

	return utils.SuccessMessage(c, "Hari libur berhasil dihapus")
}

// ─── KALENDER — TITIK ─────────────────────────────────────────────────────────
// GET /admin-cabang/leave/calendar?month=5&year=2026

func (h *LeaveHandler) GetCalendarDots(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	month := c.QueryInt("month", int(time.Now().Month()))
	year := c.QueryInt("year", time.Now().Year())

	if month < 1 || month > 12 {
		return utils.BadRequest(c, "INVALID_MONTH", "Bulan harus antara 1 sampai 12")
	}

	data, err := h.leaveRepo.GetCalendarDots(*claims.BranchID, month, year)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data kalender")
	}

	return utils.Success(c, fiber.Map{
		"month": month,
		"year":  year,
		"dates": data,
	})
}

// ─── KALENDER — DETAIL TANGGAL ────────────────────────────────────────────────
// GET /admin-cabang/leave/calendar/detail?date=2026-05-27

func (h *LeaveHandler) GetCalendarDetail(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	date := strings.TrimSpace(c.Query("date"))
	if date == "" {
		return utils.BadRequest(c, "DATE_REQUIRED", "Parameter date wajib diisi, contoh: ?date=2026-05-27")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return utils.BadRequest(c, "INVALID_DATE", "Format tanggal harus YYYY-MM-DD, contoh: 2026-05-27")
	}

	data, err := h.leaveRepo.GetCalendarDetail(*claims.BranchID, date)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil detail tanggal")
	}

	return utils.Success(c, data)
}

// ─── DETAIL PENGAJUAN CUTI BY ID ─────────────────────────────────────────────
// GET /admin-cabang/leave/requests/:id
// Dipakai FE untuk tampilkan modal "Detail Pengajuan Cuti"
 
func (h *LeaveHandler) GetRequestByID(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}
 
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID pengajuan tidak valid")
	}
 
	data, err := h.leaveRepo.GetRequestByID(id, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil detail pengajuan cuti")
	}
	if data == nil {
		return utils.NotFound(c, "Pengajuan cuti tidak ditemukan")
	}
 
	return utils.Success(c, data)
}
// GetRecentActivity — aktivitas terbaru seputar cuti
// GET /admin-cabang/leave/recent-activity
// Query param opsional: limit (default 10)
// Dipakai FE untuk tampilkan panel "Recent Activity" di dashboard cuti
 
func (h *LeaveHandler) GetRecentActivity(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}
 
	// Default 6 aktivitas terbaru, maksimal 6
	limit := c.QueryInt("limit", 6)
	if limit < 1 || limit > 6 {
    limit = 6
}
 
	data, err := h.leaveRepo.GetRecentActivity(*claims.BranchID, limit)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil aktivitas terbaru")
	}
 
	return utils.Success(c, data)
}