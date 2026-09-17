package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/komiga092-glitch/pwams/internal/i18n"
)

const localeCookieName = "pwams_lang"

// ResolveLocale determines the UI language for this request — English,
// Tamil, or Sinhala (SRS section 32) — from a `?lang=` query param
// (which also persists the choice in a cookie), else the pwams_lang
// cookie, else defaults to English. Stores it in context as "lang".
func ResolveLocale(secure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := c.Query("lang")
		if lang != "" && i18n.IsSupported(lang) {
			c.SetCookie(localeCookieName, lang, 60*60*24*365, "/", "", secure, true)
		} else {
			cookieLang, err := c.Cookie(localeCookieName)
			if err == nil && i18n.IsSupported(cookieLang) {
				lang = cookieLang
			} else {
				lang = "en"
			}
		}
		c.Set("lang", lang)
		c.Next()
	}
}
