package models

type StudentListQuery struct {
	Search   string `form:"search"`
	School   string `form:"school"`
	Grade    string `form:"grade"`
	Status   string `form:"status"`
	PersonID string `form:"person_id"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
