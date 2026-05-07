package branches

import "database/sql"

type Repository struct {
	DB *sql.DB
}

// GetAll ambil semua branch
func (r *Repository) GetAll() ([]*BranchResponse, error) {
	rows, err := r.DB.Query(`
		SELECT id, name, address, latitude, longitude, radius_meter
		FROM branches
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*BranchResponse
	for rows.Next() {
		var b BranchResponse
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Address,
			&b.Latitude, &b.Longitude, &b.RadiusMeter,
		); err != nil {
			return nil, err
		}
		result = append(result, &b)
	}
	return result, nil
}

// GetByID ambil satu branch by ID
func (r *Repository) GetByID(id int) (*BranchResponse, error) {
	var b BranchResponse
	err := r.DB.QueryRow(`
		SELECT id, name, address, latitude, longitude, radius_meter
		FROM branches WHERE id = $1
	`, id).Scan(
		&b.ID, &b.Name, &b.Address,
		&b.Latitude, &b.Longitude, &b.RadiusMeter,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// Create insert branch baru
func (r *Repository) Create(req *BranchRequest) error {
	_, err := r.DB.Exec(`
		INSERT INTO branches (name, address, latitude, longitude, radius_meter)
		VALUES ($1, $2, $3, $4, $5)
	`, req.Name, req.Address, req.Latitude, req.Longitude, req.RadiusMeter)
	return err
}

// Update update data branch
func (r *Repository) Update(id int, req *BranchRequest) error {
	_, err := r.DB.Exec(`
		UPDATE branches
		SET name = $1, address = $2, latitude = $3, longitude = $4, radius_meter = $5
		WHERE id = $6
	`, req.Name, req.Address, req.Latitude, req.Longitude, req.RadiusMeter, id)
	return err
}

// Delete hapus branch
func (r *Repository) Delete(id int) error {
	_, err := r.DB.Exec(`DELETE FROM branches WHERE id = $1`, id)
	return err
}
