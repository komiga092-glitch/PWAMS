package models

type ReviewAidRequest struct {
	Status         string  `json:"status" binding:"required"`
	ApprovedAmount float64 `json:"approved_amount" binding:"gte=0"`
	ReviewNotes    string  `json:"review_notes"`
}
