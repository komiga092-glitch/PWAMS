package models

type DonorListQuery struct {
	Search   string `form:"search"`
	Type     string `form:"type"`
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
