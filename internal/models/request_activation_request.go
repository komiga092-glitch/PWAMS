package models

type RequestActivationRequest struct {
	Email string `json:"email" binding:"required,email"`
}
