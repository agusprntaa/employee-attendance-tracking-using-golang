package employee

import "github.com/gofiber/fiber/v2"

// ─────────────────────────────────────────
// REQUEST DTO
// (data dari client ke backend)
// ─────────────────────────────────────────

// Request untuk ganti password
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type CreateEmployeeRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Name       string `json:"FullName"`
	Role       string `json:"role"` // "karyawan" / "admin_cabang" / "super_admin"
	Tipe       string `json:"tipe"` // "pusat" / "cabang"
	BranchID   int    `json:"branch_id"`
	DivisionID int    `json:"division_id"`
}

type UpdateEmployeeRequest struct {
	Name       string `json:"name"`
	Username   string `json:"username"`
	Role       string `json:"role"`
	Tipe       string `json:"tipe"`
	BranchID   int    `json:"branch_id"`
	DivisionID int    `json:"division_id"`
	Status     string `json:"status"`
}

// (Optional - future)
// Request update profile
type UpdateProfileRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
	BirthDate string `json:"birth_date"`
}

// ─────────────────────────────────────────
// RESPONSE DTO
// (data dari backend ke client)
// ─────────────────────────────────────────

// Response profil karyawan
type ProfileResponse struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	Tipe         string `json:"tipe"`
	Status       string `json:"status"`
	DivisionName string `json:"division_name"`
	BranchName   string `json:"branch_name"`
	PhotoURL     string `json:"photo_url"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	Address      string `json:"address"`
	BirthDate    string `json:"birth_date"`
}

// di employee/handler.go — tambah handler ini
func (h *Handler) GetOnboardingStatus(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	mustChange, faceRegistered, profileCompleted, err := h.Repo.GetOnboardingFlags(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil status onboarding",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"must_change_password": mustChange,
			"face_registered":      faceRegistered,
			"profile_completed":    profileCompleted,
		},
	})
}
