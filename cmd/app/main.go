package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"github.com/antonkarounis/stoic/internal/app"
	"github.com/antonkarounis/stoic/internal/platform/auth"
	"github.com/antonkarounis/stoic/internal/platform/config"
	"github.com/antonkarounis/stoic/internal/platform/db"
	"github.com/antonkarounis/stoic/internal/platform/db/gen"
)

func main() {
	// G2: Load .env file if present (not required in container deployments)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Load()

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

	// Initialize auth service (OIDC provider + DB access)
	authService, err := auth.NewAuthService(ctx, cfg, queries)
	if err != nil {
		log.Fatalf("Failed to initialize auth: %v", err)
	}

	// Periodically clean up expired sessions
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := queries.DeleteExpiredSessions(ctx); err != nil {
					log.Printf("Failed to cleanup expired sessions: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Set up router and middleware
	r := mux.NewRouter()
	r.Use(noCache)
	r.Use(securityHeaders)
	r.Use(func(next http.Handler) http.Handler {
		return gorillaHandlers.LoggingHandler(os.Stdout, next)
	})
	r.Use(gorillaHandlers.RecoveryHandler())
	cop := http.NewCrossOriginProtection()
	r.Use(func(next http.Handler) http.Handler { return cop.Handler(next) })
	r.Use(authService.OptionalAuth)

	// Register application routes
	app.RegisterRoutes(r, cfg, authService)

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

// B5: noCache sets cache-busting headers for dynamic routes only.
// Static file routes (if added later) should be excluded.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		next.ServeHTTP(w, r)
	})
}

// S7: securityHeaders adds standard security headers to all responses.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' https://unpkg.com; style-src 'self' https://cdn.jsdelivr.net; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
