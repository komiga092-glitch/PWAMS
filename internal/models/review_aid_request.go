package models

import "github.com/shopspring/decimal"

type ReviewAidRequest struct {
	Status         string          `json:"status" binding:"required"`
	ApprovedAmount decimal.Decimal `json:"approved_amount" binding:"required"`
	ReviewNotes    string          `json:"review_notes"`
}
