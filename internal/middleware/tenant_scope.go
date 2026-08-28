package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WithTenantScope returns a GORM Scope that filters queries by the
// authenticated user's tenant_id.  If the user has no tenant the
// scope is a no-op (single-tenant deployment).
func WithTenantScope(db *gorm.DB, c *gin.Context) *gorm.DB {
	value, exists := c.Get("current_user")
	if !exists {
		return db
	}

	type tenantProvider interface {
		GetTenantID() *uuid.UUID
	}

	tp, ok := value.(tenantProvider)
	if !ok {
		return db
	}

	tenantID := tp.GetTenantID()
	if tenantID == nil {
		return db
	}

	return db.Where("tenant_id = ?", tenantID)
}

// EnforceTenantID is a BeforeCreate/BeforeUpdate hook that stamps
// the tenant_id from the context into the record.
func EnforceTenantID(db *gorm.DB) {
	value, exists := db.Get("current_user")
	if !exists {
		return
	}

	type tenantProvider interface {
		GetTenantID() *uuid.UUID
	}

	tp, ok := value.(tenantProvider)
	if !ok {
		return
	}

	tenantID := tp.GetTenantID()
	if tenantID == nil {
		return
	}

	db.Statement.SetColumn("tenant_id", tenantID)
}
