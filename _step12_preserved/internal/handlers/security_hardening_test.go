package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestEscapeCSVField_NeutralizesFormulaInjection proves cells that a
// spreadsheet could interpret as formulas (= + - @) are prefixed with a
// single quote (OWASP CSV injection guidance) while benign values pass
// through untouched. Leading whitespace before a formula character is also
// neutralized because Excel and LibreOffice ignore it when deciding whether
// a cell is a formula.
func TestEscapeCSVField_NeutralizesFormulaInjection(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty stays empty", input: "", want: ""},
		{name: "plain text untouched", input: "John Doe", want: "John Doe"},
		{name: "digit stays numeric", input: "12345", want: "12345"},
		{name: "leading equals", input: "=SUM(1+1)", want: "'=SUM(1+1)"},
		{name: "leading plus", input: "+44 7700 900123", want: "'+44 7700 900123"},
		{name: "leading minus", input: "-1+1", want: "'-1+1"},
		{name: "leading at", input: "@cmd", want: "'@cmd"},
		{name: "tab before formula", input: "\t=HYPERLINK(\"x\")", want: "'\t=HYPERLINK(\"x\")"},
		{name: "spaces before formula", input: "  =1+1", want: "'  =1+1"},
		{name: "existing apostrophe preserved", input: "'=already-safe", want: "'=already-safe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeCSVField(tt.input); got != tt.want {
				t.Fatalf("escapeCSVField(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSanitizeFilenameForHeader proves CR/LF and all other HTTP control
// characters are stripped before a user-derived filename is placed in a
// Content-Disposition header, and the fallback name prevents empty headers.
func TestSanitizeFilenameForHeader(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "clean name preserved", in: "photo.jpg", want: "photo.jpg"},
		{name: "CRLF stripped", in: "photo\r\n.jpg", want: "photo.jpg"},
		{name: "bare CR and LF stripped", in: "a\rb\nc.jpg", want: "abc.jpg"},
		{name: "tab stripped", in: "a\tb.jpg", want: "ab.jpg"},
		{name: "DEL stripped", in: "bad\x7fname.jpg", want: "badname.jpg"},
		{name: "controls only falls back", in: "\r\n\t\x01", want: "file"},
		{name: "non-ASCII preserved", in: "caf\xc3\xa9.jpg", want: "caf\xc3\xa9.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFilenameForHeader(tt.in); got != tt.want {
				t.Fatalf("sanitizeFilenameForHeader(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestFileAttachment_ContentDispositionHasNoCRLF proves the sanitizer feeds
// Gin a filename that cannot smuggle response headers through the
// Content-Disposition value.
func TestFileAttachment_ContentDispositionHasNoCRLF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/files/payload.txt", nil)

	payload := t.TempDir() + string(os.PathSeparator) + "payload.txt"
	if err := os.WriteFile(payload, []byte("attachment payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	c.FileAttachment(payload, sanitizeFilenameForHeader("report\r\nSet-Cookie: x=y.txt"))
	got := w.Header().Get("Content-Disposition")
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("Content-Disposition contains raw CR/LF: %q", got)
	}
	if !strings.Contains(got, "attachment") {
		t.Fatalf("Content-Disposition missing attachment directive: %q", got)
	}
}

// TestIsSafeRedirectURL pins the local-path allow/deny policy: only
// same-origin relative paths may be used as 30x targets. Absolute URLs,
// protocol-relative URLs, javascript: URLs and backslash variants are
// rejected.
func TestIsSafeRedirectURL(t *testing.T) {
	safe := []string{
		"/",
		"/dashboard",
		"/profile?updated=1",
		"/my/aid?submitted=1#results",
		"/%2Fencoded%5Cfoo",
	}
	for _, target := range safe {
		if !isSafeRedirectURL(target) {
			t.Errorf("isSafeRedirectURL(%q) = false, want true", target)
		}
	}

	unsafe := []string{
		"",
		"dashboard",
		"https://evil.example/phish",
		"http://evil.example/",
		"//evil.example",
		"///evil.example",
		"javascript:alert(1)",
		"\\evil.example",
		" /dashboard",
	}
	for _, target := range unsafe {
		if isSafeRedirectURL(target) {
			t.Errorf("isSafeRedirectURL(%q) = true, want false", target)
		}
	}
}

// TestRedirectAfterLanguageChange_RejectsExternalReferer proves the language
// switch endpoint cannot be turned into an open redirect via the Referer
// header and falls back to the dashboard for off-site URLs.
func TestRedirectAfterLanguageChange_RejectsExternalReferer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/profile", nil)
	c.Request.Header.Set("Referer", "https://evil.example/phish")

	h := &UserHandler{}
	h.redirectAfterLanguageChange(c, false)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if got := w.Header().Get("Location"); got != "/dashboard" {
		t.Fatalf("Location = %q, want /dashboard", got)
	}
}

// TestRedirectAfterLanguageChange_KeepsLocalReferer proves same-origin
// relative Referer values keep working after the redirect hardening.
func TestRedirectAfterLanguageChange_KeepsLocalReferer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/profile", nil)
	c.Request.Header.Set("Referer", "/profile?updated=1")

	h := &UserHandler{}
	h.redirectAfterLanguageChange(c, false)

	if got := w.Header().Get("Location"); got != "/profile?updated=1" {
		t.Fatalf("Location = %q, want /profile?updated=1", got)
	}
}
