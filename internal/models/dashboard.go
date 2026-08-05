package models

type DashboardStats struct {
	TotalUsers    int64            `json:"total_users"`
	ActiveUsers   int64            `json:"active_users"`
	DisabledUsers int64            `json:"disabled_users"`
	LockedUsers   int64            `json:"locked_users"`
	PendingUsers  int64            `json:"pending_users"`
	UsersByRole   map[string]int64 `json:"users_by_role"`
}
