package leave

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	Service *Service
}

// errorMessage — pola sama persis dengan attendance/handler.go
// Return: (httpStatus, errorCode, userMessage)
func errorMessage(err error) (int, string, string) {
	switch err {
	case ErrNotEligible:
		return 403, "NOT_ELIGIBLE", "Karyawan belum memenuhi syarat cuti (minimal 1 tahun kerja)"
	case ErrNoQuota:
		return 400, "NO_QUOTA", "Kuota cuti tidak mencukupi"
	case ErrHolidayConflict:
		return 400, "HOLIDAY_CONFLICT", "Tanggal cuti mengandung hari libur nasional"
	case ErrDateOverlap:
		return 400, "DATE_OVERLAP", "Tanggal cuti sudah ada pengajuan sebelumnya"
	case ErrInvalidDate:
		return 400, "INVALID_DATE", "Format tanggal tidak valid atau end_date sebelum start_date"
	case ErrReasonTooShort:
		return 400, "REASON_TOO_SHORT", "Alasan cuti minimal 10 karakter"
	case ErrCannotCancel:
		return 400, "CANNOT_CANCEL", "Pengajuan tidak ditemukan atau sudah tidak bisa dibatalkan"
	case ErrInvalidLeaveType:
		return 400, "INVALID_LEAVE_TYPE", "Jenis cuti tidak valid. Pilih: Cuti Tahunan, Cuti Sakit, Izin Pribadi, atau Cuti Melahirkan"
	default:
		return 500, "INTERNAL_ERROR", "Terjadi kesalahan server, coba lagi"
	}
}

// ─────────────────────────────────────────
// GET /employee/leave/quota
// Karyawan lihat sisa kuota cuti tahun ini
// ─────────────────────────────────────────

func (h *Handler) GetMyQuota(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	result, err := h.Service.GetMyQuota(employeeID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil kuota cuti",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}

// ─────────────────────────────────────────
// POST /employee/leave/request
// Karyawan ajukan cuti baru

// Pakai multipart/form-data karena ada attachment opsional
// Field form:
//   - leave_type  : string (wajib)
//   - start_date  : string YYYY-MM-DD (wajib)
//   - end_date    : string YYYY-MM-DD (wajib)
//   - reason      : string min 10 karakter (wajib)
//   - attachment  : file PDF/JPG/PNG (opsional)
// ─────────────────────────────────────────

func (h *Handler) RequestLeave(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	// Ambil field form
	req := LeaveRequestDTO{
		LeaveType: strings.TrimSpace(c.FormValue("leave_type")),
		StartDate: strings.TrimSpace(c.FormValue("start_date")),
		EndDate:   strings.TrimSpace(c.FormValue("end_date")),
		Reason:    strings.TrimSpace(c.FormValue("reason")),
	}

	// Validasi field wajib
	if req.LeaveType == "" || req.StartDate == "" || req.EndDate == "" || req.Reason == "" {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "MISSING_FIELD",
			"message": "leave_type, start_date, end_date, dan reason wajib diisi",
		})
	}

	// Handle attachment opsional
	attachmentPath := ""
	file, err := c.FormFile("attachment")
	if err == nil && file != nil {
		// Ada file yang diupload — validasi format
		ext := strings.ToLower(filepath.Ext(file.Filename))
		allowedExt := map[string]bool{
			".pdf":  true,
			".jpg":  true,
			".jpeg": true,
			".png":  true,
		}
		if !allowedExt[ext] {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"code":    "INVALID_ATTACHMENT",
				"message": "Format attachment harus PDF, JPG, atau PNG",
			})
		}

		// Validasi ukuran maksimal 5MB
		const maxSize = 5 * 1024 * 1024
		if file.Size > maxSize {
			return c.Status(400).JSON(fiber.Map{
				"status":  "error",
				"code":    "ATTACHMENT_TOO_LARGE",
				"message": "Ukuran attachment maksimal 5MB",
			})
		}

		// Pastikan folder uploads/attachments/ ada
		uploadDir := "uploads/attachments"
		if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"status":  "error",
				"message": "Gagal menyiapkan folder attachment",
			})
		}

		// Nama file: {employeeID}_{timestamp}{ext}
		filename := fmt.Sprintf("%d_%d%s", employeeID, time.Now().Unix(), ext)
		savePath := filepath.Join(uploadDir, filename)

		if err := c.SaveFile(file, savePath); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"status":  "error",
				"message": "Gagal menyimpan attachment",
			})
		}

		attachmentPath = savePath
	}

	result, err := h.Service.RequestLeave(employeeID, req, attachmentPath)
	if err != nil {
		// Kalau service gagal, hapus file attachment yang sudah tersimpan
		if attachmentPath != "" {
			_ = os.Remove(attachmentPath)
		}
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
		})
	}

	return c.Status(201).JSON(fiber.Map{
		"status":  "success",
		"message": "Pengajuan cuti berhasil dikirim, menunggu persetujuan admin",
		"data":    result,
	})
}

