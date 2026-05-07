package attendance

import (
	"absensi_karyawan/utils"
	"errors"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Repo *Repository
}

// ─────────────────────────────────────────
// Error codes — dikirim ke frontend
// Frontend bisa cek err.Error() untuk handle tiap kasus
// ─────────────────────────────────────────
var (
	ErrAlreadyCheckedIn  = errors.New("ALREADY_CHECKED_IN")
	ErrAlreadyCheckedOut = errors.New("ALREADY_CHECKED_OUT")
	ErrCutoffExceeded    = errors.New("CUTOFF_EXCEEDED")
	ErrGPSAccuracyLow    = errors.New("GPS_ACCURACY_LOW")
	ErrQRInvalid         = errors.New("QR_INVALID")
	ErrBranchMismatch    = errors.New("BRANCH_MISMATCH")
	ErrOutOfRadius       = errors.New("OUT_OF_RADIUS")
	ErrWFAReasonTooShort = errors.New("WFA_REASON_TOO_SHORT")
	ErrNotCheckedIn      = errors.New("NOT_CHECKED_IN")
	ErrEarlyLeaveReason  = errors.New("EARLY_LEAVE_REASON_REQUIRED")
)

// ─────────────────────────────────────────
// CheckIn — entry point utama
// Otomatis routing ke WFO atau WFA
// ─────────────────────────────────────────
func (s *Service) CheckIn(employeeID int, req CheckInRequest) (*AttendanceRecord, error) {
	if req.WorkType == "WFA" {
		return s.checkInWFA(employeeID, req)
	}
	return s.checkInWFO(employeeID, req)
}

// checkInWFO — 7 langkah validasi berurutan, JANGAN dibalik urutannya
func (s *Service) checkInWFO(employeeID int, req CheckInRequest) (*AttendanceRecord, error) {

	// ── Step 1: Cek duplicate check-in ─────────────────────────────
	exists, err := s.Repo.TodayAttendanceExists(employeeID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAlreadyCheckedIn
	}

	// ── Step 2: Ambil data employee + division + branch ─────────────
	detail, err := s.Repo.GetEmployeeDetail(employeeID)
	if err != nil {
		return nil, err
	}

	// ── Step 3: Cek cutoff time ─────────────────────────────────────
	// Cutoff = work_start + checkin_cutoff_min
	// Contoh: work_start 08:00, cutoff 120 menit → batas check-in 10:00
	now := utils.ServerTime()
	todayWorkStart := timeOfDay(now, detail.WorkStart)
	cutoffTime := todayWorkStart.Add(minutesDuration(detail.CutoffMin))
	if now.After(cutoffTime) {
		return nil, ErrCutoffExceeded
	}

	// ── Step 4: Validasi akurasi GPS ────────────────────────────────
	// utils.ValidateLocation hitung jarak dari koordinat yg dikirim FE
	// > 200m = tolak keras
	// 50-200m = bisa lanjut tapi frontend tampilkan warning
	locStatus, err := utils.ValidateLocation(
		req.Lat, req.Lon, // dari FE
		detail.BranchLat, detail.BranchLon, // dari DB
		detail.RadiusMeter, // dari DB
		req.Accuracy,       // dari device
	)
	if err != nil {
		return nil, err
	}
	if !locStatus.IsValid {
		if req.Accuracy > 200 {
			return nil, ErrGPSAccuracyLow
		}
		return nil, ErrOutOfRadius
	}

	// ── Step 5: Validasi HMAC QR token ─────────────────────────────
	today := utils.TodayDate()
	if !utils.ValidateQRToken(req.QRToken, req.BranchID, today) {
		return nil, ErrQRInvalid
	}

	// Pastikan QR yang di-scan adalah milik cabang karyawan sendiri
	if req.BranchID != detail.BranchID {
		return nil, ErrBranchMismatch
	}

	// ── Step 6: Tentukan status PRESENT atau LATE ───────────────────
	toleranceTime := todayWorkStart.Add(time.Duration(detail.LateTolMin))
	status := "PRESENT"
	lateMinutes := 0
	if now.After(toleranceTime) {
		status = "LATE"
		lateMinutes = int(now.Sub(todayWorkStart).Minutes())
	}

	// ── INSERT attendance ───────────────────────────────────────────
	checkInTime := now
	distance := locStatus.Distance
	record := &AttendanceRecord{
		EmployeeID:    employeeID,
		BranchID:      detail.BranchID,
		WorkType:      "WFO",
		Status:        status,
		CheckIn:       &checkInTime,
		CheckInLat:    &req.Lat,
		CheckInLon:    &req.Lon,
		DistanceMeter: &distance,
		LateMinutes:   &lateMinutes,
	}
	if err := s.Repo.InsertAttendance(record); err != nil {
		return nil, err
	}
	return record, nil
}

