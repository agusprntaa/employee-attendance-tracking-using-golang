package repository

import (
	"absensi_karyawan/models"
	"database/sql"
)

type SettingsRepo struct {
	db *sql.DB
}

func NewSettingsRepo(db *sql.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

// GetSettings - ambil semua settings dari tabel branches + divisions
func (r *SettingsRepo) GetSettings(branchID int) (*models.SettingsResponse, error) {
	settings := &models.SettingsResponse{}

	// Ambil semua kolom dari tabel branches (termasuk kolom baru)
	err := r.db.QueryRow(`
		SELECT
			name,
			COALESCE(address, ''),
			COALESCE(latitude, 0),
			COALESCE(longitude, 0),
			COALESCE(radius_meter, 100),
			COALESCE(auto_refresh_qr, true),
			COALESCE(require_admin_approval, false),
			COALESCE(admin_email, ''),
			COALESCE(email_notifications, false),
			COALESCE(late_arrival_alerts, false),
			COALESCE(weekly_reports, false)
		FROM branches
		WHERE id = $1
	`, branchID).Scan(
		&settings.BranchInformation.BranchName,
		&settings.BranchInformation.Address,
		&settings.BranchInformation.Latitude,
		&settings.BranchInformation.Longitude,
		&settings.BranchInformation.RadiusMeter,
		&settings.Security.AutoRefreshQR,
		&settings.Security.RequireAdminApproval,
		&settings.Notifications.AdminEmail,
		&settings.Notifications.EmailNotifications,
		&settings.Notifications.LateArrivalAlerts,
		&settings.Notifications.WeeklyReports,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Ambil Working Hours dari divisions
	// Ambil divisi pertama yang terhubung ke karyawan di cabang ini
	var divID int
	err = r.db.QueryRow(`
		SELECT DISTINCT d.id, d.name,
			d.work_start::text,
			d.work_end::text,
			d.late_tolerance_min,
			d.checkin_cutoff_min,
			d.work_days
		FROM divisions d
		JOIN employees e ON e.division_id = d.id
		WHERE e.branch_id = $1 AND e.status = 'active'
		ORDER BY d.id
		LIMIT 1
	`, branchID).Scan(
		&divID,
		&settings.WorkingHours.DivisionName,
		&settings.WorkingHours.StartTime,
		&settings.WorkingHours.EndTime,
		&settings.WorkingHours.LateThresholdMin,
		&settings.WorkingHours.CheckinCutoffMin,
		&settings.WorkingHours.WorkDays,
	)
	if err == nil {
		settings.WorkingHours.DivisionID = &divID
	} else {
		// Default jika belum ada divisi
		settings.WorkingHours.StartTime = "08:00:00"
		settings.WorkingHours.EndTime = "17:00:00"
		settings.WorkingHours.LateThresholdMin = 15
		settings.WorkingHours.CheckinCutoffMin = 120
		settings.WorkingHours.WorkDays = "Senin,Selasa,Rabu,Kamis,Jumat"
	}

	return settings, nil
}

// UpdateSettings - update semua settings sekaligus
func (r *SettingsRepo) UpdateSettings(branchID int, req *models.UpdateSettingsRequest) error {
	// 1. Update tabel branches (branch info + security + notifications)
	_, err := r.db.Exec(`
		UPDATE branches SET
			name                  = $1,
			address               = $2,
			latitude              = $3,
			longitude             = $4,
			radius_meter          = $5,
			auto_refresh_qr       = $6,
			require_admin_approval= $7,
			admin_email           = $8,
			email_notifications   = $9,
			late_arrival_alerts   = $10,
			weekly_reports        = $11
		WHERE id = $12
	`,
		req.BranchName, req.Address, req.Latitude, req.Longitude, req.RadiusMeter,
		req.AutoRefreshQR, req.RequireAdminApproval,
		req.AdminEmail, req.EmailNotifications, req.LateArrivalAlerts, req.WeeklyReports,
		branchID,
	)
	if err != nil {
		return err
	}

	// 2. Update tabel divisions (working hours)
	// Jika division_id dikirim dari FE, gunakan itu
	// Jika tidak, cari divisi pertama yang terhubung ke cabang ini dan update
	divisionID := req.DivisionID
	if divisionID == nil {
		var foundDivID int
		err = r.db.QueryRow(`
			SELECT DISTINCT d.id
			FROM divisions d
			JOIN employees e ON e.division_id = d.id
			WHERE e.branch_id = $1 AND e.status = 'active'
			ORDER BY d.id
			LIMIT 1
		`, branchID).Scan(&foundDivID)
		if err == nil {
			divisionID = &foundDivID
		}
	}

	if divisionID != nil {
		_, err = r.db.Exec(`
			UPDATE divisions SET
				work_start         = $1::time,
				work_end           = $2::time,
				late_tolerance_min = $3,
				checkin_cutoff_min = $4,
				work_days          = $5
			WHERE id = $6
		`, req.StartTime, req.EndTime, req.LateThresholdMin, req.CheckinCutoffMin, req.WorkDays, *divisionID)
		if err != nil {
			return err
		}
	}

	return nil
}