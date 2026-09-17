package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/routes"
)

func TestI18nEndpoint_ReturnsDictionaryForEachLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.RegisterI18nRoutes(router)

	cases := []struct {
		name      string
		lang      string
		probeKey  string
		keySubstr string
	}{
		{"english", "en", "login.welcome", "Welcome"},
		{"tamil", "ta", "login.welcome", ""},
		{"sinhala", "si", "login.welcome", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/i18n/"+tc.lang, nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			var dict map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &dict); err != nil {
				t.Fatalf("response for %q is not a JSON dictionary: %v", tc.lang, err)
			}
			val, ok := dict[tc.probeKey]
			if !ok || strings.TrimSpace(val) == "" {
				t.Errorf("dictionary for %q missing/blank key %q", tc.lang, tc.probeKey)
			}
			if tc.keySubstr != "" && !strings.Contains(val, tc.keySubstr) {
				t.Errorf("value for %q should contain %q, got %q", tc.probeKey, tc.keySubstr, val)
			}
		})
	}
}

func TestI18nEndpoint_UnsupportedLanguageFallsBackToEnglish(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.RegisterI18nRoutes(router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/i18n/xx", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (English fallback)", w.Code)
	}
	var dict map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &dict); err != nil {
		t.Fatalf("response is not a JSON dictionary: %v", err)
	}
	if val, ok := dict["login.welcome"]; !ok || !strings.Contains(val, "Welcome") {
		t.Errorf("expected English fallback value for unsupported language, got %q", val)
	}
}

func TestI18nEndpoint_CacheControlHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes.RegisterI18nRoutes(router)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/i18n/en", nil)
	router.ServeHTTP(w, req)

	if !strings.Contains(w.Header().Get("Cache-Control"), "max-age") {
		t.Error("i18n responses should carry a shared cache header")
	}
}
