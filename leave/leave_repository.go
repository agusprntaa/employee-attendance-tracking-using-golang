package leave

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	DB *sql.DB
}

// ─────────────────────────────────────────
// Struct internal — data dari DB
// ─────────────────────────────────────────

type LeaveQuota struct {
	ID         int
	EmployeeID int
	Year       int
	Total      int
	Used       int
}

type LeaveRequest struct {
	ID             int
	EmployeeID     int
	LeaveType      string
	StartDate      string
	EndDate        string
	Reason         string
	Status         string
	Note           sql.NullString
	AttachmentPath sql.NullString
	CreatedAt      string
}

type HolidayRow struct {
	ID   int
	Date string
	Name string
}

// ✦ BARU — struct untuk data notifikasi dari DB
// Berbeda dengan LeaveRequest: tidak butuh Reason & AttachmentPath,
// tapi butuh IsRead dan UpdatedAt untuk keperluan notifikasi
type NotificationRow struct {
	ID         int
	EmployeeID int
	LeaveType  string
	StartDate  string
	EndDate    string
	Status     string
	Note       sql.NullString
	IsRead     bool
	UpdatedAt  string
}

// ─────────────────────────────────────────
// EMPLOYEE — ambil joined_at untuk validasi eligibility
// ─────────────────────────────────────────

// GetEmployeeJoinedAt — ambil tanggal bergabung karyawan
// Dipakai service untuk cek apakah sudah >= 1 tahun kerja
func (r *Repository) GetEmployeeJoinedAt(employeeID int) (time.Time, error) {
	var joinedAt time.Time
	err := r.DB.QueryRow(`
		SELECT created_at FROM employees WHERE id = $1
	`, employeeID).Scan(&joinedAt)
	return joinedAt, err
}

// ─────────────────────────────────────────
// QUOTA
// ─────────────────────────────────────────

