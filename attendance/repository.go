package attendance

import (
	"absensi_karyawan/utils"
	"database/sql"
	"fmt"
	"time"
)

type Repository struct {
	DB *sql.DB
}

// ─────────────────────────────────────────
// Struct internal — data dari DB
// ─────────────────────────────────────────

// EmployeeDetail berisi semua data yang dibutuhkan untuk validasi check-in / checkout.
//
// RequiredHours TIDAK membutuhkan kolom baru — dihitung otomatis dari
// work_end - work_start yang sudah ada di tabel divisions.
//
// Contoh hasil perhitungan:
//
//	IT  → work_start=08:00:00, work_end=17:00:00 → RequiredHours = 9.0
//	HR  → work_start=08:00:00, work_end=16:00:00 → RequiredHours = 8.0
type EmployeeDetail struct {
	ID            int
	BranchID      int
	DivisionID    int
	WorkStart     string  // "08:00:00" — untuk deteksi ON_TIME / LATE
	WorkDays      string  // "1,2,3,4,5" — validasi hari kerja
	LateTolMin    int     // toleransi terlambat dalam menit
	CutoffMin     int     // batas maksimal check-in dari work_start (menit)
	RequiredHours float64 // dihitung: WorkEnd - WorkStart dalam jam
	BranchLat     float64
	BranchLon     float64
	RadiusMeter   int
}

// AttendanceRecord adalah representasi satu baris tabel attendance di DB.
type AttendanceRecord struct {
	ID               int        `json:"id"`
	EmployeeID       int        `json:"employee_id"`
	BranchID         int        `json:"branch_id"`
	Date             string     `json:"date"`
	WorkType         string     `json:"work_type"`
	Status           string     `json:"status"`
	CheckIn          *time.Time `json:"check_in"`
	CheckOut         *time.Time `json:"check_out"`
	CheckInLat       *float64   `json:"check_in_lat"`
	CheckInLon       *float64   `json:"check_in_lon"`
	DistanceMeter    *float64   `json:"distance_meter"`
	LateMinutes      *int       `json:"late_minutes"`
	WFAReason        *string    `json:"wfa_reason"`
	EarlyLeaveReason *string    `json:"early_leave_reason,omitempty"`
	IsAutoCheckout   bool       `json:"is_auto_checkout"`
}

// ─────────────────────────────────────────
// Query functions
// ─────────────────────────────────────────

// GetEmployeeDetail ambil data karyawan + division + branch sekaligus.
//
// Catatan: work_start dan work_end di-cast ke ::text agar PostgreSQL mengembalikan
// string "HH:MM:SS" yang mudah di-parse di Go, bukan driver-specific time.Time.
// RequiredHours dihitung dari selisih keduanya — tidak ada kolom baru di DB.
func (r *Repository) GetEmployeeDetail(employeeID int) (*EmployeeDetail, error) {
	var e EmployeeDetail
	var workEndStr string

	err := r.DB.QueryRow(`
		SELECT
			e.id,
			e.branch_id,
			e.division_id,
			d.work_start::text,
			d.work_end::text,
			d.work_days,
			d.late_tolerance_min,
			d.checkin_cutoff_min,
			b.latitude,
			b.longitude,
			b.radius_meter
		FROM employees e
		JOIN divisions d ON d.id = e.division_id
		JOIN branches  b ON b.id = e.branch_id
		WHERE e.id = $1
	`, employeeID).Scan(
		&e.ID,
		&e.BranchID,
		&e.DivisionID,
		&e.WorkStart,
		&workEndStr,
		&e.WorkDays,
		&e.LateTolMin,
		&e.CutoffMin,
		&e.BranchLat,
		&e.BranchLon,
		&e.RadiusMeter,
	)
	if err != nil {
		return nil, err
	}

	// Hitung RequiredHours dari work_end - work_start.
	// Fallback ke 8.0 jam jika parse gagal (data korup di DB).
	e.RequiredHours = calcRequiredHours(e.WorkStart, workEndStr)

	return &e, nil
}

// calcRequiredHours hitung durasi kerja wajib (jam) dari work_start dan work_end.
//
// Format input: "HH:MM:SS" — output PostgreSQL dari cast ::text pada kolom time.
// Fallback ke 8.0 jam jika salah satu string tidak bisa di-parse atau selisihnya ≤ 0.
func calcRequiredHours(startStr, endStr string) float64 {
	const layout = "15:04:05"

	start, err := time.Parse(layout, startStr)
	if err != nil {
		return 8.0
	}
	end, err := time.Parse(layout, endStr)
	if err != nil {
		return 8.0
	}

	diff := end.Sub(start)
	if diff <= 0 {
		return 8.0
	}

	return diff.Hours()
}

// TodayAttendanceExists cek apakah karyawan sudah check-in hari ini.
func (r *Repository) TodayAttendanceExists(employeeID int) (bool, error) {
	var count int
	today := utils.TodayDate()

	err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM attendance
		WHERE employee_id = $1 AND date = $2
	`, employeeID, today).Scan(&count)

	return count > 0, err
}

// GetTodayAttendance ambil record absensi hari ini milik karyawan.
// Return (nil, nil) jika belum ada record.
func (r *Repository) GetTodayAttendance(employeeID int) (*AttendanceRecord, error) {
	today := utils.TodayDate()
	var a AttendanceRecord

	err := r.DB.QueryRow(`
		SELECT
			id, employee_id, branch_id, date,
			work_type, status,
			check_in, check_out,
			check_in_lat, check_in_lon,
			distance_meter, late_minutes,
			wfa_reason, early_leave_reason,
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
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// InsertAttendance insert record check-in baru ke tabel attendance.
func (r *Repository) InsertAttendance(a *AttendanceRecord) error {
	today := utils.TodayDate()

	_, err := r.DB.Exec(`
		INSERT INTO attendance (
			employee_id, branch_id, date, work_type, status,
			check_in, check_in_lat, check_in_lon,
			distance_meter, late_minutes, wfa_reason
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`,
		a.EmployeeID, a.BranchID, today, a.WorkType, a.Status,
		a.CheckIn, a.CheckInLat, a.CheckInLon,
		a.DistanceMeter, a.LateMinutes, a.WFAReason,
	)
	return err
}

// UpdateCheckOut update record checkout karyawan.
func (r *Repository) UpdateCheckOut(
	employeeID int,
	checkOut time.Time,
	status string,
	earlyLeaveReason *string,
) error {
	var reason any
	if earlyLeaveReason != nil {
		reason = *earlyLeaveReason
	} else {
		reason = nil
	}

	_, err := r.DB.Exec(`
		UPDATE attendance
		SET
			check_out          = $1,
			status             = $2,
			early_leave_reason = $3
		WHERE employee_id = $4
		  AND check_out IS NULL
	`, checkOut, status, reason, employeeID)

	return err
}

// GetAttendanceHistory ambil riwayat absensi karyawan dengan paginasi.
func (r *Repository) GetAttendanceHistory(employeeID, limit, offset int) ([]*AttendanceRecord, int, error) {
	var total int
	if err := r.DB.QueryRow(`
		SELECT COUNT(*) FROM attendance WHERE employee_id = $1
	`, employeeID).Scan(&total); err != nil {
		return nil, 0, err
	}

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

	fmt.Println("TOTAL RECORD:", len(records))

	return records, total, nil
}
