package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppName string
	AppEnv  string
	AppPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	SuperAdminUsername string
	SuperAdminEmail    string
	SuperAdminPassword string

	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	// TrustedProxies lists the reverse proxy addresses (IPs or CIDRs) whose
	// X-Forwarded-For / X-Real-IP headers may be trusted. It is empty by
	// default: Gin is then configured with no trusted proxy, so the client
	// IP always comes from the TCP connection. This keeps IP based controls
	// such as the login rate limiter from being bypassed with a spoofed
	// X-Forwarded-For header (TRUSTED_PROXIES).
	TrustedProxies []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppName:    getEnv("APP_NAME", "PWAMS"),
		AppEnv:     getEnv("APP_ENV", "development"),
		AppPort:    getEnv("APP_PORT", "8080"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		DBSSLMode:  getEnv("DB_SSLMODE", "require"),

		SuperAdminUsername: os.Getenv("SUPER_ADMIN_USERNAME"),
		SuperAdminEmail:    os.Getenv("SUPER_ADMIN_EMAIL"),
		SuperAdminPassword: os.Getenv("SUPER_ADMIN_PASSWORD"),

		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     os.Getenv("SMTP_FROM"),

		TrustedProxies: parseTrustedProxies(os.Getenv("TRUSTED_PROXIES")),
	}

	if cfg.DBUser == "" || cfg.DBPassword == "" || cfg.DBName == "" {
		return nil, fmt.Errorf("required database configuration is missing")
	}
	if cfg.SuperAdminUsername == "" ||
		cfg.SuperAdminEmail == "" ||
		cfg.SuperAdminPassword == "" {
		return nil, fmt.Errorf("super admin configuration is missing")
	}

	return cfg, nil

}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}

// parseTrustedProxies splits a comma separated list of proxy IPs or CIDRs
// (TRUSTED_PROXIES) into a slice. Blank entries are ignored; an unset or
// blank value yields nil, which means "trust no proxy".
func parseTrustedProxies(raw string) []string {
	var proxies []string

	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			proxies = append(proxies, trimmed)
		}
	}

	return proxies
}