// GetQuota — ambil kuota karyawan untuk tahun tertentu
// Kalau belum ada row → return quota kosong (total=0, used=0)
func (r *Repository) GetQuota(employeeID, year int) (*LeaveQuota, error) {
	var q LeaveQuota
	err := r.DB.QueryRow(`
		SELECT id, employee_id, year, total, used
		FROM leave_quotas
		WHERE employee_id = $1 AND year = $2
	`, employeeID, year).Scan(
		&q.ID, &q.EmployeeID, &q.Year, &q.Total, &q.Used,
	)
	if err == sql.ErrNoRows {
		return &LeaveQuota{
			EmployeeID: employeeID,
			Year:       year,
			Total:      0,
			Used:       0,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// AddUsed — tambah used saat cuti di-approve (dipanggil BE2)
func (r *Repository) AddUsed(employeeID, year, days int) error {
	_, err := r.DB.Exec(`
		UPDATE leave_quotas
		SET used = used + $1
		WHERE employee_id = $2 AND year = $3
	`, days, employeeID, year)
	return err
}

// SubtractUsed — kurangi used saat cuti di-cancel setelah approved
func (r *Repository) SubtractUsed(employeeID, year, days int) error {
	_, err := r.DB.Exec(`
		UPDATE leave_quotas
		SET used = GREATEST(used - $1, 0)
		WHERE employee_id = $2 AND year = $3
	`, days, employeeID, year)
	return err
}

// UpsertQuota — insert atau update kuota (dipakai cron job reset tiap tahun)
func (r *Repository) UpsertQuota(employeeID, year, total int) error {
	_, err := r.DB.Exec(`
		INSERT INTO leave_quotas (employee_id, year, total, used)
		VALUES ($1, $2, $3, 0)
		ON CONFLICT (employee_id, year)
		DO UPDATE SET total = $3, used = 0
	`, employeeID, year, total)
	return err
}

// ─────────────────────────────────────────
// LEAVE REQUEST
// ─────────────────────────────────────────

// InsertLeaveRequest — simpan pengajuan cuti baru dengan status pending
func (r *Repository) InsertLeaveRequest(employeeID int,
	leaveType, startDate, endDate, reason string,
	attachmentPath string,

) (int, error) {
	var id int

	var attachVal interface{}
	if attachmentPath != "" {
		attachVal = attachmentPath
	}

	err := r.DB.QueryRow(`
		INSERT INTO leave_requests
			(employee_id, leave_type, start_date, end_date, reason, status, attachment_path)
		VALUES ($1, $2, $3, $4, $5, 'pending', $6)
		RETURNING id
	`, employeeID, leaveType, startDate, endDate, reason, attachVal).Scan(&id)
	return id, err
}

// GetHistory — riwayat cuti karyawan dengan paginasi
func (r *Repository) GetHistory(employeeID, limit, offset int) ([]LeaveRequest, int, error) {
	var total int
	if err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM leave_requests WHERE employee_id = $1
	`, employeeID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.DB.Query(`
		SELECT
			id, employee_id, leave_type,
			start_date::text, end_date::text,
			reason, status, note, attachment_path,
			TO_CHAR(created_at, 'YYYY-MM-DD') as created_at
		FROM leave_requests
		WHERE employee_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, employeeID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []LeaveRequest
	for rows.Next() {
		var lr LeaveRequest
		if err := rows.Scan(
			&lr.ID, &lr.EmployeeID, &lr.LeaveType,
			&lr.StartDate, &lr.EndDate,
			&lr.Reason, &lr.Status, &lr.Note, &lr.AttachmentPath,
			&lr.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, lr)
	}
	if result == nil {
		result = []LeaveRequest{}
	}
	return result, total, nil
}

// GetByID — ambil satu request milik karyawan tertentu
// Return nil jika tidak ditemukan
func (r *Repository) GetByID(id, employeeID int) (*LeaveRequest, error) {
	var lr LeaveRequest
	err := r.DB.QueryRow(`
		SELECT
			id, employee_id, leave_type,
			start_date::text, end_date::text,
			reason, status, note, attachment_path,
			TO_CHAR(created_at, 'YYYY-MM-DD') as created_at
		FROM leave_requests
		WHERE id = $1 AND employee_id = $2
	`, id, employeeID).Scan(
		&lr.ID, &lr.EmployeeID, &lr.LeaveType,
		&lr.StartDate, &lr.EndDate,
		&lr.Reason, &lr.Status, &lr.Note, &lr.AttachmentPath,
		&lr.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &lr, err
}

// CancelRequest — batalkan cuti, hanya bisa kalau status masih pending
func (r *Repository) CancelRequest(id, employeeID int) error {
	result, err := r.DB.Exec(`
		UPDATE leave_requests
		SET status = 'cancelled'
		WHERE id = $1 AND employee_id = $2 AND status = 'pending'
	`, id, employeeID)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("pengajuan tidak ditemukan atau sudah tidak bisa dibatalkan")
	}
	return nil
}

// CheckOverlap — cek apakah ada pengajuan yang overlap dengan tanggal baru
// Overlap: start baru <= end lama AND end baru >= start lama
func (r *Repository) CheckOverlap(employeeID int, startDate, endDate string) (bool, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM leave_requests
		WHERE employee_id = $1
		  AND status NOT IN ('rejected', 'cancelled')
		  AND start_date <= $3
		  AND end_date   >= $2
	`, employeeID, startDate, endDate).Scan(&count)
	return count > 0, err
}

// ─────────────────────────────────────────
// PUBLIC HOLIDAYS
// ─────────────────────────────────────────

// GetHolidays — ambil semua hari libur dalam satu tahun
func (r *Repository) GetHolidays(year int) ([]HolidayRow, error) {
	rows, err := r.DB.Query(`
		SELECT id, date::text, name
		FROM public_holidays
		WHERE EXTRACT(YEAR FROM date) = $1
		ORDER BY date
	`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HolidayRow
	for rows.Next() {
		var h HolidayRow
		if err := rows.Scan(&h.ID, &h.Date, &h.Name); err != nil {
			continue
		}
		result = append(result, h)
	}
	if result == nil {
		result = []HolidayRow{}
	}
	return result, nil
}

// GetHolidaysInRange — ambil semua tanggal libur dalam range
// Dipakai untuk validasi range cuti sekaligus (lebih efisien dari IsHoliday per hari)
func (r *Repository) GetHolidaysInRange(startDate, endDate string) ([]string, error) {
	rows, err := r.DB.Query(`
		SELECT date::text FROM public_holidays
		WHERE date BETWEEN $1 AND $2
		ORDER BY date
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			continue
		}
		dates = append(dates, d)
	}
	return dates, nil
}

// ─────────────────────────────────────────
// CRON JOB helpers
// ─────────────────────────────────────────

// GetEligibleEmployees — ambil semua karyawan aktif yang sudah >= 1 tahun kerja
func (r *Repository) GetEligibleEmployees() ([]int, error) {
	rows, err := r.DB.Query(`
		SELECT id FROM employees
		WHERE status = 'active'
		  AND role   = 'karyawan'
		  AND created_at <= NOW() - INTERVAL '1 year'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// ResetQuotasForNewYear — dipanggil cron job tiap 1 Januari
// Reset kuota semua karyawan eligible ke total=12, used=0
func (r *Repository) ResetQuotasForNewYear() error {
	year := time.Now().Year()

	ids, err := r.GetEligibleEmployees()
	if err != nil {
		return err
	}

	for _, id := range ids {
		if err := r.UpsertQuota(id, year, 12); err != nil {
			// Log error tapi lanjut ke karyawan berikutnya
			fmt.Printf("[CRON] ERROR reset kuota employee %d: %v\n", id, err)
		}
	}
	return nil
}

// ─────────────────────────────────────────
// NOTIFICATION
// ─────────────────────────────────────────

// GetNotifications — ambil notifikasi karyawan dengan paginasi dan filter status opsional
//
// Status yang ditampilkan: pending, approved, rejected
// cancelled tidak ditampilkan karena itu aksi karyawan sendiri, bukan notif dari sistem
//
// Filter statusFilter: "" atau "all" = semua, "pending"/"approved"/"rejected" = filter spesifik
func (r *Repository) GetNotifications(employeeID, limit, offset int, statusFilter string) ([]NotificationRow, int, error) {
	// base query hanya tampilkan pending/approved/rejected, bukan cancelled
	args := []interface{}{employeeID}
	where := "WHERE employee_id = $1 AND status NOT IN ('cancelled')"

	// tambahkan filter status jika diminta
	if statusFilter != "" && statusFilter != "all" {
		args = append(args, statusFilter)
		where += fmt.Sprintf(" AND status = $%d", len(args))
	}

	var total int
	if err := r.DB.QueryRow(
		"SELECT COUNT(*) FROM leave_requests "+where, args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	// urut berdasarkan updated_at DESC supaya notif terbaru di atas
	limitIdx := len(args) + 1
	offsetIdx := len(args) + 2
	args = append(args, limit, offset)

	rows, err := r.DB.Query(`
		SELECT
			id, employee_id, leave_type,
			start_date::text, end_date::text,
			status, note, is_read,
			TO_CHAR(updated_at, 'YYYY-MM-DD HH24:MI') as updated_at
		FROM leave_requests
		`+where+`
		ORDER BY updated_at DESC
		LIMIT $`+fmt.Sprintf("%d", limitIdx)+` OFFSET $`+fmt.Sprintf("%d", offsetIdx),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []NotificationRow
	for rows.Next() {
		var n NotificationRow
		if err := rows.Scan(
			&n.ID, &n.EmployeeID, &n.LeaveType,
			&n.StartDate, &n.EndDate,
			&n.Status, &n.Note, &n.IsRead, &n.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, n)
	}
	if result == nil {
		result = []NotificationRow{}
	}
	return result, total, nil
}

// tandai satu notifikasi sudah dibaca
// Dipanggil saat karyawan tap notifikasi di FE
func (r *Repository) MarkNotificationRead(leaveID, employeeID int) error {
	_, err := r.DB.Exec(`
		UPDATE leave_requests
		SET is_read = true
		WHERE id = $1 AND employee_id = $2
	`, leaveID, employeeID)
	return err
}

// tandai semua notifikasi sudah dibaca sekaligus
// Dipanggil saat karyawan tap tombol "Tandai semua dibaca"
func (r *Repository) MarkAllNotificationsRead(employeeID int) error {
	_, err := r.DB.Exec(`
		UPDATE leave_requests
		SET is_read = true
		WHERE employee_id = $1
		  AND is_read = false
		  AND status NOT IN ('cancelled')
	`, employeeID)
	return err
}

// hitung notifikasi yang belum dibaca
// Dipakai untuk badge angka merah di icon notifikasi FE
// pending yang belum dibaca juga dihitung karena karyawan perlu tahu ada pengajuan diproses
func (r *Repository) GetUnreadCount(employeeID int) (int, error) {
	var count int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM leave_requests
		WHERE employee_id = $1
		  AND is_read = false
		  AND status NOT IN ('cancelled')
	`, employeeID).Scan(&count)
	return count, err
}

// ─────────────────────────────────────────
// Helper
// ─────────────────────────────────────────

func parseInt(s string) int {
	n := 0
	for _, c := range strings.TrimSpace(s) {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
