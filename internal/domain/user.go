package domain

type User struct {
	ID       string `json:"user_id"`
	Username string `json:"username"`
	TeamID   int    `json:"-"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}
