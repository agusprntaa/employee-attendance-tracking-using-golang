package branches

type BranchRequest struct {
	BranchName  string  `json:"branch_name"`
	Address     string  `json:"address"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	RadiusMeter int     `json:"radius_meter"`
	Status      string  `json:"status"`
}

type BranchResponse struct {
	ID             int     `json:"id"`
	BranchID       string  `json:"branch_id"`
	BranchName     string  `json:"branch_name"`
	Address        string  `json:"address"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	RadiusMeter    int     `json:"radius_meter"`
	Status         string  `json:"status"`
	CreatedDate    string  `json:"created_date"`
	TodayPresent   int     `json:"today_present"`
	TodayAbsent    int     `json:"today_absent"`
	TotalEmployees int     `json:"total_employees"`
}
