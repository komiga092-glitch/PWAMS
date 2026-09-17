package main

import (
	"fmt"
	"os"

	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

func main() {
	svc := services.NewReportPDFService()
	b, err := svc.RenderSummaryReport(
		&models.DashboardReport{TotalUsers: 12, TotalPersons: 40, TotalStudents: 25, TotalDonors: 8, TotalDonations: 31, TotalAidRequests: 17, TotalCareProvided: 22},
		&models.DonationReport{TotalDonations: 31, TotalAmount: 154375.5},
		&models.AidRequestReport{TotalRequests: 17, PendingRequests: 5, ApprovedRequests: 8, RejectedRequests: 2, CancelledRequests: 2},
	)
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	fmt.Printf("bytes=%d header=%q\n", len(b), string(b[:5]))
	if string(b[:5]) != "%PDF-" {
		fmt.Println("BAD HEADER")
		os.Exit(1)
	}
	os.WriteFile("bin\\smoke-report.pdf", b, 0o644)
	fmt.Println("OK")
}
