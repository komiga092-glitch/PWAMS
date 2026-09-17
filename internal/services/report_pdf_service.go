package services

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/komiga092-glitch/pwams/internal/models"
)

// ReportPDFService renders the summary report data (dashboard,
// donations, aid requests) into a downloadable PDF. Extend this with
// a per-category table renderer once row-level report data exists.
type ReportPDFService struct{}

func NewReportPDFService() *ReportPDFService {
	return &ReportPDFService{}
}

func (s *ReportPDFService) RenderSummaryReport(
	dashboard *models.DashboardReport,
	donations *models.DonationReport,
	aidRequests *models.AidRequestReport,
) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("PWAMS Summary Report", false)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(41, 55, 127) // brand blue #29377f
	pdf.CellFormat(0, 10, "PWAMS Summary Report", "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(90, 90, 90)
	pdf.CellFormat(
		0, 6,
		"Generated "+time.Now().Format("02 Jan 2006 15:04"),
		"", 1, "C", false, 0, "",
	)
	pdf.Ln(8)

	section := func(title string, rows [][2]string) {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetTextColor(41, 55, 127)
		pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(30, 30, 30)
		for _, row := range rows {
			pdf.CellFormat(90, 7, row[0], "B", 0, "L", false, 0, "")
			pdf.CellFormat(90, 7, row[1], "B", 1, "L", false, 0, "")
		}
		pdf.Ln(6)
	}

	section("Dashboard", [][2]string{
		{"Total Users", fmt.Sprintf("%d", dashboard.TotalUsers)},
		{"Total Beneficiaries", fmt.Sprintf("%d", dashboard.TotalPersons)},
		{"Total Students", fmt.Sprintf("%d", dashboard.TotalStudents)},
		{"Total Donors", fmt.Sprintf("%d", dashboard.TotalDonors)},
		{"Total Donations", fmt.Sprintf("%d", dashboard.TotalDonations)},
		{"Total Aid Requests", fmt.Sprintf("%d", dashboard.TotalAidRequests)},
		{"Total Care Provided", fmt.Sprintf("%d", dashboard.TotalCareProvided)},
	})

	section("Donations", [][2]string{
		{"Total Donations", fmt.Sprintf("%d", donations.TotalDonations)},
		{"Total Amount", fmt.Sprintf("%.2f", donations.TotalAmount)},
	})

	section("Aid Requests", [][2]string{
		{"Total Requests", fmt.Sprintf("%d", aidRequests.TotalRequests)},
		{"Pending", fmt.Sprintf("%d", aidRequests.PendingRequests)},
		{"Approved", fmt.Sprintf("%d", aidRequests.ApprovedRequests)},
		{"Rejected", fmt.Sprintf("%d", aidRequests.RejectedRequests)},
		{"Cancelled", fmt.Sprintf("%d", aidRequests.CancelledRequests)},
	})

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to render report PDF: %w", err)
	}
	return buf.Bytes(), nil
}
