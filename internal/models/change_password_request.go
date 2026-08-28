package models

type ChangePasswordRequest struct {
	CurrentPassword string `form:"current_password" binding:"required"`
	NewPassword     string `form:"new_password" binding:"required,min=8,max=72"`
	ConfirmPassword string `form:"confirm_password" binding:"required"`
}
