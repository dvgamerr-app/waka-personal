package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	LogLevel  string
	LogFormat string

	HTTPPort                string
	DatabaseURL             string
	AppTimezone             string
	AppAPIKey               string
	KeystrokeTimeoutMinutes int
	WritesOnly              bool
	CORSAllowOrigins        []string
	MigrationDir            string
	GooseTable              string

	ProfileUsername    string
	ProfileDisplayName string
	ProfileFullName    string
	ProfileEmail       string
	ProfilePhotoURL    string
	ProfileProfileURL  string
}

func Load() *Config {
	return &Config{
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),

		HTTPPort:                getEnv("PORT", "8080"),
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/waka_personal?sslmode=disable"),
		AppTimezone:             getEnv("APP_TIMEZONE", "UTC"),
		AppAPIKey:               getEnv("APP_API_KEY", ""),
		KeystrokeTimeoutMinutes: getEnvInt("KEYSTROKE_TIMEOUT_MINUTES", 15),
		WritesOnly:              getEnvBool("WRITES_ONLY", false),
		CORSAllowOrigins:        getEnvList("CORS_ALLOW_ORIGINS", []string{"*"}),
		MigrationDir:            getEnv("MIGRATION_DIR", "db/migrations"),
		GooseTable:              getEnv("GOOSE_TABLE", "goose_db_version"),

		ProfileUsername:    getEnv("PROFILE_USERNAME", "local"),
		ProfileDisplayName: getEnv("PROFILE_DISPLAY_NAME", "Local User"),
		ProfileFullName:    getEnv("PROFILE_FULL_NAME", "Local User"),
		ProfileEmail:       getEnv("PROFILE_EMAIL", ""),
		ProfilePhotoURL:    getEnv("PROFILE_PHOTO_URL", ""),
		ProfileProfileURL:  getEnv("PROFILE_PROFILE_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func getEnvList(key string, fallback []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
