package models

type PersonListQuery struct {
	Search   string `form:"search"`
	Status   string `form:"status"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
