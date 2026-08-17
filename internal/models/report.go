package models

type DashboardReport struct {
	TotalUsers        int64 `json:"total_users"`
	TotalPersons      int64 `json:"total_persons"`
	TotalStudents     int64 `json:"total_students"`
	TotalDonors       int64 `json:"total_donors"`
	TotalDonations    int64 `json:"total_donations"`
	TotalAidRequests  int64 `json:"total_aid_requests"`
	TotalCareProvided int64 `json:"total_care_provided"`
}

type DonationReport struct {
	TotalDonations int64   `json:"total_donations"`
	TotalAmount    float64 `json:"total_amount"`
}

type AidRequestReport struct {
	TotalRequests     int64 `json:"total_requests"`
	PendingRequests   int64 `json:"pending_requests"`
	ApprovedRequests  int64 `json:"approved_requests"`
	RejectedRequests  int64 `json:"rejected_requests"`
	CancelledRequests int64 `json:"cancelled_requests"`
}
