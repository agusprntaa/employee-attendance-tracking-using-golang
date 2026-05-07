package repository

import (
	"absensi/models"
	"database/sql"
)

type DivisionRepo struct {
	db *sql.DB
}

func NewDivisionRepo(db *sql.DB) *DivisionRepo {
	return &DivisionRepo{db: db}
}

func (r *DivisionRepo) List() ([]models.Division, error) {
	rows, err := r.db.Query(`
		SELECT id, name, work_days, 
			work_start::text, work_end::text,
			late_tolerance_min, checkin_cutoff_min
		FROM divisions ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Division
	for rows.Next() {
		var d models.Division
		rows.Scan(&d.ID, &d.Name, &d.WorkDays, &d.WorkStart, &d.WorkEnd,
			&d.LateToleanceMin, &d.CheckinCutoffMin)
		list = append(list, d)
	}
	return list, nil
}

func (r *DivisionRepo) GetByID(id int) (*models.Division, error) {
	var d models.Division
	err := r.db.QueryRow(`
		SELECT id, name, work_days, 
			work_start::text, work_end::text,
			late_tolerance_min, checkin_cutoff_min
		FROM divisions WHERE id = $1
	`, id).Scan(&d.ID, &d.Name, &d.WorkDays, &d.WorkStart, &d.WorkEnd,
		&d.LateToleanceMin, &d.CheckinCutoffMin)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &d, err
}

func (r *DivisionRepo) Create(req *models.CreateDivisionRequest) (int, error) {
	var id int
	err := r.db.QueryRow(`
		INSERT INTO divisions (name, work_days, work_start, work_end, late_tolerance_min, checkin_cutoff_min)
		VALUES ($1, $2, $3::time, $4::time, $5, $6)
		RETURNING id
	`, req.Name, req.WorkDays, req.WorkStart, req.WorkEnd,
		req.LateToleanceMin, req.CheckinCutoffMin).Scan(&id)
	return id, err
}

func (r *DivisionRepo) Update(id int, req *models.CreateDivisionRequest) error {
	_, err := r.db.Exec(`
		UPDATE divisions 
		SET name=$1, work_days=$2, work_start=$3::time, work_end=$4::time,
			late_tolerance_min=$5, checkin_cutoff_min=$6
		WHERE id = $7
	`, req.Name, req.WorkDays, req.WorkStart, req.WorkEnd,
		req.LateToleanceMin, req.CheckinCutoffMin, id)
	return err
}

func (r *DivisionRepo) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM divisions WHERE id = $1`, id)
	return err
}
