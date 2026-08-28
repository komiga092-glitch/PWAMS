package models

import "github.com/shopspring/decimal"

type UpdateAidRequest struct {
	PersonID        string          `json:"person_id" binding:"required,uuid"`
	AidType         string          `json:"aid_type" binding:"required"`
	Priority        string          `json:"priority" binding:"required"`
	Title           string          `json:"title" binding:"required,min=3,max=200"`
	Description     string          `json:"description" binding:"required,min=5"`
	RequestedAmount decimal.Decimal `json:"requested_amount" binding:"required"`
	Currency        string          `json:"currency" binding:"omitempty,max=10"`
	RequestDate     string          `json:"request_date"`
	NeededBy        string          `json:"needed_by"`
}
