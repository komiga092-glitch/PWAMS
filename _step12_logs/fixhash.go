package main

import (
	"fmt"
	"os"

	"github.com/komiga092-glitch/pwams/internal/config"
	"github.com/komiga092-glitch/pwams/internal/database"
	"github.com/komiga092-glitch/pwams/internal/models"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("config error:", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		fmt.Println("db error:", err)
		os.Exit(1)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	var user models.User
	if err := db.Where("username = ?", "komikukan").First(&user).Error; err != nil {
		fmt.Println("find error:", err)
		os.Exit(1)
	}

	if err := db.Model(&user).Update("password_hash", "$2a$10$j01VmM50Gt5hfFTiX4Sp/erAI.7nb0XvXsSO10WDVttT4maLJvKHy").Error; err != nil {
		fmt.Println("update error:", err)
		os.Exit(1)
	}

	fmt.Println("Password hash updated for user:", user.Username)
}
