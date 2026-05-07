package attendance

import (
	"database/sql"
	"time"
)

type Repository struct {
	DB *sql.DB
}

// ─────────────────────────────────────────
// Struct internal untuk data dari DB
// ─────────────────────────────────────────

type EmployeeDetail struct {
	ID          int
	BranchID    int
	DivisionID  int
	WorkStart   time.Time
	WorkEnd     time.Time
	WorkDays    string // "1,2,3,4,5"
	LateTolMin  int
	CutoffMin   int
	BranchLat   float64
	BranchLon   float64
	RadiusMeter int
}

type AttendanceRecord struct {
	ID               int
	EmployeeID       int
	BranchID         int
	Date             string
	WorkType         string
	Status           string
	CheckIn          *time.Time
	CheckOut         *time.Time
	CheckInLat       *float64
	CheckInLon       *float64
	DistanceMeter    *float64
	LateMinutes      *int
	WFAReason        *string
	EarlyLeaveReason *string
	IsAutoCheckout   bool
}

// ─────────────────────────────────────────
// Query functions
// ─────────────────────────────────────────

// GetEmployeeDetail ambil data karyawan + division + branch sekaligus
// Dipakai saat check-in untuk validasi work_start, cutoff, radius, dll
func (r *Repository) GetEmployeeDetail(employeeID int) (*EmployeeDetail, error) {
	var e EmployeeDetail

	err := r.DB.QueryRow(`
		SELECT 
			e.id, e.branch_id, e.division_id,
			d.work_start, d.work_end, d.work_days,
			d.late_tolerance_min, d.checkin_cutoff_min,
			b.latitude, b.longitude, b.radius_meter
		FROM employees e
		JOIN divisions d ON d.id = e.division_id
		JOIN branches b  ON b.id = e.branch_id
		WHERE e.id = $1
	`, employeeID).Scan(
		&e.ID, &e.BranchID, &e.DivisionID,
		&e.WorkStart, &e.WorkEnd, &e.WorkDays,
		&e.LateTolMin, &e.CutoffMin,
		&e.BranchLat, &e.BranchLon, &e.RadiusMeter,
	)

	if err != nil {
		return nil, err
	}
	return &e, nil
}

// TodayAttendanceExists cek apakah karyawan sudah check-in hari ini
func (r *Repository) TodayAttendanceExists(employeeID int) (bool, error) {
	var count int
	today := time.Now().UTC().Format("2006-01-02")

	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM attendance
		WHERE employee_id = $1 AND date = $2
	`, employeeID, today).Scan(&count)

	return count > 0, err
}

// GetTodayAttendance ambil record absensi hari ini milik karyawan
func (r *Repository) GetTodayAttendance(employeeID int) (*AttendanceRecord, error) {
	today := time.Now().UTC().Format("2006-01-02")
	var a AttendanceRecord

	err := r.DB.QueryRow(`
		SELECT id, employee_id, branch_id, date, work_type, status,
		       check_in, check_out, check_in_lat, check_in_lon,
		       distance_meter, late_minutes, wfa_reason, early_leave_reason,
		       is_auto_checkout
		FROM attendance
		WHERE employee_id = $1 AND date = $2
	`, employeeID, today).Scan(
		&a.ID, &a.EmployeeID, &a.BranchID, &a.Date,
		&a.WorkType, &a.Status,
		&a.CheckIn, &a.CheckOut,
		&a.CheckInLat, &a.CheckInLon,
		&a.DistanceMeter, &a.LateMinutes,
		&a.WFAReason, &a.EarlyLeaveReason,
		&a.IsAutoCheckout,
	)

	if err == sql.ErrNoRows {
		return nil, nil // belum check-in
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// InsertAttendance insert record check-in baru
func (r *Repository) InsertAttendance(a *AttendanceRecord) error {
	today := time.Now().UTC().Format("2006-01-02")

	_, err := r.DB.Exec(`
		INSERT INTO attendance (
			employee_id, branch_id, date, work_type, status,
			check_in, check_in_lat, check_in_lon,
			distance_meter, late_minutes, wfa_reason
		)	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`,
		a.EmployeeID, a.BranchID, today, a.WorkType, a.Status,
		a.CheckIn, a.CheckInLat, a.CheckInLon,
		a.DistanceMeter, a.LateMinutes, a.WFAReason,
	)
	return err
}

// UpdateCheckOut update record saat karyawan checkout
func (r *Repository) UpdateCheckOut(employeeID int, checkOut time.Time, status string, earlyReason *string) error {
	today := time.Now().UTC().Format("2006-01-02")

	_, err := r.DB.Exec(`
		UPDATE attendance
		SET check_out = $1, status = $2, early_leave_reason = $3
		WHERE employee_id = $4 AND date = $5
	`, checkOut, status, earlyReason, employeeID, today)

	return err
}

// GetAttendanceHistory ambil riwayat absensi karyawan (untuk halaman riwayat)
func (r *Repository) GetAttendanceHistory(employeeID, limit, offset int) ([]*AttendanceRecord, int, error) {
	// Query 1: hitung total record milik karyawan ini
	var total int
	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM attendance WHERE employee_id = $1
	`, employeeID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Query 2: ambil data sesuai halaman
	rows, err := r.DB.Query(`
	SELECT 
		id, employee_id, branch_id, date,
		work_type, status,
		check_in, check_out, 
		late_minutes, wfa_reason,
		early_leave_reason, is_auto_checkout
	FROM attendance
	WHERE employee_id = $1
	ORDER BY date DESC
	LIMIT $2 OFFSET $3
	`, employeeID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []*AttendanceRecord
	for rows.Next() {
		var a AttendanceRecord
		if err := rows.Scan(
			&a.ID, &a.EmployeeID, &a.BranchID, &a.Date,
			&a.WorkType, &a.Status,
			&a.CheckIn, &a.CheckOut,
			&a.LateMinutes, &a.WFAReason,
			&a.EarlyLeaveReason, &a.IsAutoCheckout,
		); err != nil {
			return nil, 0, err
		}
		records = append(records, &a)
	}
	return records, total, nil
}
