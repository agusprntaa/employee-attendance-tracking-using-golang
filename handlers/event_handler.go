package handlers

import (
	"absensi_karyawan/auth"
	"absensi_karyawan/models"
	"absensi_karyawan/repository"
	"absensi_karyawan/utils"
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type EventHandler struct {
	eventRepo *repository.EventRepo
}

func NewEventHandler(er *repository.EventRepo) *EventHandler {
	return &EventHandler{eventRepo: er}
}

// POST /admin-cabang/events
// Admin cabang buat event baru
func (h *EventHandler) CreateEvent(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	var req models.CreateEventRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("BODY PARSER ERROR: %v", err)
		log.Printf("RAW BODY: %s", string(c.Body()))
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	// Debug request setelah BodyParser
	log.Printf("CreateEvent Request: %+v", req)
	log.Printf("Latitude: %f | Longitude: %f", req.Latitude, req.Longitude)

	req.Name = strings.TrimSpace(req.Name)
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)

	if req.Name == "" {
		return utils.BadRequest(c, "NAME_REQUIRED", "Nama event wajib diisi")
	}
	if req.StartDate == "" || req.EndDate == "" {
		return utils.BadRequest(c, "DATE_REQUIRED", "Tanggal mulai dan selesai wajib diisi")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return utils.BadRequest(c, "TIME_REQUIRED", "Jam mulai dan selesai wajib diisi")
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return utils.BadRequest(c, "INVALID_START_DATE", "Format tanggal mulai harus YYYY-MM-DD")
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return utils.BadRequest(c, "INVALID_END_DATE", "Format tanggal selesai harus YYYY-MM-DD")
	}

	if endDate.Before(startDate) {
		return utils.BadRequest(c, "INVALID_DATE_RANGE", "Tanggal selesai tidak boleh sebelum tanggal mulai")
	}

	if req.RadiusMeter == 0 {
		req.RadiusMeter = 100
	}

	// Debug sebelum masuk repository
	log.Printf("SEND TO REPOSITORY -> Latitude: %f | Longitude: %f", req.Latitude, req.Longitude)

	id, err := h.eventRepo.CreateEvent(*claims.BranchID, claims.UserID, req)
	if err != nil {
		log.Printf("CREATE EVENT ERROR: %v", err)
		return utils.InternalError(c, "Gagal membuat event")
	}

	log.Printf("CREATE EVENT SUCCESS -> ID: %d", id)

	return utils.Created(c, fiber.Map{
		"id":           id,
		"name":         req.Name,
		"start_date":   req.StartDate,
		"end_date":     req.EndDate,
		"start_time":   req.StartTime,
		"end_time":     req.EndTime,
		"location":     req.Location,
		"radius_meter": req.RadiusMeter,
	})
}

// GET /admin-cabang/events
// List semua event di cabang ini
// Query params opsional: status (upcoming/active/done), date (YYYY-MM-DD)
func (h *EventHandler) GetEvents(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	status := c.Query("status", "")
	date := c.Query("date", "")

	if status != "" && status != "upcoming" && status != "active" && status != "done" {
		return utils.BadRequest(c, "INVALID_STATUS", "Status harus upcoming, active, atau done")
	}

	data, err := h.eventRepo.GetEventsByBranch(*claims.BranchID, status, date)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data event")
	}

	return utils.Success(c, data)
}

// GET /admin-cabang/events/:id/qr
// Ambil QR token aktif + sisa waktu berlaku
func (h *EventHandler) GetActiveQR(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	token, expiresAt, err := h.eventRepo.GetActiveQRToken(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil QR token")
	}
	if token == "" {
		return utils.NotFound(c, "QR token belum di-generate atau sudah expired")
	}

	// Hitung sisa waktu berlaku dalam detik
	remainingSeconds := int(time.Until(expiresAt).Seconds())

	return utils.Success(c, fiber.Map{
		"event_id":          eventID,
		"token":             token,
		"expires_at":        expiresAt.Format("2006-01-02 15:04:05"),
		"remaining_seconds": remainingSeconds,
	})
}

// GET /admin-cabang/events/:id/participants
func (h *EventHandler) GetParticipants(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	data, err := h.eventRepo.GetParticipants(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil daftar peserta")
	}

	return utils.Success(c, data)
}

