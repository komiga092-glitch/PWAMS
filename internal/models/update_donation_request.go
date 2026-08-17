package models

type UpdateDonationRequest struct {
	PersonID string `json:"person_id"`

	DonationType string `json:"donation_type" binding:"required"`

	Amount   float64 `json:"amount" binding:"gte=0"`
	Currency string  `json:"currency"`

	ItemName string  `json:"item_name"`
	Quantity float64 `json:"quantity" binding:"gte=0"`
	Unit     string  `json:"unit"`

	Description  string `json:"description"`
	DonationDate string `json:"donation_date"`

	Status string `json:"status" binding:"required"`
}
