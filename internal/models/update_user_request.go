package models

type UpdateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email,max=100"`
	Role     string `json:"role" binding:"required"`
	Status   string `json:"status" binding:"required"`
}
