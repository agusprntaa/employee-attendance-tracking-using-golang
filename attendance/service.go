package attendance

import (
	"absensi_karyawan/face"
	"absensi_karyawan/utils"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	Repo     *Repository
	FaceRepo *face.Repository
}

var (
	ErrAlreadyCheckedIn       = errors.New("ALREADY_CHECKED_IN")
	ErrAlreadyCheckedOut      = errors.New("ALREADY_CHECKED_OUT")
	ErrCutoffExceeded         = errors.New("CUTOFF_EXCEEDED")
	ErrGPSAccuracyLow         = errors.New("GPS_ACCURACY_LOW")
	ErrQRInvalid              = errors.New("QR_INVALID")
	ErrBranchMismatch         = errors.New("BRANCH_MISMATCH")
	ErrOutOfRadius            = errors.New("OUT_OF_RADIUS")
	ErrWFAReasonTooShort      = errors.New("WFA_REASON_TOO_SHORT")
	ErrNotCheckedIn           = errors.New("NOT_CHECKED_IN")
	ErrEarlyLeaveReason       = errors.New("EARLY_LEAVE_REASON_REQUIRED")
	ErrNotWorkDay             = errors.New("NOT_WORK_DAY")
	ErrEmployeeDataIncomplete = errors.New("EMPLOYEE_DATA_INCOMPLETE")
)

// ─────────────────────────────────────────
// Helper functions (inline — tidak perlu helper.go)
// ─────────────────────────────────────────

// parseWorkStart parse string "HH:MM:SS" atau "HH:MM" menjadi time.Time
// pada hari yang sama dengan base, dalam timezone yang sama dengan base.
// Dipakai HANYA untuk menentukan status ON_TIME / LATE dan validasi cutoff.
func parseWorkStart(base time.Time, hhmm string) (time.Time, error) {
	// Coba format dengan detik dulu (output PostgreSQL ::text = "08:00:00")
	t, err := time.Parse("15:04:05", hhmm)
	if err != nil {
		// Fallback ke format tanpa detik
		t, err = time.Parse("15:04", hhmm)
		if err != nil {
			return time.Time{}, err
		}
	}
	return time.Date(
		base.Year(), base.Month(), base.Day(),
		t.Hour(), t.Minute(), 0, 0,
		base.Location(),
	), nil
}

// minutesDuration konversi menit (int) ke time.Duration
func minutesDuration(min int) time.Duration {
	return time.Duration(min) * time.Minute
}

// isWorkDay cek apakah hari ini termasuk hari kerja berdasarkan work_days divisi.
// work_days disimpan sebagai "1,2,3,4,5" di mana 1=Senin, 7=Minggu (ISO 8601).
func isWorkDay(workDays string) bool {
	today := utils.NowWITA()
	dayNum := int(today.Weekday()) // Go: 0=Sunday
	if dayNum == 0 {
		dayNum = 7 // normalisasi: Minggu = 7
	}
	todayStr := strconv.Itoa(dayNum)
	for _, d := range strings.Split(workDays, ",") {
		if strings.TrimSpace(d) == todayStr {
			return true
		}
	}
	return false
}

// recordToResponse konversi AttendanceRecord ke AttendanceResponse untuk frontend.
// Timestamp dikonversi eksplisit ke WITA karena lib/pq membaca dari DB sebagai UTC.
func recordToResponse(a *AttendanceRecord) *AttendanceResponse {
	resp := &AttendanceResponse{
		ID:             a.ID,
		Date:           a.Date,
		WorkType:       a.WorkType,
		Status:         a.Status,
		IsAutoCheckout: a.IsAutoCheckout,
	}
	// TIMESTAMP WITHOUT TIME ZONE: lib/pq simpan dan baca nilai UTC mentah.
	// Konversi ke WITA di sini agar frontend terima waktu lokal yang benar.
	if a.CheckIn != nil {
		t := a.CheckIn.In(utils.WITA)
		resp.CheckIn = &t
		fmt.Println("========== RESPONSE ==========")
		fmt.Println("RAW :", *a.CheckIn)
		fmt.Println("LOC :", a.CheckIn.Location())

		fmt.Println("WITA:", t)
		fmt.Println("==============================")

		resp.CheckIn = &t
	}
	if a.CheckOut != nil {
		t := a.CheckOut.In(utils.WITA)
		resp.CheckOut = &t
	}
	if a.LateMinutes != nil {
		resp.LateMinutes = *a.LateMinutes
	}
	if a.WFAReason != nil {
		resp.WFAReason = *a.WFAReason
	}
	if a.EarlyLeaveReason != nil {
		resp.EarlyLeaveReason = a.EarlyLeaveReason
	}
	if a.DistanceMeter != nil {
		resp.DistanceMeter = *a.DistanceMeter
	}
	return resp
}

