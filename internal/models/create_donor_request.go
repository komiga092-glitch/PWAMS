package models

type CreateDonorRequest struct {
	Name                  string `json:"name" binding:"required,min=2,max=150"`
	DonorType             string `json:"donor_type" binding:"required"`
	NICPassport           string `json:"nic_passport" binding:"omitempty,max=30"`
	OrganizationName      string `json:"organization_name" binding:"omitempty,max=150"`
	RegistrationNumber    string `json:"registration_number" binding:"omitempty,max=50"`
	Phone                 string `json:"phone" binding:"omitempty,max=20"`
	Email                 string `json:"email" binding:"omitempty,email,max=100"`
	Address               string `json:"address"`
	ContactPersonName     string `json:"contact_person_name" binding:"omitempty,max=150"`
	ContactPersonPhone    string `json:"contact_person_phone" binding:"omitempty,max=20"`
	PreferredDonationType string `json:"preferred_donation_type" binding:"omitempty,max=50"`
	Notes                 string `json:"notes"`
}
