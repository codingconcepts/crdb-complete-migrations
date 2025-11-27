package model

type Account struct {
	CustomerID int     `json:"customer_id"`
	Balance    float64 `json:"balance"`
}
