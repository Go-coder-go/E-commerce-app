package models

type Order struct {
	ID     int64      `json:"id"`
	UserID string     `json:"user_id"`
	Items  []CartItem `json:"items"`
	Total  float64    `json:"total"`
}
