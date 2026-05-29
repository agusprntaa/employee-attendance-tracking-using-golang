package leave

import (
	"errors"
	"fmt" //
	"strings"
	"time"
)

type Service struct {
	Repo *Repository
}

// ─────────────────────────────────────────
// Error variables — pola sama dengan attendance
// ─────────────────────────────────────────

var (
	ErrNotEligible      = errors.New("NOT_ELIGIBLE")
	ErrNoQuota          = errors.New("NO_QUOTA")
	ErrHolidayConflict  = errors.New("HOLIDAY_CONFLICT")
	ErrDateOverlap      = errors.New("DATE_OVERLAP")
	ErrInvalidDate      = errors.New("INVALID_DATE")
	ErrReasonTooShort   = errors.New("REASON_TOO_SHORT")
	ErrCannotCancel     = errors.New("CANNOT_CANCEL")
	ErrInvalidLeaveType = errors.New("INVALID_LEAVE_TYPE")
)

// ─────────────────────────────────────────
// Helper
// ─────────────────────────────────────────

// calcTotalDays — hitung total hari dari range tanggal
// end - start + 1 (inklusif)
func calcTotalDays(startDate, endDate string) int {
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 0
	}
	return int(end.Sub(start).Hours()/24) + 1
}

// buildNotifText
// Generate title dan description notifikasi berdasarkan status cuti
//
// Alur notifikasi:
//   - submit pengajuan  → status 'pending'   → "Pengajuan cuti diproses"
//   - admin approve     → status 'approved'  → "Pengajuan cuti disetujui"
//   - admin reject      → status 'rejected'  → "Pengajuan cuti ditolak" + catatan admin
//
// Title dan description digenerate di sini supaya FE tidak perlu logic mapping sendiri.
// Nilai yang dikembalikan langsung dipakai di NotificationItem.Title dan .Description
func buildNotifText(leaveType, status, note string) (title, description string) {
	switch status {
	case "pending":
		title = "Pengajuan cuti diproses"
		description = fmt.Sprintf("Pengajuan %s sedang menunggu approval.", leaveType)
	case "approved":
		title = "Pengajuan cuti disetujui"
		description = fmt.Sprintf("Pengajuan %s telah disetujui HR.", leaveType)
	case "rejected":
		title = "Pengajuan cuti ditolak"
		description = fmt.Sprintf("Pengajuan %s ditolak HR.", leaveType)
		// sertakan catatan admin jika ada, supaya karyawan tahu alasan penolakan
		if note != "" {
			description += " Catatan: " + note
		}
	default:
		title = "Status cuti diperbarui"
		description = fmt.Sprintf("Status pengajuan %s telah diperbarui.", leaveType)
	}
	return
}

// ─────────────────────────────────────────
// GetMyQuota — ambil kuota karyawan tahun ini
// ─────────────────────────────────────────

func (s *Service) GetMyQuota(employeeID int) (*QuotaResponse, error) {
	year := time.Now().Year()

	// Ambil joined_at dari DB untuk cek eligibility
	joinedAt, err := s.Repo.GetEmployeeJoinedAt(employeeID)
	if err != nil {
		return nil, err
	}

	quota, err := s.Repo.GetQuota(employeeID, year)
	if err != nil {
		return nil, err
	}

	remaining := quota.Total - quota.Used
	if remaining < 0 {
		remaining = 0
	}

	// Bisa apply cuti kalau sudah 1 tahun kerja DAN masih ada sisa kuota
	eligible := time.Since(joinedAt) >= 365*24*time.Hour
	canApply := eligible && remaining > 0

	return &QuotaResponse{
		Year:      year,
		Total:     quota.Total,
		Used:      quota.Used,
		Remaining: remaining,
		CanApply:  canApply,
	}, nil
}

