package employee

import (
	"absensi_karyawan/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	Repo *Repository
}

// ─────────────────────────────────────────
// GET /employee/profile
// Data profil karyawan yang sedang login
// ─────────────────────────────────────────
func (h *Handler) GetProfile(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	profile, err := h.Repo.GetProfile(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil profil",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   profile,
	})
}

// ─────────────────────────────────────────
// PATCH /employee/profile
// Karyawan update nama, email, telepon, alamat, tanggal lahir
// ─────────────────────────────────────────

func (h *Handler) UpdateProfile(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Format request tidak valid",
		})
	}

	// Validasi nama wajib diisi
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Nama lengkap wajib diisi",
		})
	}

	// Validasi format email jika diisi
	if req.Email != "" && !strings.Contains(req.Email, "@") {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Format email tidak valid",
		})
	}

	// Validasi format tanggal lahir jika diisi
	if req.BirthDate != "" {
		if _, err := time.Parse("2006-01-02", req.BirthDate); err != nil {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"message": "Format tanggal lahir harus YYYY-MM-DD, contoh: 1995-08-17",
			})
		}
	}

	if err := h.Repo.UpdateProfile(employeeID, req); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menyimpan profil",
		})
	}

	// Ambil data terbaru setelah update
	profile, err := h.Repo.GetProfile(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Profil tersimpan tapi gagal memuat data terbaru",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Profil berhasil diperbarui",
		"data":    profile,
	})
}

// ─────────────────────────────────────────
// PATCH /employee/change-password
// Ubah password karyawan sendiri
// ─────────────────────────────────────────
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Format request tidak valid",
		})
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Password lama dan baru wajib diisi",
		})
	}

	if len(req.NewPassword) < 6 {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"message": "Password baru minimal 6 karakter",
		})
	}

	// Ambil password hash saat ini dari DB
	currentHash, err := h.Repo.GetPasswordHash(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal memverifikasi password",
		})
	}

	// Verifikasi password lama
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.OldPassword)); err != nil {
		return c.Status(401).JSON(fiber.Map{
			"status":  "error",
			"code":    "WRONG_OLD_PASSWORD",
			"message": "Password lama tidak sesuai",
		})
	}

	// Hash password baru
	newHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal memproses password baru",
		})
	}

	// Update ke DB
	if err := h.Repo.UpdatePassword(employeeID, newHash); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menyimpan password baru",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Password berhasil diubah",
	})
}

// ─────────────────────────────────────────
// F5: POST /employee/profile/photo
//
// Upload foto profil karyawan.
// Pakai multipart/form-data dengan field "photo".
//
// Validasi:
//   - Format: jpg, jpeg, png, webp
//   - Ukuran maksimal: 2MB
//
// File disimpan di: uploads/photos/{employeeID}_{timestamp}.{ext}
// Foto lama otomatis dihapus dari disk saat upload baru.
// ─────────────────────────────────────────

func (h *Handler) UploadPhoto(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	// Handle upload dengan middleware Fiber
	file, err := c.FormFile("photo")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "NO_FILE",
			"message": "File foto wajiib diunggah dengan field 'photo'",
		})
	}

	// Validasi format & ukuran file
	const maxSize = 2 * 1024 * 1024 // 2MB
	if file.Size > maxSize {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "FILE_TOO_LARGE",
			"message": "Ukuran file maksimal 2MB",
		})
	}

	// Validasi ekstensi file
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}
	if !allowedExts[ext] {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "INVALID_FILE_TYPE",
			"message": "Format foto harus jpg, jpeg, png, atau webp",
		})
	}

	// Pastikan folder uploads/photos sudah ada
	uploadDir := "uploads/photos"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal membuat folder penyimpanan",
		})
	}

	// Hapus foto lama dari disk (jika ada)
	oldURL, err := h.Repo.GetPhotoURL(employeeID)
	if err == nil && oldURL != "" {
		// oldURL format: "uploads/photos/5_1716123456.jpg"
		// Hapus file lama, abaikan error jika file sudah tidak ada
		os.Remove(oldURL)
	}

	// Generate nama file unik: {employeeID}_{unix_timestamp}.{ext}
	filename := fmt.Sprintf("%d_%d%s", employeeID, time.Now().Unix(), ext)
	savePath := filepath.Join(uploadDir, filename)
	photoURL := "/uploads/photos/" + filename

	// Simpan file ke disk
	if err := c.SaveFile(file, savePath); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menyimpan foto",
		})
	}

	// Simpan path foto ke DB
	if err := h.Repo.UpdatePhotoURL(employeeID, photoURL); err != nil {
		// Rollback: hapus file yang baru disimpan kalau DB gagal
		_ = os.Remove(savePath)
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menyimpan data foto",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Foto profil berhasil diupload",
		"data": fiber.Map{
			"photo_url": "/employee/profile/photo/view/" + filename,
		},
	})
}

// ─────────────────────────────────────────
// F5: DELETE /employee/profile/photo
//
// Hapus foto profil karyawan.
// File di disk dihapus + kolom photo_url di DB di-set NULL.
// ─────────────────────────────────────────

func (h *Handler) DeletePhoto(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	// Ambil path foto dari DB
	photoURL, err := h.Repo.GetPhotoURL(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil data foto",
		})
	}

	if photoURL == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "NO_PHOTO",
			"message": "Tidak ada foto profil yang bisa dihapus",
		})
	}

	// Hapus file dari disk
	_ = os.Remove(photoURL) // Abaikan error kalau file sudah tidak ada

	// Clear path foto di DB
	if err := h.Repo.ClearPhotoURL(employeeID); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menghapus data foto",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Foto profil berhasil dihapus",
	})
}

func (h *Handler) ViewPhoto(c *fiber.Ctx) error {
	filename := c.Params("filename")

	filePath := "./uploads/photos/" + filename

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.Status(404).JSON(fiber.Map{
			"status":  "error",
			"message": "File tidak ditemukan",
			"path":    filePath,
		})
	}

	// penting untuk ngrok
	c.Set("ngrok-skip-browser-warning", "true")

	return c.SendFile(filePath)
}
