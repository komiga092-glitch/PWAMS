package models

type UserListQuery struct {
	Search   string `form:"search"`
	Role     string `form:"role"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
