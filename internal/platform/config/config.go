package config

import (
	"fmt"
	"os"
)

type Config struct {
	Environment string // "dev" or "prod"
	AppURL      string // e.g. "http://localhost:8080"
	Addr        string // e.g. ":8080"

	DatabaseURL    string
	MigrationPaths []string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCLogoutURL    string // optional
}

func Load() *Config {
	return &Config{
		Environment:      getEnv("ENVIRONMENT", "prod"),
		AppURL:           requireEnv("APP_URL"),
		Addr:             getEnv("ADDR", ":8080"),
		DatabaseURL:      requireEnv("DATABASE_URL"),
		MigrationPaths:   []string{"file://internal/platform/db/migrations"},
		OIDCIssuerURL:    requireEnv("OIDC_ISSUER_URL"),
		OIDCClientID:     requireEnv("OIDC_CLIENT_ID"),
		OIDCClientSecret: requireEnv("OIDC_CLIENT_SECRET"),
		OIDCLogoutURL:    requireEnv("OIDC_LOGOUT_URL"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}
