package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type EmployeeHandler struct {
	empRepo *repository.EmployeeRepo
}

func NewEmployeeHandler(er *repository.EmployeeRepo) *EmployeeHandler {
	return &EmployeeHandler{empRepo: er}
}

// GET /admin-cabang/employees
func (h *EmployeeHandler) List(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
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
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
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
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	var req models.CreateEmployeeRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	// Validasi field wajib
	if strings.TrimSpace(req.Username) == "" {
		return utils.BadRequest(c, "USERNAME_REQUIRED", "Username wajib diisi")
	}
	if strings.TrimSpace(req.Name) == "" {
		return utils.BadRequest(c, "NAME_REQUIRED", "Nama wajib diisi")
	}
	if len(req.Password) < 6 {
		return utils.BadRequest(c, "PASSWORD_TOO_SHORT", "Password minimal 6 karakter")
	}

	// Default role dan tipe
	validRoles := map[string]bool{"karyawan": true, "admin_cabang": true, "admin": true}
	if !validRoles[req.Role] {
		req.Role = "karyawan"
	}
	if req.Tipe == "" {
		req.Tipe = "cabang"
	}

	// Cek username duplikat
	exists, err := h.empRepo.UsernameExists(req.Username)
	if err != nil {
		return utils.InternalError(c, "Gagal memeriksa username")
	}
	if exists {
		return utils.BadRequest(c, "USERNAME_TAKEN", "Username sudah digunakan")
	}

	// Fix: pakai bcrypt bukan SHA256
	// Harus sama dengan yang dipakai BE1 saat login
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return utils.InternalError(c, "Gagal memproses password")
	}
	hashedPassword := string(hashedBytes)

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
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
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
func (h *EmployeeHandler) Delete(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
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

	if err := h.empRepo.Delete(id, *claims.BranchID); err != nil {
		return utils.InternalError(c, "Gagal menghapus karyawan")
	}

	return utils.SuccessMessage(c, "Karyawan berhasil dihapus")
}

// Deactivate — alias untuk Delete
func (h *EmployeeHandler) Deactivate(c *fiber.Ctx) error {
	return h.Delete(c)
}