// ─────────────────────────────────────────
// RequestLeave — karyawan ajukan cuti
//
// Urutan validasi:
// 1. Validasi jenis cuti
// 2. Validasi format & logika tanggal
// 3. Validasi alasan minimal 10 karakter
// 4. Cek eligibility (1 tahun kerja) — hanya untuk Cuti Tahunan
// 5. Cek kuota sisa — hanya untuk Cuti Tahunan
// 6. Cek tidak ada overlap pengajuan sebelumnya
// 7. Cek tidak ada hari libur nasional dalam range
// ─────────────────────────────────────────

func (s *Service) RequestLeave(employeeID int, req LeaveRequestDTO, attachmentPath string) (*LeaveItem, error) {
	// 1. Validasi jenis cuti
	if !ValidLeaveTypes[req.LeaveType] {
		return nil, ErrInvalidLeaveType
	}

	// 2. Validasi format tanggal
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, ErrInvalidDate
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, ErrInvalidDate
	}
	if endDate.Before(startDate) {
		return nil, ErrInvalidDate
	}

	// 3. Validasi alasan minimal 10 karakter
	if len(strings.TrimSpace(req.Reason)) < 10 {
		return nil, ErrReasonTooShort
	}

	totalDays := calcTotalDays(req.StartDate, req.EndDate)

	// 4 & 5. Cek eligibility — sudah 1 tahun kerja? dan Validasi kuota — HANYA untuk Cuti Tahunan
	if LeaveTypeDeductsQuota(req.LeaveType) {
		joinedAt, err := s.Repo.GetEmployeeJoinedAt(employeeID)
		if err != nil {
			return nil, err
		}
		// cek sudah 1 tahun kerja
		if time.Since(joinedAt) < 365*24*time.Hour {
			return nil, ErrNotEligible
		}

		// Cek kuota sisa
		year := startDate.Year()
		quota, err := s.Repo.GetQuota(employeeID, year)
		if err != nil {
			return nil, err
		}
		remaining := quota.Total - quota.Used
		if remaining < totalDays {
			return nil, ErrNoQuota
		}
	}

	// Cuti Sakit, Izin Pribadi, Cuti Melahirkan:
	// Tidak perlu cek eligibility dan kuota — langsung lanjut

	// 6. Cek overlap dengan pengajuan yang sudah ada
	overlap, err := s.Repo.CheckOverlap(employeeID, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, ErrDateOverlap
	}

	// 7. Cek hari libur nasional dalam range tanggal
	holidays, err := s.Repo.GetHolidaysInRange(req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}
	if len(holidays) > 0 {
		return nil, ErrHolidayConflict
	}

	// Semua validasi lolos → simpan ke DB
	id, err := s.Repo.InsertLeaveRequest(
		employeeID,
		req.LeaveType,
		req.StartDate,
		req.EndDate,
		req.Reason,
		attachmentPath,
	)
	if err != nil {
		return nil, err
	}

	return &LeaveItem{
		ID:             id,
		LeaveType:      req.LeaveType,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		TotalDays:      totalDays,
		Reason:         req.Reason,
		Status:         "pending",
		AttachmentPath: attachmentPath,
		CreatedAt:      time.Now().Format("2006-01-02"),
	}, nil
}

// ─────────────────────────────────────────
// GetMyHistory — riwayat cuti karyawan
// ─────────────────────────────────────────

func (s *Service) GetMyHistory(employeeID, page, limit int) (*LeaveHistoryResponse, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	rows, total, err := s.Repo.GetHistory(employeeID, limit, offset)
	if err != nil {
		return nil, err
	}

	var data []LeaveItem
	for _, r := range rows {
		item := LeaveItem{
			ID:        r.ID,
			LeaveType: r.LeaveType,
			StartDate: r.StartDate,
			EndDate:   r.EndDate,
			TotalDays: calcTotalDays(r.StartDate, r.EndDate),
			Reason:    r.Reason,
			Status:    r.Status,
			CreatedAt: r.CreatedAt,
		}
		if r.Note.Valid {
			item.Note = r.Note.String
		}
		if r.AttachmentPath.Valid {
			item.AttachmentPath = r.AttachmentPath.String
		}
		data = append(data, item)
	}

	if data == nil {
		data = []LeaveItem{}
	}

	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}

	return &LeaveHistoryResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// ─────────────────────────────────────────
