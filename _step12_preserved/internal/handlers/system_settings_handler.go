package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SystemSettingsHandler renders the read-only system settings page for the
// Super Admin. Step 2 only requires the navigation destination and a
// non-sensitive overview; the settings *editing* module belongs to a later
// step, so this handler accepts no mutations at all.
type SystemSettingsHandler struct {
	// settings holds non-sensitive configuration values prepared at startup
	// (never credentials, secrets, or connection strings with passwords).
	settings gin.H
}

func NewSystemSettingsHandler(settings gin.H) *SystemSettingsHandler {
	return &SystemSettingsHandler{settings: settings}
}

// Page renders the system settings overview.
func (h *SystemSettingsHandler) Page(c *gin.Context) {
	c.HTML(http.StatusOK, "base", PageData(c, gin.H{
		"title":           "System Settings",
		"page_template":   "system_settings_content",
		"system_settings": h.settings,
	}))
}
