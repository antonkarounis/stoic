package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antonkarounis/stoic/internal/adapters/db"
	"github.com/antonkarounis/stoic/internal/adapters/db/gen"
	views "github.com/antonkarounis/stoic/internal/adapters/web"
	"github.com/antonkarounis/stoic/internal/domain/models"
	"github.com/antonkarounis/stoic/internal/domain/ports"
	"github.com/antonkarounis/stoic/internal/domain/services"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// G2: Load .env file if present (not required in container deployments)
	_ = godotenv.Load()

	// Load configurations
	cfg := LoadConfig()

	// Initialize structured logger before any other work
	ConfigureLogging(cfg.Environment == "dev")

	// Run migrations (using embedded SQL files, with advisory lock for safe multi-instance startup)
	if err := db.Migrate(ctx, db.PlatformMigrations, "migrations", cfg.DatabaseURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Create connection pool
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize SQLC queries
	queries := gen.New(pool)

	sessionRepository := db.NewSessionRepository(ctx, queries)
	identityRepository := db.NewIdentityRepository(queries)

	userRepository := db.NewUserRepository(queries)

	userService := services.NewUserService(userRepository)

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
	authService, err := views.NewAuthService(ctx, authCfg, sessionRepository, identityRepository)
	if err != nil {
		slog.Error("failed to initialize auth", "error", err)
		os.Exit(1)
	}

	authService.SetFirstLoginHook(func(ctx context.Context, email, name string) (models.UserID, error) {
		user, err := userService.Register(ctx, ports.RegisterInput{Email: email, Name: name})
		if err != nil {
			return "", err
		}
		return user.ID, nil
	})

	// Stub: sync email/name changes from OIDC provider on each subsequent login
	authService.SetOnLoginHook(func(ctx context.Context, userID models.UserID, email, name string) error {
		return nil
	})

	// Set up router and middleware
	r := mux.NewRouter()

	isDev := cfg.Environment == "dev"
	views.RegisterRoutes(r, *authService, userRepository, pool, isDev)

	// Start HTTP server with timeouts
	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// A2: Graceful shutdown on SIGINT/SIGTERM
	go func() {
		slog.Info("starting HTTP server", "addr", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server exited gracefully")
}

func ConfigureLogging(isDev bool) {
	logLevel := slog.LevelInfo
	var logHandler slog.Handler
	if isDev {
		logLevel = slog.LevelDebug
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	slog.SetDefault(slog.New(logHandler))
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

type noopEmailSender struct{}

func (noopEmailSender) SendInvite(ctx context.Context, toEmail string, token string) error {
	return nil
}
