package models

type UpdateOwnProfileRequest struct {
	Username string `form:"username" binding:"required,min=3,max=50"`
	Email    string `form:"email" binding:"required,email,max=100"`
}
