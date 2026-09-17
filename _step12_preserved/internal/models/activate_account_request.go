package models

type ActivateAccountRequest struct {
	Token           string `json:"token" form:"token" binding:"required,min=32,max=128"`
	NewPassword     string `json:"new_password" form:"new_password" binding:"required,min=8,max=72"`
	ConfirmPassword string `json:"confirm_password" form:"confirm_password" binding:"required,min=8,max=72"`
}
