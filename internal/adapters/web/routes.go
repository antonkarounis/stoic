package web

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/adapters/web/controllers"
	"github.com/antonkarounis/stoic/internal/adapters/web/middleware"
	"github.com/antonkarounis/stoic/internal/adapters/web/views"
	"github.com/antonkarounis/stoic/internal/domain/ports"
	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterRoutes sets up all application routes.
// Edit this file to add your pages and API endpoints.
func RegisterRoutes(mux *mux.Router, authService AuthService, userRepo ports.UserRepository, pool *pgxpool.Pool, isDev bool) {

	// Health endpoints — registered before any middleware so they are always reachable
	mux.HandleFunc("/healthz", healthz).Methods("GET")
	mux.HandleFunc("/readyz", readyz(pool)).Methods("GET")

	// general always-on middleware
	mux.Use(middleware.AccessLog)
	mux.Use(middleware.NoCache)
	mux.Use(middleware.SecurityHeadersMiddleware(isDev))
	cop := http.NewCrossOriginProtection()
	mux.Use(func(next http.Handler) http.Handler { return cop.Handler(next) })
	mux.Use(gorillaHandlers.RecoveryHandler(gorillaHandlers.PrintRecoveryStack(true)))
	mux.Use(middleware.UrlForMiddleware(mux))

	// auth and user loading
	mux.Use(authService.CheckAuth)
	mux.Use(middleware.ResolveUser(userRepo))

	registry := initTemplates()

	// Public routes
	mux.PathPrefix("/static/").Handler(StaticHandler(views.StaticFS)).Name("static")
	mux.HandleFunc("/", controllers.Home(registry)).Methods("GET").Name("index")

	// Auth routes
	mux.HandleFunc("/login", authService.Login).Methods("GET").Name("login")
	mux.HandleFunc("/register", authService.Register).Methods("GET").Name("register")
	mux.HandleFunc("/callback", authService.Callback).Methods("GET")
	mux.HandleFunc("/logout", authService.Logout).Methods("POST").Name("logout")

	// Authenticated routes
	app := mux.PathPrefix("/app").Subrouter()
	app.Use(authService.RequireAuth)
	app.HandleFunc("/dashboard", controllers.Dashboard(registry)).Methods("GET").Name("dashboard")
	app.HandleFunc("/profile", controllers.Profile(registry)).Methods("GET").Name("profile")
	app.HandleFunc("/time", controllers.Time()).Methods("GET").Name("time")

	authService.SetLoginRedirect("dashboard")
	authService.SetLoginFailureRedirect("login")
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func readyz(pool *pgxpool.Pool) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}
