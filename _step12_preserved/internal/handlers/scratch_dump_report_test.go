package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestScratchDumpReportPage dumps the rendered report center body for diagnosis.
func TestScratchDumpReportPage(t *testing.T) {
	router := reportPageTestDB(t)

	router.Use(func(c *gin.Context) {
		c.Next()
		for _, e := range c.Errors {
			t.Logf("CTX ERROR: %v", e)
		}
	})

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/reports/dashboard/page", nil))

	t.Logf("status=%d content-type=%q bodylen=%d", w.Code, w.Header().Get("Content-Type"), w.Body.Len())
	if err := os.WriteFile("../../report_dump.html", w.Body.Bytes(), 0o644); err != nil {
		t.Fatalf("write dump: %v", err)
	}
}
