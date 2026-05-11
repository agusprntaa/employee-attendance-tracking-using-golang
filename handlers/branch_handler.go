package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"

	"github.com/gofiber/fiber/v2"
)

type BranchHandler struct {
	branchRepo *repository.BranchRepo
}

func NewBranchHandler(br *repository.BranchRepo) *BranchHandler {
	return &BranchHandler{branchRepo: br}
}

// GET /admin-cabang/branch
func (h *BranchHandler) GetBranch(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang")
	}
	branch, err := h.branchRepo.GetByID(*claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data cabang")
	}
	if branch == nil {
		return utils.NotFound(c, "Cabang tidak ditemukan")
	}
	return utils.Success(c, branch)
}

// PATCH /admin-cabang/branch
func (h *BranchHandler) UpdateBranch(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang")
	}
	var req models.UpdateBranchRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}
	if req.Name == "" {
		return utils.BadRequest(c, "NAME_REQUIRED", "Nama cabang wajib diisi")
	}
	if req.RadiusMeter <= 0 {
		req.RadiusMeter = 100
	}
	if err := h.branchRepo.Update(*claims.BranchID, &req); err != nil {
		return utils.InternalError(c, "Gagal update data cabang")
	}
	return utils.SuccessMessage(c, "Data cabang berhasil diupdate")
}
