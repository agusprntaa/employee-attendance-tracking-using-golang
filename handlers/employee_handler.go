package handlers

import (
	"absensi/middleware"
	"absensi/models"
	"absensi/repository"
	"absensi/utils"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type EmployeeHandler struct {
	empRepo *repository.EmployeeRepo
}

func NewEmployeeHandler(er *repository.EmployeeRepo) *EmployeeHandler {
	return &EmployeeHandler{empRepo: er}
}

// GET /admin-cabang/employees
func (h *EmployeeHandler) List(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	page, limit := utils.ParsePage(c)
	search := c.Query("search")
	status := c.Query("status")

	employees, total, err := h.empRepo.ListByBranch(*claims.BranchID, page, limit, search, status)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data karyawan")
	}

	return utils.Success(c, fiber.Map{
		"data": employees,
		"pagination": models.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// GET /admin-cabang/employees/:id
func (h *EmployeeHandler) GetByID(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	emp, err := h.empRepo.GetByID(id, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data karyawan")
	}
	if emp == nil {
		return utils.NotFound(c, "Karyawan tidak ditemukan di cabang ini")
	}
	return utils.Success(c, emp)
}

// POST /admin-cabang/employees
func (h *EmployeeHandler) Create(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	var req models.CreateEmployeeRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	if strings.TrimSpace(req.Username) == "" {
		return utils.BadRequest(c, "USERNAME_REQUIRED", "Username wajib diisi")
	}
	if len(req.Password) < 6 {
		return utils.BadRequest(c, "PASSWORD_TOO_SHORT", "Password minimal 6 karakter")
	}

	validRoles := map[string]bool{"karyawan": true, "admin_cabang": true, "admin": true}
	if !validRoles[req.Role] {
		req.Role = "karyawan"
	}
	if req.Tipe == "" {
		req.Tipe = "cabang"
	}

	exists, err := h.empRepo.UsernameExists(req.Username)
	if err != nil {
		return utils.InternalError(c, "Gagal memeriksa username")
	}
	if exists {
		return utils.BadRequest(c, "USERNAME_TAKEN", "Username sudah digunakan")
	}

	hashedPassword := hashPassword(req.Password)

	id, err := h.empRepo.Create(&req, hashedPassword, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal membuat akun karyawan")
	}

	return utils.Created(c, fiber.Map{
		"id":      id,
		"message": "Karyawan berhasil ditambahkan",
	})
}

// PATCH /admin-cabang/employees/:id
func (h *EmployeeHandler) Update(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	var req models.UpdateEmployeeRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	validStatus := map[string]bool{"active": true, "inactive": true}
	if req.Status != "" && !validStatus[req.Status] {
		return utils.BadRequest(c, "INVALID_STATUS", "Status harus 'active' atau 'inactive'")
	}

	emp, err := h.empRepo.GetByID(id, *claims.BranchID)
	if err != nil || emp == nil {
		return utils.NotFound(c, "Karyawan tidak ditemukan di cabang ini")
	}

	if err := h.empRepo.Update(id, *claims.BranchID, &req); err != nil {
		return utils.InternalError(c, "Gagal mengupdate data karyawan")
	}

	return utils.SuccessMessage(c, "Data karyawan berhasil diupdate")
}

// DELETE /admin-cabang/employees/:id
func (h *EmployeeHandler) Deactivate(c *fiber.Ctx) error {
	claims := middleware.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	emp, err := h.empRepo.GetByID(id, *claims.BranchID)
	if err != nil || emp == nil {
		return utils.NotFound(c, "Karyawan tidak ditemukan di cabang ini")
	}

	if err := h.empRepo.Deactivate(id, *claims.BranchID); err != nil {
		return utils.InternalError(c, "Gagal menonaktifkan karyawan")
	}

	return utils.SuccessMessage(c, "Karyawan berhasil dinonaktifkan")
}

func hashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte(password))
	return fmt.Sprintf("%x", h.Sum(nil))
}