// checkInWFA — lebih simpel, tidak perlu QR atau GPS radius
func (s *Service) checkInWFA(employeeID int, req CheckInRequest) (*AttendanceRecord, error) {

	// Step 1: Cek duplicate
	exists, err := s.Repo.TodayAttendanceExists(employeeID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAlreadyCheckedIn
	}

	// Step 2: Validasi alasan WFA minimal 20 karakter
	if len(strings.TrimSpace(req.WFAReason)) < 20 {
		return nil, ErrWFAReasonTooShort
	}

	// Ambil branch_id karyawan untuk field branch_id di attendance
	detail, err := s.Repo.GetEmployeeDetail(employeeID)
	if err != nil {
		return nil, err
	}

	// Step 3: INSERT (waktu dari server)
	now := utils.ServerTime()
	reason := req.WFAReason
	record := &AttendanceRecord{
		EmployeeID: employeeID,
		BranchID:   detail.BranchID,
		WorkType:   "WFA",
		Status:     "WFA",
		CheckIn:    &now,
		WFAReason:  &reason,
	}
	if err := s.Repo.InsertAttendance(record); err != nil {
		return nil, err
	}
	return record, nil
}

// ─────────────────────────────────────────
// CheckOut
// ─────────────────────────────────────────
func (s *Service) CheckOut(employeeID int, req CheckOutRequest) (*AttendanceRecord, error) {

	// Step 1: Pastikan sudah check-in
	record, err := s.Repo.GetTodayAttendance(employeeID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrNotCheckedIn
	}

	// Step 2: Cek sudah checkout sebelumnya
	if record.CheckOut != nil {
		return nil, ErrAlreadyCheckedOut
	}

	// Step 3: Cek apakah early leave
	detail, err := s.Repo.GetEmployeeDetail(employeeID)
	if err != nil {
		return nil, err
	}

	// waktu dari server
	now := utils.ServerTime()
	todayWorkEnd := timeOfDay(now, detail.WorkEnd)

	status := record.Status // tetap PRESENT / LATE / WFA
	var earlyReason *string

	if now.Before(todayWorkEnd) {
		// Pulang lebih awal — wajib ada alasan
		if strings.TrimSpace(req.EarlyLeaveReason) == "" {
			return nil, ErrEarlyLeaveReason
		}
		status = "EARLY_LEAVE"
		r := req.EarlyLeaveReason
		earlyReason = &r
	}

	// Step 4: UPDATE
	if err := s.Repo.UpdateCheckOut(employeeID, now, status, earlyReason); err != nil {
		return nil, err
	}

	record.CheckOut = &now
	record.Status = status
	record.EarlyLeaveReason = earlyReason
	return record, nil
}

// ─────────────────────────────────────────
// GetToday — status absensi hari ini
// ─────────────────────────────────────────
func (s *Service) GetToday(employeeID int) (*TodayResponse, error) {
	record, err := s.Repo.GetTodayAttendance(employeeID)
	if err != nil {
		return nil, err
	}

	if record == nil {
		return &TodayResponse{
			HasCheckedIn:  false,
			HasCheckedOut: false,
			Attendance:    nil,
		}, nil
	}

	resp := recordToResponse(record)
	return &TodayResponse{
		HasCheckedIn:  true,
		HasCheckedOut: record.CheckOut != nil,
		Attendance:    resp,
	}, nil
}

// ─────────────────────────────────────────
// GetHistory — riwayat absensi (dengan pagination)
// ─────────────────────────────────────────
func (s *Service) GetHistory(employeeID, page, limit int) (*HistoryResponse, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	records, total, err := s.Repo.GetAttendanceHistory(employeeID, limit, offset)
	if err != nil {
		return nil, err
	}

	var data []*AttendanceResponse
	for _, r := range records {
		data = append(data, recordToResponse(r))
	}

	totalPages := total / limit
	if total%limit != 0 {
		totalPages++
	}

	return &HistoryResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}, nil
}

// ─────────────────────────────────────────
// Helper: konversi record DB ke response
// ─────────────────────────────────────────
func recordToResponse(a *AttendanceRecord) *AttendanceResponse {
	resp := &AttendanceResponse{
		ID:             a.ID,
		Date:           a.Date,
		WorkType:       a.WorkType,
		Status:         a.Status,
		CheckIn:        a.CheckIn,
		CheckOut:       a.CheckOut,
		IsAutoCheckout: a.IsAutoCheckout,
	}
	if a.LateMinutes != nil {
		resp.LateMinutes = *a.LateMinutes
	}
	if a.WFAReason != nil {
		resp.WFAReason = *a.WFAReason
	}
	if a.EarlyLeaveReason != nil {
		resp.EarlyLeaveReason = *a.EarlyLeaveReason
	}
	if a.DistanceMeter != nil {
		resp.DistanceMeter = *a.DistanceMeter
	}
	return resp
}

// isWorkDay cek apakah hari ini adalah hari kerja divisi ini
// Dipakai oleh cron job
func isWorkDay(workDays string) bool {
	today := utils.ServerTime()
	dayNum := int(today.Weekday())
	if dayNum == 0 {
		dayNum = 7
	}
	todayStr := strconv.Itoa(dayNum)
	for _, d := range strings.Split(workDays, ",") {
		if strings.TrimSpace(d) == todayStr {
			return true
		}
	}
	return false
}