// ─────────────────────────────────────────
// CheckIn — entry point
// ─────────────────────────────────────────

func (s *Service) CheckIn(
	employeeID int,
	req CheckInRequest,
) (*AttendanceRecord, error) {

	faceData, err := s.FaceRepo.GetEmployeeFaceData(employeeID)
	if err != nil {
		return nil, err
	}

	if !faceData.FaceRegistered {
		return nil, errors.New("FACE_NOT_REGISTERED")
	}

	if req.WorkType == "WFA" {
		return s.checkInWFA(employeeID, req)
	}

	return s.checkInWFO(employeeID, req)
}

// ─────────────────────────────────────────
// checkInWFO
// ─────────────────────────────────────────

func (s *Service) checkInWFO(employeeID int, req CheckInRequest) (*AttendanceRecord, error) {

	// 1. Cek duplikat check-in hari ini
	exists, err := s.Repo.TodayAttendanceExists(employeeID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAlreadyCheckedIn
	}

	// 2. Ambil data karyawan + divisi + cabang
	detail, err := s.Repo.GetEmployeeDetail(employeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmployeeDataIncomplete
		}
		return nil, err
	}

	now := utils.NowWITA()

	// 3. Validasi hari kerja divisi
	if !isWorkDay(detail.WorkDays) {
		return nil, ErrNotWorkDay
	}

	// 4. Validasi cutoff check-in (work_start + checkin_cutoff_min)
	//    Jika work_start gagal di-parse, lewati — jangan blokir check-in.
	workStart, parseErr := parseWorkStart(now, detail.WorkStart)
	if parseErr == nil {
		cutoffTime := workStart.Add(minutesDuration(detail.CutoffMin))
		if now.After(cutoffTime) {
			log.Println("CHECKIN CUTOFF — now:", now, "cutoff:", cutoffTime)
			return nil, ErrCutoffExceeded
		}
	} else {
		log.Println("WARN: gagal parse work_start:", detail.WorkStart, "—", parseErr)
	}

	// 5. Validasi GPS akurasi + radius
	locStatus, err := utils.ValidateLocation(
		req.Lat, req.Lon,
		detail.BranchLat, detail.BranchLon,
		detail.RadiusMeter, req.Accuracy,
	)
	if err != nil {
		return nil, err
	}
	if !locStatus.IsValid {
		if req.Accuracy > 200 {
			return nil, ErrGPSAccuracyLow
		}
		log.Println("===== LOCATION DEBUG =====")
		log.Println("USER LAT    :", req.Lat)
		log.Println("USER LON    :", req.Lon)
		log.Println("BRANCH LAT  :", detail.BranchLat)
		log.Println("BRANCH LON  :", detail.BranchLon)
		log.Println("DISTANCE    :", locStatus.Distance)
		log.Println("MAX RADIUS  :", detail.RadiusMeter)
		log.Println("GPS ACCURACY:", req.Accuracy)
		return nil, ErrOutOfRadius
	}

	// 6. Validasi QR token + branch match
	today := utils.TodayDate()
	if !utils.ValidateQRToken(req.QRToken, req.BranchID, today) {
		return nil, ErrQRInvalid
	}
	if req.BranchID != detail.BranchID {
		return nil, ErrBranchMismatch
	}

	// 7. Tentukan status: ON_TIME atau LATE
	//    Jika work_start gagal di-parse, default ON_TIME.
	status := "ON_TIME"
	lateMinutes := 0
	if parseErr == nil {
		toleranceDeadline := workStart.Add(time.Duration(detail.LateTolMin) * time.Minute)
		if now.After(toleranceDeadline) {
			status = "LATE"
			lateMinutes = int(now.Sub(workStart).Minutes())
		}
	}

	log.Printf("CHECKIN — employee:%d status:%s late:%dm time:%s",
		employeeID, status, lateMinutes, now.In(utils.WITA).Format("15:04:05"))

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

