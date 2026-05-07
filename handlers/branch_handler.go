package handlers

import (
	"absensi/middleware"
	"absensi/models"
	"absensi/repository"
	"absensi/utils"
	"net/http"
)

type BranchHandler struct {
	branchRepo *repository.BranchRepo
}

func NewBranchHandler(br *repository.BranchRepo) *BranchHandler {
	return &BranchHandler{branchRepo: br}
}

// GET /admin-cabang/branch
func (h *BranchHandler) GetBranch(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang")
		return
	}
	branch, err := h.branchRepo.GetByID(*claims.BranchID)
	if err != nil {
		utils.InternalError(w, "Gagal mengambil data cabang")
		return
	}
	if branch == nil {
		utils.NotFound(w, "Cabang tidak ditemukan")
		return
	}
	utils.Success(w, branch)
}

// PATCH /admin-cabang/branch
func (h *BranchHandler) UpdateBranch(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims.BranchID == nil {
		utils.BadRequest(w, "NO_BRANCH", "Admin tidak memiliki cabang")
		return
	}
	var req models.UpdateBranchRequest
	if err := utils.ParseBody(r, &req); err != nil {
		utils.BadRequest(w, "INVALID_BODY", "Request body tidak valid")
		return
	}
	if req.Name == "" {
		utils.BadRequest(w, "NAME_REQUIRED", "Nama cabang wajib diisi")
		return
	}
	if req.RadiusMeter <= 0 {
		req.RadiusMeter = 100
	}
	if err := h.branchRepo.Update(*claims.BranchID, &req); err != nil {
		utils.InternalError(w, "Gagal update data cabang")
		return
	}
	utils.SuccessMessage(w, "Data cabang berhasil diupdate")
}
