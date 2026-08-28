package models

import "github.com/shopspring/decimal"

type UpdateDonationRequest struct {
	PersonID string `json:"person_id"`

	DonationType string `json:"donation_type" binding:"required"`

	Amount   decimal.Decimal `json:"amount" binding:"required"`
	Currency string          `json:"currency" binding:"omitempty,max=10"`

	ItemName string          `json:"item_name" binding:"omitempty,max=150"`
	Quantity decimal.Decimal `json:"quantity" binding:"required"`
	Unit     string          `json:"unit" binding:"omitempty,max=30"`

	Description  string `json:"description"`
	DonationDate string `json:"donation_date"`
	ReferenceNo  string `json:"reference_no" binding:"omitempty,max=100"`

	Status string `json:"status" binding:"required"`
}
