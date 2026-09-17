package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/i18n"
)

// RegisterI18nRoutes exposes the canonical translation dictionary over HTTP
// for public pages (login, forgot-password, ...). Those pages render before a
// user session exists, so the browser's language preference cannot be resolved
// server-side; the client fetches the dictionary for its detected language.
//
// This endpoint is a thin adapter over the single canonical i18n.Dictionary —
// it does NOT define a second translation source. The same data backs the
// server-rendered window.PWAMS_I18N injection (handlers.PageData) and this
// API. Supported languages are en / ta / si; an unsupported language falls
// back to English, exactly matching the i18n package's normalization contract.
// The endpoint is intentionally public so the unauthenticated login page can
// localise itself.
func RegisterI18nRoutes(router *gin.Engine) {
	router.GET("/api/i18n/:lang", func(c *gin.Context) {
		lang := i18n.EffectiveLanguage(c.Param("lang"))
		dict := i18n.Dictionary(lang)

		out := make(gin.H, len(dict))
		for key, value := range dict {
			out[key] = value
		}

		// The dictionary is read-only application data; a short shared cache
		// avoids a re-serialization on every page load.
		c.Header("Cache-Control", "public, max-age=300")
		c.JSON(http.StatusOK, out)
	})
}
