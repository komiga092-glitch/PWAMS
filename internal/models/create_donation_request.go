package models

type CreateDonationRequest struct {
	DonorID  string `json:"donor_id" binding:"required"`
	PersonID string `json:"person_id"`

	DonationType string  `json:"donation_type" binding:"required"`
	Amount       float64 `json:"amount" binding:"gte=0"`
	Currency     string  `json:"currency" binding:"omitempty,max=10"`

	ItemName string  `json:"item_name" binding:"omitempty,max=150"`
	Quantity float64 `json:"quantity" binding:"gte=0"`
	Unit     string  `json:"unit" binding:"omitempty,max=30"`

	Description  string `json:"description"`
	DonationDate string `json:"donation_date"`
	ReferenceNo  string `json:"reference_no" binding:"omitempty,max=100"`
}
