package config

import (
	"encoding/base64"
	"fmt"
	"os"
)

type Config struct {
	Environment string // "dev" or "prod"
	AppURL      string // e.g. "http://localhost:8080"
	Addr        string // e.g. ":8080"

	DatabaseURL string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCLogoutURL    string // optional: omit to skip provider-side logout

	SecretKey []byte // 32-byte key for token encryption and CSRF protection
}

func (c *Config) IsDev() bool {
	return c.Environment == "dev"
}

func Load() *Config {
	secretKeyB64 := requireEnv("SECRET_KEY")
	secretKey, err := base64.StdEncoding.DecodeString(secretKeyB64)
	if err != nil {
		panic(fmt.Sprintf("SECRET_KEY is not valid base64: %v", err))
	}
	if len(secretKey) != 32 {
		panic(fmt.Sprintf("SECRET_KEY must decode to exactly 32 bytes, got %d", len(secretKey)))
	}

	return &Config{
		Environment:      getEnv("ENVIRONMENT", "prod"),
		AppURL:           requireEnv("APP_URL"),
		Addr:             getEnv("ADDR", ":8080"),
		DatabaseURL:      requireEnv("DATABASE_URL"),
		OIDCIssuerURL:    requireEnv("OIDC_ISSUER_URL"),
		OIDCClientID:     requireEnv("OIDC_CLIENT_ID"),
		OIDCClientSecret: requireEnv("OIDC_CLIENT_SECRET"),
		OIDCLogoutURL:    getEnv("OIDC_LOGOUT_URL", ""),
		SecretKey:        secretKey,
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