// ─────────────────────────────────────────
// GET /employee/leave/history?page=1&limit=10
// Riwayat pengajuan cuti karyawan
// ─────────────────────────────────────────

func (h *Handler) GetMyHistory(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	result, err := h.Service.GetMyHistory(employeeID, page, limit)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil riwayat cuti",
			"detail":  err.Error(), // ← tambah baris ini saja
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}

// ─────────────────────────────────────────
// PATCH /employee/leave/:id/cancel
// Batalkan pengajuan cuti yang masih pending
// ─────────────────────────────────────────

func (h *Handler) CancelLeave(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	leaveID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "INVALID_ID",
			"message": "ID pengajuan tidak valid",
		})
	}

	if err := h.Service.CancelLeave(employeeID, leaveID); err != nil {
		status, code, msg := errorMessage(err)
		return c.Status(status).JSON(fiber.Map{
			"status":  "error",
			"code":    code,
			"message": msg,
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Pengajuan cuti berhasil dibatalkan",
	})
}

// ─────────────────────────────────────────
// GET /employee/leave/holidays?tahun=2026
// List hari libur nasional untuk kalender FE
// ─────────────────────────────────────────

func (h *Handler) GetHolidays(c *fiber.Ctx) error {
	year, _ := strconv.Atoi(c.Query("tahun", "0"))

	result, err := h.Service.GetHolidays(year)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil data hari libur",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}

// ─────────────────────────────────────────
// GET /employee/leave/types
// List jenis cuti yang tersedia untuk dropdown FE
// ─────────────────────────────────────────

func (h *Handler) GetLeaveTypes(c *fiber.Ctx) error {
	types := []fiber.Map{
		{"value": "Cuti Tahunan", "label": "Cuti Tahunan", "deducts_quota": true},
		{"value": "Cuti Sakit", "label": "Cuti Sakit", "deducts_quota": false},
		{"value": "Izin Pribadi", "label": "Izin Pribadi", "deducts_quota": false},
		{"value": "Cuti Melahirkan", "label": "Cuti Melahirkan", "deducts_quota": false},
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   types,
	})
}

// ─────────────────────────────────────────
// NOTIFICATION HANDLERS
// ─────────────────────────────────────────

// GET /employee/leave/notifications?page=1&limit=5&status=all
// Notifikasi perubahan status pengajuan cuti
//
// Query params:
//   - page   : int, default 1
//   - limit  : int, default 5
//   - status : string, default "all" (all / pending / approved / rejected)
//
// Alur: submit → pending (diproses) → approved/rejected setelah admin action
func (h *Handler) GetNotifications(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "5"))
	// filter status untuk chip filter di FE (Semua / Diproses / Disetujui / Ditolak)
	status := c.Query("status", "all")

	result, err := h.Service.GetNotifications(employeeID, page, limit, status)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal mengambil notifikasi",
		})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   result,
	})
}

// PATCH /employee/leave/notifications/:id/read
// Tandai satu notifikasi sudah dibaca
// Dipanggil saat karyawan tap salah satu notifikasi di FE
func (h *Handler) MarkNotificationRead(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	leaveID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"status":  "error",
			"code":    "INVALID_ID",
			"message": "ID tidak valid",
		})
	}

	if err := h.Service.MarkNotificationRead(employeeID, leaveID); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menandai notifikasi",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Notifikasi ditandai sudah dibaca",
	})
}

// PATCH /employee/leave/notifications/read-all
// Tandai semua notifikasi sudah dibaca sekaligus
// Dipanggil saat karyawan tap tombol "Tandai semua dibaca"
//
//	supaya Fiber tidak salah parse "read-all" sebagai :id
func (h *Handler) MarkAllNotificationsRead(c *fiber.Ctx) error {
	employeeID := c.Locals("user_id").(int)

	if err := h.Service.MarkAllNotificationsRead(employeeID); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"status":  "error",
			"message": "Gagal menandai semua notifikasi",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Semua notifikasi ditandai sudah dibaca",
	})
}
