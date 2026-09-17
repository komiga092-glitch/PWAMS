package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/komiga092-glitch/pwams/internal/constants"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/services"
)

// permissionContextKey is the gin-context key under which the authenticated
// user's effective permission set is cached for the lifetime of a single
// request.
const permissionContextKey = "permissions"

// LoadPermissions populates the gin context with the authenticated user's
// effective permission set. It should be used once per route group, right
// after RequireAuth. Individual RequirePermission calls then read from the
// cache, avoiding repeated database queries for the same request.
func LoadPermissions(permissionService *services.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// A nil service is a programming error, but aborting every request
		// would take the whole application down; continue the chain instead
		// so RequirePermission falls back to its own guard.
		if permissionService == nil {
			c.Next()
			return
		}

		currentUser, ok := c.Get("current_user")
		if !ok {
			c.Next()
			return
		}

		user, ok := currentUser.(*models.User)
		if !ok {
			c.Next()
			return
		}

		perms, err := permissionService.GetUserPermissions(user)
		if err != nil {
			// Don't block the request — log and continue without a cached
			// set so RequirePermission can fall back to its own lookup.
			c.Next()
			return
		}

		c.Set(permissionContextKey, perms)
		c.Next()
	}
}

// RequirePermission returns a middleware that rejects requests for which the
// authenticated user lacks the specified permission. Permissions are read
// from the per-request cache populated by LoadPermissions; if the cache is
// absent (LoadPermissions not used), the service is called directly.
func RequirePermission(
	permissionService *services.PermissionService,
	permission string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		has, err := hasPermission(c, permissionService, permission)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Permission check failed",
			})
			return
		}

		if !has {
			abortForbidden(c)
			return
		}

		c.Next()
	}
}

// RequireAllPermissions returns a middleware that requires ALL of the listed
// permissions.
func RequireAllPermissions(
	permissionService *services.PermissionService,
	permissions ...string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		has, missing, err := hasAllPermissions(c, permissionService, permissions)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Permission check failed",
			})
			return
		}

		if !has {
			_ = c
			_ = missing
			abortForbidden(c)
			return
		}

		c.Next()
	}
}

// hasPermission checks a single permission against the cached set or via
// the service fallback. A nil service with no cached set fails closed: the
// permission cannot be proven, so it is denied (with an error so callers
// return a clean failure rather than panicking).
func hasPermission(
	c *gin.Context,
	permissionService *services.PermissionService,
	permission string,
) (bool, error) {
	user, ok := currentUser(c)
	if !ok {
		return false, nil
	}

	// Try the cached permission set first.
	if perms, exists := c.Get(permissionContextKey); exists {
		if permList, ok := perms.([]string); ok {
			return containsString(permList, permission), nil
		}
	}

	if permissionService == nil {
		// Fail closed: without a permission service no permission can be
		// proven. Deny with an error so the middleware surfaces a clean 500
		// (never a panic-based 500 and never an accidental allow).
		return false, errPermissionCheckFailed
	}

	// Fallback: query the service directly.
	return permissionService.HasPermission(user, permission)
}

// errPermissionCheckFailed reports an unserviceable permission check.
var errPermissionCheckFailed = errors.New("permission check failed: permission service unavailable")

// hasAllPermissions checks multiple permissions.
func hasAllPermissions(
	c *gin.Context,
	permissionService *services.PermissionService,
	permissions []string,
) (bool, string, error) {
	user, ok := currentUser(c)
	if !ok {
		return false, "", nil
	}

	var permList []string

	if cached, exists := c.Get(permissionContextKey); exists {
		if p, ok := cached.([]string); ok {
			permList = p
		}
	}

	if permList == nil {
		var err error
		permList, err = permissionService.GetUserPermissions(user)
		if err != nil {
			return false, "", err
		}
		c.Set(permissionContextKey, permList)
	}

	for _, required := range permissions {
		if !containsString(permList, required) {
			return false, required, nil
		}
	}

	return true, "", nil
}

// currentUser extracts the authenticated user from the gin context.
func currentUser(c *gin.Context) (*models.User, bool) {
	value, exists := c.Get("current_user")
	if !exists {
		return nil, false
	}

	user, ok := value.(*models.User)
	if !ok {
		return nil, false
	}

	return user, true
}

// hasPermission is a convenience function for handlers that need to check
// a permission at the handler level (e.g. conditional rendering of data).
//
// HasPermissionFromContext returns whether the authenticated user (cached in
// the context) has the given permission.
func HasPermissionFromContext(c *gin.Context, permission string) bool {
	perms, exists := c.Get(permissionContextKey)
	if !exists {
		return false
	}

	permList, ok := perms.([]string)
	if !ok {
		return false
	}

	return containsString(permList, permission)
}

// CachedPermissions returns the effective permission set cached for the
// current request by LoadPermissions. It reports false when no cached set is
// available. Handlers use this to share the already-loaded permission set
// with template rendering (e.g. permission-aware navigation) without issuing
// any additional database queries.
func CachedPermissions(c *gin.Context) ([]string, bool) {
	value, exists := c.Get(permissionContextKey)
	if !exists {
		return nil, false
	}

	permList, ok := value.([]string)
	if !ok {
		return nil, false
	}

	return permList, true
}

func containsString(slice []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, s := range slice {
		if strings.ToLower(strings.TrimSpace(s)) == target {
			return true
		}
	}
	return false
}

// ensure constants import is used (avoids unused import in some builds).
var _ = constants.ErrAuthenticationRequired
