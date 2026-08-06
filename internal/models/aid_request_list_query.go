package models

type AidRequestListQuery struct {
	Search   string `form:"search"`
	PersonID string `form:"person_id"`
	Type     string `form:"type"`
	Priority string `form:"priority"`
	Status   string `form:"status"`
	FromDate string `form:"from_date"`
	ToDate   string `form:"to_date"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
