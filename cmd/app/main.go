package main

import (
	"context"
	"log"
	"net/http"
	"os"
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

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
}

func main() {
	ctx := context.Background()
	cfg := config.Load()

	// Initialize OIDC provider
	if err := auth.Init(ctx, cfg); err != nil {
		log.Fatalf("Failed to initialize auth: %v", err)
	}

	// Run migrations
	for _, path := range cfg.MigrationPaths {
		if err := db.Migrate(ctx, path, cfg.DatabaseURL); err != nil {
			log.Fatalf("Failed to run migrations (%s): %v", path, err)
		}
	}

	// Create connection pool
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to create database pool: %v", err)
	}
	defer pool.Close()

	// Initialize SQLC queries and inject into auth layer
	queries := gen.New(pool)
	auth.InitDB(queries)

	// Periodically clean up expired sessions
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := queries.DeleteExpiredSessions(context.Background()); err != nil {
				log.Printf("Failed to cleanup expired sessions: %v", err)
			}
		}
	}()

	// Set up router and middleware
	r := mux.NewRouter()
	r.Use(noCache)
	r.Use(func(next http.Handler) http.Handler {
		return gorillaHandlers.LoggingHandler(os.Stdout, next)
	})
	r.Use(gorillaHandlers.RecoveryHandler())
	r.Use(auth.OptionalAuth)

	// Register application routes
	app.RegisterRoutes(r, cfg)

	// Start HTTP server
	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}

	log.Printf("Starting HTTP Server. Listening at %q", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("%v", err)
	} else {
		log.Println("Server closed!")
	}
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
