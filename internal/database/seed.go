package database

import (
	"fmt"

	"github.com/komiga092-glitch/pwams/internal/models"
	"gorm.io/gorm"
)

func SeedDefaultRoles(db *gorm.DB) error {
	defaultRoles := []models.Role{
		{
			Name:        models.RoleSuperAdmin,
			Description: "Full system administration and high-level approval access.",
		},
		{
			Name:        models.RoleAdmin,
			Description: "Manages users, daily operations, approvals, and reports.",
		},
		{
			Name:        models.RoleStaff,
			Description: "Registers beneficiaries and manages aid, loans, and cases.",
		},
		{
			Name:        models.RoleVolunteer,
			Description: "Performs limited field data collection and support activities.",
		},
		{
			Name:        models.RoleDonor,
			Description: "Views personal donation details and contribution history.",
		},
		{
			Name:        models.RoleBeneficiary,
			Description: "Applies for aid or loans and views personal request status.",
		},
	}

	for _, role := range defaultRoles {
		result := db.Where("name = ?", role.Name).FirstOrCreate(&role)
		if result.Error != nil {
			return fmt.Errorf("failed to seed role %s: %w", role.Name, result.Error)
		}
	}

	return nil
}