// GET /admin-cabang/events/:id/participants/list
// Hanya peserta yang sudah terdaftar + data absensi
func (h *EventHandler) GetEventParticipantsList(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	date := c.Query("date", "")

	data, err := h.eventRepo.GetEventParticipantsList(eventID, *claims.BranchID, date)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil daftar peserta event")
	}

	return utils.Success(c, data)
}
// POST /admin-cabang/events/:id/participants
// Body: { "employee_ids": [1, 2, 3] }
func (h *EventHandler) SetParticipants(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	// Validasi event milik cabang ini
	event, err := h.eventRepo.GetEventByID(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data event")
	}
	if event == nil {
		return utils.NotFound(c, "Event tidak ditemukan")
	}

	var req struct {
		EmployeeIDs []int `json:"employee_ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}
	if len(req.EmployeeIDs) == 0 {
		return utils.BadRequest(c, "EMPTY_PARTICIPANTS", "Minimal pilih 1 karyawan")
	}

	if err := h.eventRepo.SetParticipants(eventID, req.EmployeeIDs); err != nil {
		return utils.InternalError(c, "Gagal menyimpan peserta event")
	}

	return utils.Success(c, fiber.Map{
		"event_id": eventID,
		"total":    len(req.EmployeeIDs),
		"pesan":    "Peserta event berhasil disimpan",
	})
}

// POST /admin-cabang/events/:id/participants/select-all
// Pilih semua karyawan aktif di cabang sebagai peserta event
func (h *EventHandler) SelectAllParticipants(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	// Validasi event milik cabang ini
	event, err := h.eventRepo.GetEventByID(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data event")
	}
	if event == nil {
		return utils.NotFound(c, "Event tidak ditemukan")
	}

	// Ambil semua ID karyawan aktif di cabang
	employeeIDs, err := h.eventRepo.GetAllEmployeeIDs(*claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data karyawan")
	}
	if len(employeeIDs) == 0 {
		return utils.BadRequest(c, "NO_EMPLOYEES", "Tidak ada karyawan aktif di cabang ini")
	}

	// Set semua karyawan sebagai peserta
	if err := h.eventRepo.SetParticipants(eventID, employeeIDs); err != nil {
		return utils.InternalError(c, "Gagal menyimpan peserta event")
	}

	return utils.Success(c, fiber.Map{
		"event_id": eventID,
		"total":    len(employeeIDs),
		"pesan":    "Semua karyawan berhasil ditambahkan sebagai peserta event",
	})
}

// DELETE /admin-cabang/events/:id/participants/:employee_id
func (h *EventHandler) RemoveParticipant(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	employeeID, err := strconv.Atoi(c.Params("employee_id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID karyawan tidak valid")
	}

	// Validasi event milik cabang ini
	event, err := h.eventRepo.GetEventByID(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data event")
	}
	if event == nil {
		return utils.NotFound(c, "Event tidak ditemukan")
	}

	deleted, err := h.eventRepo.RemoveParticipant(eventID, employeeID)
	if err != nil {
		return utils.InternalError(c, "Gagal menghapus peserta event")
	}
	if !deleted {
		return utils.NotFound(c, "Karyawan tidak ditemukan dalam peserta event")
	}

	return utils.SuccessMessage(c, "Peserta event berhasil dihapus")
}

// GET /admin-cabang/events/:id/dates
// List tanggal dari start_date - end_date untuk dropdown search
func (h *EventHandler) GetEventDateRange(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	dates, err := h.eventRepo.GetEventDateRange(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil rentang tanggal event")
	}

	return utils.Success(c, fiber.Map{
		"event_id": eventID,
		"dates":    dates,
	})
}

// GET /admin-cabang/events/:id/attendance
// Rekap siapa saja yang sudah scan QR event
func (h *EventHandler) GetEventAttendance(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	event, err := h.eventRepo.GetEventByID(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil data event")
	}
	if event == nil {
		return utils.NotFound(c, "Event tidak ditemukan")
	}

	// Ambil query param date, default ke hari ini atau hari pertama event
	date := c.Query("date", "")

	data, err := h.eventRepo.GetEventAttendanceByDate(eventID, *claims.BranchID, date)
	if err != nil {
		return utils.InternalError(c, "Gagal mengambil rekap absen event")
	}

	return utils.Success(c, data)
}
// GET /admin-cabang/events/:id
func (h *EventHandler) GetEventByID(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

event, err := h.eventRepo.GetEventByID(eventID, *claims.BranchID)
if err != nil {
	log.Printf("GetEventByID ERROR: %v", err)
	return utils.InternalError(c, "Gagal mengambil detail event")
}
	if event == nil {
		return utils.NotFound(c, "Event tidak ditemukan")
	}

	// Ambil date dari query param, default ke hari ini / hari pertama / hari terakhir
	date := c.Query("date", "")

	// Summary peserta berdasarkan tanggal
	totalParticipants, totalHadir, totalBelum, err := h.eventRepo.GetEventSummary(eventID, *claims.BranchID, date)
if err != nil {
	log.Printf("GetEventSummary ERROR: %v", err)
	return utils.InternalError(c, "Gagal mengambil summary event")
}


	// Generate list tanggal
	dates, err := h.eventRepo.GetEventDateRange(eventID, *claims.BranchID)
if err != nil {
	log.Printf("GetEventDateRange ERROR: %v", err)
	return utils.InternalError(c, "Gagal mengambil rentang tanggal event")
}

	return utils.Success(c, fiber.Map{
		"id":                 event.ID,
		"name":               event.Name,
		"description":        event.Description,
		"location":           event.Location,
		"latitude":           event.Latitude,
		"longitude":          event.Longitude,
		"radius_meter":       event.RadiusMeter,
		"start_date":         event.StartDate,
		"end_date":           event.EndDate,
		"start_time":         event.StartTime,
		"end_time":           event.EndTime,
		"expires_at":         event.ExpiresAt,
		"created_at":         event.CreatedAt,
		"dates":              dates,
		"total_participants": totalParticipants,
		"total_hadir":        totalHadir,
		"total_belum":        totalBelum,
	})
}

//update event 
func (h *EventHandler) UpdateEvent(c *fiber.Ctx) error {

	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	var req models.UpdateEventRequest

	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequest(c, "INVALID_BODY", "Request body tidak valid")
	}

	req.Name = strings.TrimSpace(req.Name)
	req.StartDate = strings.TrimSpace(req.StartDate)
	req.EndDate = strings.TrimSpace(req.EndDate)
	req.StartTime = strings.TrimSpace(req.StartTime)
	req.EndTime = strings.TrimSpace(req.EndTime)

	if req.Name == "" {
		return utils.BadRequest(c, "NAME_REQUIRED", "Nama event wajib diisi")
	}

	if req.StartDate == "" || req.EndDate == "" {
		return utils.BadRequest(c, "DATE_REQUIRED", "Tanggal mulai dan selesai wajib diisi")
	}

	if req.StartTime == "" || req.EndTime == "" {
		return utils.BadRequest(c, "TIME_REQUIRED", "Jam mulai dan selesai wajib diisi")
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return utils.BadRequest(c, "INVALID_START_DATE", "Format tanggal mulai harus YYYY-MM-DD")
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return utils.BadRequest(c, "INVALID_END_DATE", "Format tanggal selesai harus YYYY-MM-DD")
	}

	if endDate.Before(startDate) {
		return utils.BadRequest(c, "INVALID_DATE_RANGE", "Tanggal selesai tidak boleh sebelum tanggal mulai")
	}

	if req.RadiusMeter == 0 {
		req.RadiusMeter = 100
	}

	err = h.eventRepo.UpdateEvent(eventID, *claims.BranchID, req)
	if err != nil {
		if err == sql.ErrNoRows {
			return utils.NotFound(c, "Event tidak ditemukan")
		}
		return utils.InternalError(c, "Gagal mengubah event")
	}

	return utils.Success(c, fiber.Map{
		"id":             eventID,
		"name":           req.Name,
		"description":    req.Description,
		"location":       req.Location,
		"latitude":       req.Latitude,
		"longitude":      req.Longitude,
		"radius_meter":   req.RadiusMeter,
		"start_date":     req.StartDate,
		"end_date":       req.EndDate,
		"start_time":     req.StartTime,
		"end_time":       req.EndTime,
	})
}
// DELETE /admin-cabang/events/:id
// Hapus event
func (h *EventHandler) DeleteEvent(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims == nil || claims.BranchID == nil {
		return utils.BadRequest(c, "NO_BRANCH", "Admin tidak memiliki cabang yang terdaftar")
	}

	eventID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return utils.BadRequest(c, "INVALID_ID", "ID event tidak valid")
	}

	deleted, err := h.eventRepo.DeleteEvent(eventID, *claims.BranchID)
	if err != nil {
		return utils.InternalError(c, "Gagal menghapus event")
	}
	if !deleted {
		return utils.NotFound(c, "Event tidak ditemukan")
	}

	return utils.SuccessMessage(c, "Event berhasil dihapus")
}