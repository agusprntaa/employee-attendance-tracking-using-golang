package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"log"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type EmployeeHandler struct {
	empRepo  *repository.EmployeeRepo
	AuthRepo *auth.Repository
}

func NewEmployeeHandler(er *repository.EmployeeRepo, ar *auth.Repository) *EmployeeHandler {
	return &EmployeeHandler{empRepo: er, AuthRepo: ar}
}

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

func (h *EmployeeHandler) Create(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	var req models.CreateEmployeeRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	log.Printf("=== CREATE EMPLOYEE ===")
	log.Printf("Username: %s, Name: %s, Role: %s, DivisionID: %v",
		req.Username, req.Name, req.Role, req.DivisionID)

	if strings.TrimSpace(req.Username) == "" {
		return utils.BadRequest(c, "USERNAME_REQUIRED", "Username wajib diisi")
	}
	if strings.TrimSpace(req.Name) == "" {
		return utils.BadRequest(c, "NAME_REQUIRED", "Nama wajib diisi")
	}
	// generate password acak — tidak perlu dari request
	plainPassword := utils.GenerateRandomPassword()

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

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(plainPassword), 12)
	if err != nil {
		return utils.InternalError(c, "Gagal memproses password")
	}

	id, err := h.empRepo.Create(&req, string(hashedBytes), *claims.BranchID)
	if err != nil {
		log.Printf("CREATE ERROR: %v", err)
		return utils.InternalError(c, "Gagal membuat akun karyawan")
	}

	return utils.Created(c, fiber.Map{
		"id":           id,
		"username":     req.Username,
		"temp_password": plainPassword,
		"message":      "Karyawan berhasil ditambahkan",
	})
}

// Update — fix: handle error dari repo dan log request body
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

	// Log untuk debug — lihat apa yang dikirim FE
	log.Printf("=== UPDATE EMPLOYEE ===")
	log.Printf("Employee ID : %d", id)
	log.Printf("Branch ID   : %d", *claims.BranchID)
	log.Printf("Name        : %s", req.Name)
	log.Printf("Role        : %s", req.Role)
	if req.DivisionID != nil {
		log.Printf("DivisionID: %d", *req.DivisionID)
	}

	// Cek karyawan ada di cabang ini
	emp, err := h.empRepo.GetByID(id, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data karyawan")
	}
	if emp == nil {
		log.Printf("UPDATE: karyawan %d tidak ditemukan di branch %d", id, *claims.BranchID)
		return utils.NotFound(c, "Karyawan tidak ditemukan di cabang ini")
	}

	// Kalau name kosong di request, pertahankan name yang lama
	if strings.TrimSpace(req.Name) == "" {
		req.Name = emp.Name
	}

	if strings.TrimSpace(req.Username) == "" {
		req.Username = emp.Username
	}

	if req.Status == "" {
		req.Status = emp.Status
	}

	if err := h.empRepo.Update(id, *claims.BranchID, &req); err != nil {
		log.Printf("UPDATE ERROR: %v", err)
		return utils.InternalError(c, "Gagal mengupdate data karyawan: "+err.Error())
	}

	return utils.SuccessMessage(c, "Data karyawan berhasil diupdate")
}

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

func (h *EmployeeHandler) Activate(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID tidak valid")
	}

	err = h.empRepo.Activate(id, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.SuccessMessage(c, "Karyawan berhasil diaktifkan")
}

func (h *EmployeeHandler) Deactivate(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID tidak valid")
	}

	err = h.empRepo.Deactivate(id, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, err.Error())
	}

	return utils.SuccessMessage(c, "Karyawan berhasil dinonaktifkan")
}
