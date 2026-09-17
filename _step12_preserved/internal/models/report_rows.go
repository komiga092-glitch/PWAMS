package models

import "time"

// ── Report row results (flattened for table rendering) ───────────────────

type UserReportRow struct {
	ID        string
	Username  string
	Email     string
	Role      string
	Status    string
	Active    bool
	LastLogin *time.Time
	CreatedAt time.Time
}

type PersonReportRow struct {
	ID          string
	FullName    string
	NICPassport string
	Phone       string
	Status      string
	CreatedAt   time.Time
}

type StudentReportRow struct {
	ID        string
	FullName  string
	School    string
	Grade     string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DonorReportRow struct {
	ID             string
	Name           string
	Email          string
	Phone          string
	Status         string
	TotalDonations int64
	CreatedAt      time.Time
}

type DonationReportRow struct {
	ID           string
	ReferenceNo  string
	DonorName    string
	DonationType string
	Amount       float64
	Currency     string
	DonationDate time.Time
	Status       string
	RecordedBy   string
	CreatedAt    time.Time
}

type AidRequestReportRow struct {
	ID              string
	Title           string
	BeneficiaryName string
	AidType         string
	RequestedAmount float64
	RequestDate     time.Time
	Status          string
	ApprovedBy      string
}

type CareProvidedReportRow struct {
	ID           string
	PersonName   string
	CareType     string
	Status       string
	ProvidedDate time.Time
	ProvidedBy   string
	CreatedAt    time.Time
}

type LoanReportRow struct {
	ID             string
	ReferenceNo    string
	BorrowerName   string
	Principal      float64
	TotalInterest  float64
	TotalRepayable float64
	PaidAmount     float64
	Outstanding    float64
	Status         string
	IssueDate      time.Time
	DueDate        *time.Time
	LastRepayment  *time.Time
}

type LoanRepaymentReportRow struct {
	ID           string
	PaymentRef   string
	LoanID       string
	Installment  int
	BorrowerName string
	Amount       float64
	PaidAmount   float64
	Status       string
	DueDate      time.Time
	PaidAt       *time.Time
}

type RevenueReportRow struct {
	ID          string
	ReferenceNo string
	RecordType  string
	Category    string
	Amount      float64
	RecordDate  time.Time
	RecordedBy  string
	CreatedAt   time.Time
}

type AuditActivityReportRow struct {
	ID        string
	Timestamp time.Time
	Username  string
	Action    string
	Entity    string
	EntityID  string
	Details   string
	OldValue  string
	NewValue  string
}

type SystemAlertReportRow struct {
	ID            string
	Severity      string
	Category      string
	AlertCode     string
	Title         string
	Status        string
	Occurrences   int
	FirstOccurred time.Time
	LastOccurred  time.Time
	ResolvedAt    *time.Time
	ResolvedBy    string
}

// AccountStatusReportRow is one account line of the account-status report
// detail table.
type AccountStatusReportRow struct {
	ID        string
	Username  string
	Email     string
	Role      string
	Status    string
	LastLogin *time.Time
	CreatedAt time.Time
}
