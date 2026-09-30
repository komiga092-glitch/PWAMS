// Command qaseed creates (or refreshes) one QA login per application role so
// that role-based test cases can be executed against a local environment.
//
// It is idempotent: existing qa_* users get their password reset to the known
// QA password and are re-activated; missing users are created. Roles are
// seeded via database.SeedDefaultRoles first. Failures (e.g. the
// one-active-Manager rule) are reported per role without aborting the rest.
//
// Usage: go run ./cmd/qaseed
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/joho/godotenv"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
	"github.com/komiga092-glitch/pwams/internal/utils"
)

// qaPassword is the known password for every seeded qa_* account.
const qaPassword = "QaPassw0rd!2026"

var qaRoles = []string{
	models.RoleSuperAdmin,
	models.RoleAdmin,
	models.RoleManager,
	models.RoleStaff,
	models.RoleVolunteer,
	models.RoleDonor,
	models.RoleBeneficiary,
	models.RoleStudent,
}

func qaUsername(roleName string) string {
	return "qa_" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(roleName), " ", "_"))
}

func main() {
	_ = godotenv.Load() // repo-root .env, same convention as the app

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}

	if err := database.SeedDefaultRoles(db); err != nil {
		log.Fatalf("seed roles: %v", err)
	}

	hash, err := utils.HashPassword(qaPassword)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	for _, roleName := range qaRoles {
		var role models.Role
		if err := db.Where("name = ?", roleName).First(&role).Error; err != nil {
			fmt.Printf("ROLE-MISSING\t%s\t%v\n", roleName, err)
			continue
		}

		username := qaUsername(roleName)
		email := username + "@qa.local"

		var existing models.User
		err := db.Where("username = ?", username).First(&existing).Error
		if err == nil {
			existing.PasswordHash = hash
			existing.Status = models.UserStatusActive
			existing.RoleID = role.ID
			existing.FailedLoginAttempts = 0
			existing.LockedUntil = nil
			if err := db.Save(&existing).Error; err != nil {
				fmt.Printf("REFRESH-FAILED\t%s\t%v\n", username, err)
				continue
			}
			fmt.Printf("REFRESHED\t%s\t%s\t%s\n", username, roleName, email)
			continue
		}

		user := models.User{
			Username:     username,
			Email:        email,
			PasswordHash: hash,
			RoleID:       role.ID,
			Status:       models.UserStatusActive,
		}
		if err := db.Create(&user).Error; err != nil {
			fmt.Printf("CREATE-FAILED\t%s\t%s\t%v\n", username, roleName, err)
			continue
		}
		fmt.Printf("CREATED\t%s\t%s\t%s\n", username, roleName, email)
	}

	fmt.Printf("DONE\tpassword=%s\n", qaPassword)
}
