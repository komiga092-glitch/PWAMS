package models

type UpdatePersonRequest struct {
	FullName      string  `json:"full_name" binding:"required,min=3,max=150"`
	NICPassport   string  `json:"nic_passport" binding:"required,max=30"`
	DateOfBirth   string  `json:"date_of_birth"`
	Gender        string  `json:"gender" binding:"omitempty,oneof=Male Female Other"`
	Phone         string  `json:"phone" binding:"omitempty,max=20"`
	Email         string  `json:"email" binding:"omitempty,email,max=100"`
	Address       string  `json:"address"`
	Occupation    string  `json:"occupation" binding:"omitempty,max=100"`
	MonthlyIncome float64 `json:"monthly_income" binding:"gte=0"`
	Status        string  `json:"status" binding:"required"`
}
