package models

// Password setup mode for managed-account creation. The only supported mode
// is a hashed temporary password chosen by the operator: the account is
// created Active and the user can log in immediately. The legacy
// activation-link mode (which created Pending accounts) has been removed.
const (
	// PasswordSetupTemporaryPassword hashes a temporary password chosen by
	// the operator and activates the account so the user can log in.
	PasswordSetupTemporaryPassword = "temporary_password"
)

// CreateManagedAccountRequest is the payload for the role-scoped account
// creation endpoints (/admins, /partners, /staff, /volunteers). The Role
// field is IGNORED by the backend: the target role is forced server-side
// from the endpoint itself so a malicious client cannot escalate.
type CreateManagedAccountRequest struct {
	Username     string `json:"username" binding:"omitempty,min=3,max=50"`
	Email        string `json:"email" binding:"required,email,max=100"`
	FullName     string `json:"full_name" binding:"required,max=100"`
	Phone        string `json:"phone" binding:"omitempty,max=30"`
	Language     string `json:"language" binding:"omitempty,max=30"`
	Role         string `json:"role" binding:"omitempty,max=50"`
	TempPassword string `json:"temporary_password" binding:"required,min=8,max=72"`
}

// UpdateManagedAccountRequest is the payload for the role-scoped account
// edit endpoints. It intentionally carries NO role field: managed accounts
// can never have their role changed through these endpoints, which removes
// the privilege-escalation path entirely (role changes only happen through
// the generic endpoint, guarded by the role hierarchy).
type UpdateManagedAccountRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email,max=100"`
	FullName string `json:"full_name" binding:"required,max=100"`
	Phone    string `json:"phone" binding:"omitempty,max=30"`
	Language string `json:"language" binding:"omitempty,max=30"`
	Status   string `json:"status" binding:"required,oneof=Active Disabled Locked"`
}
