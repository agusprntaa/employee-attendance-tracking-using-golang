package handlers

import (
	"absensi/middleware"
	"absensi/models"
	"absensi/repository"
	"absensi/utils"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type EmployeeHandler struct {
	empRepo *repository.EmployeeRepo
}

func NewEmployeeHandler(er *repository.EmployeeRepo) *EmployeeHandler {
	return &EmployeeHandler{empRepo: er}
}

// GET /admin-cabang/employees?page=1&limit=10&search=budi&status=active
func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	page, limit := utils.ParsePage(r)
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")

	employees, total, err := h.empRepo.ListByBranch(*claims.BranchID, page, limit, search, status)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data karyawan")
		return
	}

	utils.Success(w, map[string]interface{}{
		"data": employees,
		"pagination": models.Pagination{
			Page:  page,
			Limit: limit,
			Total: total,
		},
	})
}

// GET /admin-cabang/employees/{id}
func (h *EmployeeHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	id, err := parseIDFromPath(r.URL.Path)
	if err != nil {
		utils.BadRequest(w, "INVALID_ID", "ID karyawan tidak valid")
		return
	}

	emp, err := h.empRepo.GetByID(id, *claims.BranchID)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data karyawan")
		return
	}
	if emp == nil {
		utils.NotFound(w, "Karyawan tidak ditemukan di cabang ini")
		return
	}
	utils.Success(w, emp)
}

// POST /admin-cabang/employees
// Body: { "username", "password", "role", "tipe", "division_id" }
// CATATAN: tidak ada field "name" karena tidak ada di database
func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	var req models.CreateEmployeeRequest
	if err := utils.ParseBody(r, &req); err != nil {
		utils.BadRequest(w, "INVALID_BODY", "Request body tidak valid")
		return
	}

	// Validasi
	if strings.TrimSpace(req.Username) == "" {
		utils.BadRequest(w, "USERNAME_REQUIRED", "Username wajib diisi")
		return
	}
	if len(req.Password) < 6 {
		utils.BadRequest(w, "PASSWORD_TOO_SHORT", "Password minimal 6 karakter")
		return
	}

	// Default values
	validRoles := map[string]bool{"karyawan": true, "admin_cabang": true, "admin": true}
	if !validRoles[req.Role] {
		req.Role = "karyawan"
	}
	if req.Tipe == "" {
		req.Tipe = "cabang"
	}

	// Cek username sudah dipakai
	exists, err := h.empRepo.UsernameExists(req.Username)
	if err != nil {
		utils.InternalError(w, "Gagal memeriksa username")
		return
	}
	if exists {
		utils.BadRequest(w, "USERNAME_TAKEN", "Username sudah digunakan")
		return
	}

	// Hash password — SHA-256 sederhana
	// ⚠️ PENTING: Sesuaikan dengan method hashing Backend 1
	hashedPassword := hashPassword(req.Password)

	id, err := h.empRepo.Create(&req, hashedPassword, *claims.BranchID)
	if err != nil {
		utils.InternalError(w, "Gagal membuat akun karyawan")
		return
	}

	utils.Created(w, map[string]interface{}{
		"id":      id,
		"message": "Karyawan berhasil ditambahkan",
	})
}

// PATCH /admin-cabang/employees/{id}
// Body: { "role", "status", "division_id" }
func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	id, err := parseIDFromPath(r.URL.Path)
	if err != nil {
		utils.BadRequest(w, "INVALID_ID", "ID karyawan tidak valid")
		return
	}

	var req models.UpdateEmployeeRequest
	if err := utils.ParseBody(r, &req); err != nil {
		utils.BadRequest(w, "INVALID_BODY", "Request body tidak valid")
		return
	}

	// Validasi status
	validStatus := map[string]bool{"active": true, "inactive": true}
	if req.Status != "" && !validStatus[req.Status] {
		utils.BadRequest(w, "INVALID_STATUS", "Status harus 'active' atau 'inactive'")
		return
	}

	emp, err := h.empRepo.GetByID(id, *claims.BranchID)
	if err != nil || emp == nil {
		utils.NotFound(w, "Karyawan tidak ditemukan di cabang ini")
		return
	}

	if err := h.empRepo.Update(id, *claims.BranchID, &req); err != nil {
		utils.InternalError(w, "Gagal mengupdate data karyawan")
		return
	}

	utils.SuccessMessage(w, "Data karyawan berhasil diupdate")
}

// DELETE /admin-cabang/employees/{id} → set status = inactive
func (h *EmployeeHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
		return
	}

	id, err := parseIDFromPath(r.URL.Path)
	if err != nil {
		utils.BadRequest(w, "INVALID_ID", "ID karyawan tidak valid")
		return
	}

	emp, err := h.empRepo.GetByID(id, *claims.BranchID)
	if err != nil || emp == nil {
		utils.NotFound(w, "Karyawan tidak ditemukan di cabang ini")
		return
	}

	if err := h.empRepo.Deactivate(id, *claims.BranchID); err != nil {
		utils.InternalError(w, "Gagal menonaktifkan karyawan")
		return
	}

	utils.SuccessMessage(w, "Karyawan berhasil dinonaktifkan")
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// hashPassword - SHA-256 sederhana
// ⚠️ WAJIB disesuaikan dengan cara Backend 1 menyimpan password
// Jika Backend 1 pakai bcrypt: ganti dengan bcrypt.GenerateFromPassword
func hashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte(password))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func parseIDFromPath(path string) (int, error) {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) == 0 {
		return 0, fmt.Errorf("no id in path")
	}
	return strconv.Atoi(parts[len(parts)-1])
}
