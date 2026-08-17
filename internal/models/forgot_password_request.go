package models

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}
