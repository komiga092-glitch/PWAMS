package models

type ResetUserPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}
