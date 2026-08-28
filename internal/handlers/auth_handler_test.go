package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLogoutRedirectsBrowserRequestsAndClearsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAuthHandler(nil, nil, nil, nil, false)
	router.POST("/logout", handler.Logout)

	request := httptest.NewRequest(http.MethodPost, "/logout", nil)
	request.Header.Set("Accept", "text/html")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("Logout status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	if location := response.Header().Get("Location"); location != "/login" {
		t.Fatalf("Logout location = %q, want %q", location, "/login")
	}
	if cookie := response.Header().Get("Set-Cookie"); cookie == "" {
		t.Fatal("Logout did not clear the session cookie")
	}
}
