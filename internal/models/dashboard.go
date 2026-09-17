package models

type DashboardStats struct {
	TotalBeneficiaries  int64            `json:"total_beneficiaries"`
	TotalStudents       int64            `json:"total_students"`
	TotalDonors         int64            `json:"total_donors"`
	ActiveLoans         int64            `json:"active_loans"`
	PendingAidRequests  int64            `json:"pending_aid_requests"`
	RevenueSummary      float64          `json:"revenue_summary"`
	UnreadNotifications int64            `json:"unread_notifications"`
	TotalUsers          int64            `json:"total_users"`
	ActiveUsers         int64            `json:"active_users"`
	DisabledUsers       int64            `json:"disabled_users"`
	LockedUsers         int64            `json:"locked_users"`
	UsersByRole         map[string]int64 `json:"users_by_role"`
}
