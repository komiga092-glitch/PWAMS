package models

type DonationListQuery struct {
	Search   string `form:"search"`
	DonorID  string `form:"donor_id"`
	PersonID string `form:"person_id"`
	Type     string `form:"type"`
	Status   string `form:"status"`
	FromDate string `form:"from_date"`
	ToDate   string `form:"to_date"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
