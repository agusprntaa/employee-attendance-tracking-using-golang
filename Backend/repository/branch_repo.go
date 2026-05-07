package repository

import (
	"absensi/models"
	"database/sql"
)

type BranchRepo struct {
	db *sql.DB
}

func NewBranchRepo(db *sql.DB) *BranchRepo {
	return &BranchRepo{db: db}
}

func (r *BranchRepo) GetByID(id int) (*models.Branch, error) {
	var b models.Branch
	err := r.db.QueryRow(`
		SELECT id, name, COALESCE(address,''), 
			COALESCE(latitude,0), COALESCE(longitude,0), 
			COALESCE(radius_meter,100)
		FROM branches WHERE id = $1
	`, id).Scan(&b.ID, &b.Name, &b.Address, &b.Latitude, &b.Longitude, &b.RadiusMeter)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (r *BranchRepo) Update(id int, req *models.UpdateBranchRequest) error {
	_, err := r.db.Exec(`
		UPDATE branches 
		SET name=$1, address=$2, latitude=$3, longitude=$4, radius_meter=$5
		WHERE id = $6
	`, req.Name, req.Address, req.Latitude, req.Longitude, req.RadiusMeter, id)
	return err
}

func (r *BranchRepo) List() ([]models.Branch, error) {
	rows, err := r.db.Query(`
		SELECT id, name, COALESCE(address,''), 
			COALESCE(latitude,0), COALESCE(longitude,0),
			COALESCE(radius_meter,100)
		FROM branches ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []models.Branch
	for rows.Next() {
		var b models.Branch
		rows.Scan(&b.ID, &b.Name, &b.Address, &b.Latitude, &b.Longitude, &b.RadiusMeter)
		list = append(list, b)
	}
	return list, nil
}