// ─────────────────────────────────────────
// checkInWFA
// ─────────────────────────────────────────

func (s *Service) checkInWFA(employeeID int, req CheckInRequest) (*AttendanceRecord, error) {

	// 1. Cek duplikat
	exists, err := s.Repo.TodayAttendanceExists(employeeID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrAlreadyCheckedIn
	}

	// 2. Validasi alasan WFA minimal 20 karakter
	if len(strings.TrimSpace(req.WFAReason)) < 20 {
		return nil, ErrWFAReasonTooShort
	}

	// 3. Ambil data karyawan
	detail, err := s.Repo.GetEmployeeDetail(employeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmployeeDataIncomplete
		}
		return nil, err
	}

	// 4. Validasi hari kerja
	if !isWorkDay(detail.WorkDays) {
		return nil, ErrNotWorkDay
	}

	now := utils.NowWITA()
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
//
// Logika berbasis durasi kerja aktual:
//
//	worked_duration  = now - check_in
//	required_duration = RequiredHours (dari work_end - work_start divisi)
//
//	Jika worked_duration < required_duration → EARLY_LEAVE, wajib isi alasan
//	Jika cukup                               → checkout normal
//
// Tidak ada perbandingan terhadap fixed work_end.
func (s *Service) CheckOut(employeeID int, req CheckOutRequest) (*AttendanceRecord, error) {

	// 1. Ambil record check-in hari ini
	record, err := s.Repo.GetTodayAttendance(employeeID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrNotCheckedIn
	}
	if record.CheckOut != nil {
		return nil, ErrAlreadyCheckedOut
	}

	// 2. Ambil aturan divisi
	detail, err := s.Repo.GetEmployeeDetail(employeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmployeeDataIncomplete
		}
		return nil, err
	}

	now := utils.NowWITA()

	// 3. Hitung durasi kerja aktual vs durasi wajib
	workedDuration := now.Sub(*record.CheckIn)
	requiredDuration := time.Duration(float64(time.Hour) * detail.RequiredHours)
	minCheckoutTime := record.CheckIn.Add(requiredDuration)

	log.Println("===== CHECKOUT DEBUG =====")
	log.Println("NOW              :", now.In(utils.WITA).Format("15:04:05"))
	log.Println("CHECK-IN         :", record.CheckIn.In(utils.WITA).Format("15:04:05"))
	log.Printf("WORKED           : %.0f menit\n", workedDuration.Minutes())
	log.Printf("REQUIRED         : %.0f menit (%.1f jam)\n", requiredDuration.Minutes(), detail.RequiredHours)
	log.Println("MIN CHECKOUT     :", minCheckoutTime.In(utils.WITA).Format("15:04:05"))
	log.Println("IS EARLY LEAVE   :", workedDuration < requiredDuration)
	log.Println("=========================")

	status := record.Status
	var earlyReason *string

	// 4. Deteksi early leave
	if workedDuration < requiredDuration {
		reason := strings.TrimSpace(req.EarlyLeaveReason)
		if reason == "" {
			return nil, ErrEarlyLeaveReason
		}
		status = "EARLY_LEAVE"
		earlyReason = &reason
	}

	// 5. Simpan ke DB
	if err := s.Repo.UpdateCheckOut(employeeID, now, status, earlyReason); err != nil {
		log.Println("UPDATE CHECKOUT ERROR:", err)
		return nil, err
	}

	record.CheckOut = &now
	record.Status = status
	record.EarlyLeaveReason = earlyReason

	return record, nil
}

// ─────────────────────────────────────────
// GetToday
// ─────────────────────────────────────────

func (s *Service) GetToday(employeeID int) (*TodayResponse, error) {
	record, err := s.Repo.GetTodayAttendance(employeeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &TodayResponse{HasCheckedIn: false, HasCheckedOut: false, Attendance: nil}, nil
		}
		return nil, err
	}
	if record == nil {
		return &TodayResponse{HasCheckedIn: false, HasCheckedOut: false, Attendance: nil}, nil
	}

	resp := recordToResponse(record)
	return &TodayResponse{
		HasCheckedIn:  true,
		HasCheckedOut: record.CheckOut != nil,
		Attendance:    resp,
	}, nil
}

// ─────────────────────────────────────────
// GetHistory
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
