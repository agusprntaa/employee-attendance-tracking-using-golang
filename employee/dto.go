package employee

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
	Name       string `json:"name"`
	Role       string `json:"role"` // "karyawan" / "admin_cabang" / "super_admin"
	Tipe       string `json:"tipe"` // "pusat" / "cabang"
	BranchID   int    `json:"branch_id"`
	DivisionID int    `json:"division_id"`
}

type UpdateEmployeeRequest struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	Tipe       string `json:"tipe"`
	BranchID   int    `json:"branch_id"`
	DivisionID int    `json:"division_id"`
}

// (Optional - future)
// Request update profile
type UpdateProfileRequest struct {
	Name     string `json:"name"`
	Username string `json:"username"`
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
	DivisionName string `json:"division_name"`
	BranchName   string `json:"branch_name"`
}
