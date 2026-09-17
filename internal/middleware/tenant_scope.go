package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TenantProvider is implemented by authenticated principals that are
// bound to a tenant (NGO). models.User does not implement it in the
// current single-tenant deployment, so every scope below is a no-op
// for plain users; any future tenant-bound principal type is scoped
// automatically through this contract.
type TenantProvider interface {
	GetTenantID() *uuid.UUID
}

// ContextTenantID extracts the authenticated principal's tenant id
// using exactly the same contract as WithTenantScope. It returns nil
// when the principal is absent or not tenant-bound (single-tenant
// deployment), so callers can pass the result straight into
// tenant-scoped queries.
func ContextTenantID(c *gin.Context) *uuid.UUID {
	value, exists := c.Get("current_user")
	if !exists {
		return nil
	}

	tp, ok := value.(TenantProvider)
	if !ok {
		return nil
	}

	return tp.GetTenantID()
}

// WithTenantScope returns a GORM Scope that filters queries by the
// authenticated user's tenant_id.  If the user has no tenant the
// scope is a no-op (single-tenant deployment).
func WithTenantScope(db *gorm.DB, c *gin.Context) *gorm.DB {
	tenantID := ContextTenantID(c)
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

	tp, ok := value.(TenantProvider)
	if !ok {
		return
	}

	tenantID := tp.GetTenantID()
	if tenantID == nil {
		return
	}

	db.Statement.SetColumn("tenant_id", tenantID)
}
