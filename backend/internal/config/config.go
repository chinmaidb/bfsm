package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Address            string
	Environment        string
	DatabaseURL        string
	WebDir             string
	JWTIssuer          string
	JWTPrivateKey      string
	JWTLifetimeMinutes string
	CORSAllowedOrigins string
	SuperAdminUsername string
	SuperAdminPassword string
	S3Region           string
	S3Bucket           string
	S3KeyPrefix        string
	S3Endpoint         string
	S3UsePathStyle     bool
}

func Load() (Config, error) {
	cfg := Config{
		Address:            env("API_ADDRESS", ":8080"),
		Environment:        env("APP_ENV", "development"),
		DatabaseURL:        env("DATABASE_URL", ""),
		WebDir:             strings.TrimSpace(os.Getenv("WEB_DIR")),
		JWTIssuer:          env("JWT_ISSUER", "benefit-scheme-api"),
		JWTPrivateKey:      env("JWT_PRIVATE_KEY", ""),
		JWTLifetimeMinutes: env("JWT_LIFETIME_MINUTES", "15"),
		CORSAllowedOrigins: env("CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		SuperAdminUsername: strings.TrimSpace(os.Getenv("SUPER_ADMIN_USERNAME")),
		SuperAdminPassword: os.Getenv("SUPER_ADMIN_PASSWORD"),
		S3Region:           env("S3_REGION", ""),
		S3Bucket:           env("S3_BUCKET", ""),
		S3KeyPrefix:        strings.Trim(env("S3_KEY_PREFIX", "bfsm"), "/"),
		S3Endpoint:         env("S3_ENDPOINT", ""),
		S3UsePathStyle:     strings.EqualFold(env("S3_USE_PATH_STYLE", "false"), "true"),
	}
	if !strings.HasPrefix(cfg.Address, ":") && !strings.Contains(cfg.Address, ":") {
		return Config{}, fmt.Errorf("API_ADDRESS must include a port, got %q", cfg.Address)
	}
	if (cfg.SuperAdminUsername == "") != (cfg.SuperAdminPassword == "") {
		return Config{}, fmt.Errorf("SUPER_ADMIN_USERNAME and SUPER_ADMIN_PASSWORD must be configured together")
	}
	if cfg.Environment == "production" && (cfg.SuperAdminUsername == "" || len(cfg.SuperAdminPassword) < 16) {
		return Config{}, fmt.Errorf("production requires SUPER_ADMIN_USERNAME and a SUPER_ADMIN_PASSWORD of at least 16 characters")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
