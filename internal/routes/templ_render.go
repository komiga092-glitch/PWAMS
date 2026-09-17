package routes

import (
	"context"
	"io"

	"github.com/gin-gonic/gin"
)

// Render writes a templ component to the Gin response writer.
func Render(c *gin.Context, code int, templFn func(w io.Writer) error) {
	c.Status(code)
	c.Header("Content-Type", "text/html; charset=utf-8")

	if err := templFn(c.Writer); err != nil {
		c.Error(err)
		c.Abort()
	}
}

// RenderError renders an error page.
func RenderError(c *gin.Context, code int, message string) {
	lang, ok := c.Get("lang")
	if !ok {
		lang = "en"
	}
	c.HTML(code, "base", gin.H{
		"page_template": "error_content",
		"title":         "Error",
		"error_code":    code,
		"error_message": message,
		"Lang":          lang,
	})
}

// renderTemplComponent is a helper that writes a templ component.
// Usage in handlers:
//
//	routes.Render(c, http.StatusOK, func(w io.Writer) error {
//	    return components.DashboardPage(stats, "").Render(context.Background(), w)
//	})
func renderTemplComponent(c *gin.Context, code int, renderer func(ctx context.Context, w io.Writer) error) {
	c.Status(code)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := renderer(context.Background(), c.Writer); err != nil {
		c.Error(err)
	}
}
