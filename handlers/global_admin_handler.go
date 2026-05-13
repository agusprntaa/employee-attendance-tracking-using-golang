package handlers

import (
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type GlobalAdminHandler struct {
	Repo *repository.GlobalAdminRepository
}

func NewGlobalAdminHandler(repo *repository.GlobalAdminRepository) *GlobalAdminHandler {
	return &GlobalAdminHandler{Repo: repo}
}

// ============================================================
// GET /admin-pusat/dashboard
// ============================================================
func (h *GlobalAdminHandler) GetDashboard(c *fiber.Ctx) error {

	stats, err := h.Repo.GetDashboardStats()
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil statistik dashboard")
	}

	attendancePerBranch, _ := h.Repo.GetAttendancePerBranch()
	workMode, _ := h.Repo.GetWorkModeDistribution()
	branchPerformance, _ := h.Repo.GetBranchPerformance()

	if attendancePerBranch == nil {
		attendancePerBranch = []map[string]interface{}{}
	}

	if workMode == nil {
		workMode = map[string]interface{}{}
	}

	if branchPerformance == nil {
		branchPerformance = []map[string]interface{}{}
	}

	return utils.Success(c, fiber.Map{
		"stats":                 stats,
		"attendance_per_branch": attendancePerBranch,
		"work_mode":             workMode,
		"branch_performance":    branchPerformance,
	})
}

// ============================================================
// GET /admin-pusat/employees
// ============================================================
func (h *GlobalAdminHandler) GetAllEmployees(c *fiber.Ctx) error {

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	search := c.Query("search")
	branch := c.Query("branch")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}

	if limit < 1 || limit > 100 {
		limit = 10
	}

	data, total, err := h.Repo.GetAllEmployees(
		page,
		limit,
		search,
		branch,
		status,
	)

	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data karyawan")
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return utils.Success(c, fiber.Map{
		"data":         data,
		"total_data":   total,
		"current_page": page,
		"total_pages":  totalPages,
		"limit":        limit,
	})
}

// ============================================================
// GET /admin-pusat/employees/:id
// ============================================================
func (h *GlobalAdminHandler) GetEmployeeDetail(c *fiber.Ctx) error {

	employeeID, err := strconv.Atoi(c.Params("id"))
	if err != nil || employeeID <= 0 {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	data, err := h.Repo.GetEmployeeDetail(employeeID)
	if err != nil {

		if err.Error() == "karyawan tidak ditemukan" {
			return utils.NotFound(c, "Karyawan tidak ditemukan")
		}

		return utils.InternalError(c, "Gagal mengambil detail karyawan")
	}

	return utils.Success(c, data)
}

// ============================================================
// GET /admin-pusat/attendance/today
// ============================================================
func (h *GlobalAdminHandler) GetTodayAttendance(c *fiber.Ctx) error {

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)
	search := c.Query("search")

	if page < 1 {
		page = 1
	}

	if limit < 1 || limit > 100 {
		limit = 10
	}

	result, err := h.Repo.GetTodayAttendanceOverview(
		page,
		limit,
		search,
	)

	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data kehadiran hari ini")
	}

	return utils.Success(c, result)
}

// ============================================================
// GET /admin-pusat/attendance/analytics
// ============================================================
func (h *GlobalAdminHandler) GetAttendanceAnalytics(c *fiber.Ctx) error {

	period := c.Query("period", "weekly")

	if period != "weekly" && period != "monthly" {
		period = "weekly"
	}

	result, err := h.Repo.GetAttendanceAnalytics(period)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data analitik kehadiran")
	}

	return utils.Success(c, result)
}

// ============================================================
// GET /admin-pusat/branches
// ============================================================
func (h *GlobalAdminHandler) GetAllBranches(c *fiber.Ctx) error {

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	search := c.Query("search")
	status := c.Query("status")

	if page < 1 {
		page = 1
	}

	if limit < 1 || limit > 100 {
		limit = 10
	}

	data, total, err := h.Repo.GetAllBranches(
		page,
		limit,
		search,
		status,
	)

	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data cabang")
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	return utils.Success(c, fiber.Map{
		"data":         data,
		"total_data":   total,
		"current_page": page,
		"total_pages":  totalPages,
		"limit":        limit,
	})
}

// ============================================================
// GET /admin-pusat/branches/:id
// ============================================================
func (h *GlobalAdminHandler) GetBranchDetail(c *fiber.Ctx) error {

	branchID, err := strconv.Atoi(c.Params("id"))
	if err != nil || branchID <= 0 {
		return utils.BadRequest(c, "INVALID_ID", "ID cabang tidak valid")
	}

	data, err := h.Repo.GetBranchDetail(branchID)
	if err != nil {

		if err.Error() == "cabang tidak ditemukan" {
			return utils.NotFound(c, "Cabang tidak ditemukan")
		}

		return utils.InternalError(c, "Gagal mengambil detail cabang")
	}

	return utils.Success(c, data)
}

// ============================================================
// POST /admin-pusat/branches
// ============================================================
func (h *GlobalAdminHandler) CreateBranch(c *fiber.Ctx) error {

	var req struct {
		BranchName  string  `json:"branch_name"`
		Address     string  `json:"address"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		RadiusMeter int     `json:"radius_meter"`
		Status      string  `json:"status"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Format request tidak valid")
	}

	if req.BranchName == "" {
		return utils.BadRequest(
			c,
			"VALIDATION_ERROR",
			"Nama cabang wajib diisi",
		)
	}

	req.Status = strings.ToLower(req.Status)

	if req.Status != "active" && req.Status != "inactive" {
		return utils.BadRequest(
			c,
			"VALIDATION_ERROR",
			"Status harus active atau inactive",
		)
	}

	if req.RadiusMeter <= 0 {
		req.RadiusMeter = 100
	}

	id, err := h.Repo.CreateBranch(
		req.BranchName,
		req.Address,
		req.Latitude,
		req.Longitude,
		req.RadiusMeter,
		req.Status,
	)

	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.Success(c, fiber.Map{
		"message": "Cabang berhasil dibuat",
		"data": fiber.Map{
			"id":          id,
			"branch_id":   fmt.Sprintf("BR%03d", id),
			"branch_name": req.BranchName,
		},
	})
}

// ============================================================
// PUT /admin-pusat/branches/:id
// ============================================================
func (h *GlobalAdminHandler) UpdateBranch(c *fiber.Ctx) error {

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID cabang tidak valid")
	}

	var req struct {
		BranchName  string  `json:"branch_name"`
		Address     string  `json:"address"`
		Latitude    float64 `json:"latitude"`
		Longitude   float64 `json:"longitude"`
		RadiusMeter int     `json:"radius_meter"`
		Status      string  `json:"status"`
	}

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Format request tidak valid")
	}

	err = h.Repo.UpdateBranch(
		id,
		req.BranchName,
		req.Address,
		req.Latitude,
		req.Longitude,
		req.RadiusMeter,
		req.Status,
	)

	if err != nil {
		return utils.InternalError(c, "Gagal update cabang")
	}

	return utils.Success(c, fiber.Map{
		"message": "Branch berhasil diupdate",
	})
}

// ============================================================
// DELETE /admin-pusat/branches/:id
// ============================================================
func (h *GlobalAdminHandler) DeleteBranch(c *fiber.Ctx) error {

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID cabang tidak valid")
	}

	err = h.Repo.DeleteBranch(id)
	if err != nil {
		return utils.InternalError(c, "Gagal menghapus cabang")
	}

	return utils.Success(c, fiber.Map{
		"message": "Branch berhasil dihapus",
	})
}
