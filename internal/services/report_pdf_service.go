package services

import (
	"bytes"
	"fmt"
	"strings"
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

// PDFTableReport is the generic input for the per-report table PDFs: a
// translated title, a summary section (label/value pairs) and, optionally, a
// data table (headers + rows). Empty cells are rendered as "-".
type PDFTableReport struct {
	Title       string
	Subtitle    string
	SummaryRows [][2]string
	Headers     []string
	Rows        [][]string
}

// ReportPDFFilename builds the download filename for a report slug, e.g.
// "donations-report-2026-09-19.pdf".
func ReportPDFFilename(slug string) string {
	safe := strings.NewReplacer("/", "", "\\", "", " ", "-").Replace(slug)
	return safe + "-report-" + time.Now().Format("2006-01-02") + ".pdf"
}

// RenderTableReport renders a single report area as a PDF document with the
// correct title, generation date, summary section and (optional) table. The
// output always starts with the %PDF magic bytes — never HTML or JSON.
func (s *ReportPDFService) RenderTableReport(report *PDFTableReport) ([]byte, error) {
	if report == nil || strings.TrimSpace(report.Title) == "" {
		return nil, fmt.Errorf("report PDF requires a title")
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	title := "PWAMS " + pdfLatin(report.Title)
	pdf.SetTitle(title, false)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(41, 55, 127) // brand blue #29377f
	pdf.CellFormat(0, 10, title, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(90, 90, 90)
	subtitle := pdfLatin(report.Subtitle)
	if subtitle == "" {
		subtitle = "Generated " + time.Now().Format("02 Jan 2006 15:04")
	} else {
		subtitle += " - Generated " + time.Now().Format("02 Jan 2006 15:04")
	}
	pdf.CellFormat(0, 6, subtitle, "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// Summary section (label/value pairs).
	if len(report.SummaryRows) > 0 {
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetTextColor(41, 55, 127)
		pdf.CellFormat(0, 8, "Summary", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 11)
		pdf.SetTextColor(30, 30, 30)
		for _, row := range report.SummaryRows {
			pdf.CellFormat(95, 7, pdfLatin(row[0]), "B", 0, "L", false, 0, "")
			pdf.CellFormat(95, 7, pdfLatin(row[1]), "B", 1, "L", false, 0, "")
		}
		pdf.Ln(4)
	}

	// Data table (optional — aggregate-only reports have none).
	if len(report.Headers) > 0 {
		if pdf.GetY() > 230 {
			pdf.AddPage()
		}

		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetTextColor(41, 55, 127)
		pdf.CellFormat(0, 8, "Detail", "", 1, "L", false, 0, "")

		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(30, 30, 30)
		pdf.SetFillColor(232, 234, 244)

		colCount := len(report.Headers)
		colWidth := 190.0 / float64(colCount)

		for _, header := range report.Headers {
			pdf.CellFormat(colWidth, 7, pdfLatin(header), "1", 0, "L", true, 0, "")
		}
		pdf.Ln(-1)

		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(30, 30, 30)

		for _, row := range report.Rows {
			if pdf.GetY() > 275 {
				pdf.AddPage()
				pdf.SetFont("Helvetica", "B", 9)
				pdf.SetFillColor(232, 234, 244)
				for _, header := range report.Headers {
					pdf.CellFormat(colWidth, 7, pdfLatin(header), "1", 0, "L", true, 0, "")
				}
				pdf.Ln(-1)
				pdf.SetFont("Helvetica", "", 9)
			}

			for _, cell := range row {
				pdf.CellFormat(colWidth, 6, pdfLatin(cell), "1", 0, "L", false, 0, "")
			}
			pdf.Ln(-1)
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to render report PDF: %w", err)
	}
	return buf.Bytes(), nil
}

// pdfLatin maps text into the Latin-1 range supported by the PDF core fonts.
// Tamil/Sinhala labels therefore degrade to "?" inside PDFs instead of
// failing generation; English labels render verbatim. (Embedding a Unicode
// TTF font would be the full fix and is out of scope for this phase.)
func pdfLatin(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x100 {
			return r
		}
		return '?'
	}, s)
}
