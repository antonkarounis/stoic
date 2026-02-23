package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antonkarounis/balance/internal/adapters/db"
	"github.com/antonkarounis/balance/internal/adapters/db/gen"
	views "github.com/antonkarounis/balance/internal/adapters/web"
	"github.com/antonkarounis/balance/internal/ports"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// G2: Load .env file if present (not required in container deployments)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := LoadConfig()

	// Run migrations (using embedded SQL files, with advisory lock for safe multi-instance startup)
	if err := db.Migrate(ctx, db.PlatformMigrations, "migrations", cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create connection pool
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to create database pool: %v", err)
	}
	defer pool.Close()

	// Initialize SQLC queries
	queries := gen.New(pool)

	sessionRepository := db.NewSessionRepository(ctx, queries)
	userRepository := db.NewUserRepository(queries)

	// Create auth config from infrastructure config
	authCfg := &views.AuthConfig{
		OIDCIssuerURL:    cfg.OIDCIssuerURL,
		OIDCClientID:     cfg.OIDCClientID,
		OIDCClientSecret: cfg.OIDCClientSecret,
		OIDCLogoutURL:    cfg.OIDCLogoutURL,
		AppURL:           cfg.AppURL,
		SecretKey:        cfg.SecretKey,
		IsDev:            cfg.Environment == "dev",
	}

	// Initialize auth service (OIDC provider + DB access)
	authService, err := views.NewAuthService(ctx, authCfg, sessionRepository, userRepository)
	if err != nil {
		log.Fatalf("Failed to initialize auth: %v", err)
	}

	// Set up router and middleware
	r := mux.NewRouter()

	// Register application routes
	views.RegisterRoutes(r, *authService)

	// Start HTTP server
	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	// A2: Graceful shutdown on SIGINT/SIGTERM
	go func() {
		log.Printf("Starting HTTP Server. Listening at %q", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited gracefully")
}

func LoadConfig() *ports.Config {
	secretKeyB64 := requireEnv("SECRET_KEY")
	secretKey, err := base64.StdEncoding.DecodeString(secretKeyB64)
	if err != nil {
		panic(fmt.Sprintf("SECRET_KEY is not valid base64: %v", err))
	}
	if len(secretKey) != 32 {
		panic(fmt.Sprintf("SECRET_KEY must decode to exactly 32 bytes, got %d", len(secretKey)))
	}

	return &ports.Config{
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
