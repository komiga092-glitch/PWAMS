package models

type UpdateAidRequest struct {
	PersonID        string  `json:"person_id" binding:"required"`
	AidType         string  `json:"aid_type" binding:"required"`
	Priority        string  `json:"priority" binding:"required"`
	Title           string  `json:"title" binding:"required,min=3,max=200"`
	Description     string  `json:"description" binding:"required,min=5"`
	RequestedAmount float64 `json:"requested_amount" binding:"gte=0"`
	Currency        string  `json:"currency" binding:"omitempty,max=10"`
	RequestDate     string  `json:"request_date"`
	NeededBy        string  `json:"needed_by"`
}