// CancelLeave — batalkan cuti yang masih pending
// ─────────────────────────────────────────

func (s *Service) CancelLeave(employeeID, leaveID int) error {
	lr, err := s.Repo.GetByID(leaveID, employeeID)
	if err != nil {
		return err
	}
	if lr == nil {
		return ErrCannotCancel
	}
	if lr.Status != "pending" {
		return ErrCannotCancel
	}
	return s.Repo.CancelRequest(leaveID, employeeID)
}

// ─────────────────────────────────────────
// GetHolidays — list hari libur untuk kalender FE
// Default ke tahun ini jika tidak diisi
// ─────────────────────────────────────────

func (s *Service) GetHolidays(year int) ([]HolidayItem, error) {
	if year == 0 {
		year = time.Now().Year()
	}

	rows, err := s.Repo.GetHolidays(year)
	if err != nil {
		return nil, err
	}

	var result []HolidayItem
	for _, r := range rows {
		result = append(result, HolidayItem{
			ID:   r.ID,
			Date: r.Date,
			Name: r.Name,
		})
	}
	if result == nil {
		result = []HolidayItem{}
	}
	return result, nil
}

// ─────────────────────────────────────────
// NOTIFICATION SERVICE
// ─────────────────────────────────────────

// GetNotifications — ambil daftar notifikasi karyawan
//
// Alur yang benar:
//   - Karyawan submit → status 'pending'  → notif muncul: "diproses"   (is_read=false)
//   - Admin approve   → status 'approved' → notif berubah: "disetujui" (is_read=false, di-reset oleh DB trigger)
//   - Admin reject    → status 'rejected' → notif berubah: "ditolak"   (is_read=false, di-reset oleh DB trigger)
//
// Default limit = 5 sesuai permintaan FE
// statusFilter: "all" / "pending" / "approved" / "rejected"
func (s *Service) GetNotifications(employeeID, page, limit int, statusFilter string) (*NotificationResponse, error) {
	// default limit 5 untuk halaman notifikasi
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	rows, total, err := s.Repo.GetNotifications(employeeID, limit, offset, statusFilter)
	if err != nil {
		return nil, err
	}

	// ambil unread count untuk badge di icon notif FE
	unread, err := s.Repo.GetUnreadCount(employeeID)
	if err != nil {
		return nil, err
	}

	var data []NotificationItem
	for _, r := range rows {
		noteStr := ""
		if r.Note.Valid {
			noteStr = r.Note.String
		}

		// generate title & description dari status, bukan hardcode di FE
		title, desc := buildNotifText(r.LeaveType, r.Status, noteStr)

		item := NotificationItem{
			ID:          r.ID,
			LeaveType:   r.LeaveType,
			StartDate:   r.StartDate,
			EndDate:     r.EndDate,
			Status:      r.Status,
			Title:       title,
			Description: desc,
			IsRead:      r.IsRead,
			UpdatedAt:   r.UpdatedAt,
		}
		if noteStr != "" {
			item.Note = noteStr
		}
		data = append(data, item)
	}

	if data == nil {
		data = []NotificationItem{}
	}

	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}

	return &NotificationResponse{
		Data:        data,
		Total:       total,
		Page:        page,
		Limit:       limit,
		TotalPages:  totalPages,
		UnreadCount: unread,
	}, nil
}

// MarkNotificationRead
// Tandai satu notifikasi sudah dibaca
// Dipanggil saat karyawan tap salah satu notifikasi
func (s *Service) MarkNotificationRead(employeeID, leaveID int) error {
	return s.Repo.MarkNotificationRead(leaveID, employeeID)
}

// MarkAllNotificationsRead
// Tandai semua notifikasi sudah dibaca sekaligus
// Dipanggil saat karyawan tap "Tandai semua dibaca"
func (s *Service) MarkAllNotificationsRead(employeeID int) error {
	return s.Repo.MarkAllNotificationsRead(employeeID)
}
