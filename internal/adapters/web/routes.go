package web

import (
	"net/http"
	"os"

	"github.com/antonkarounis/balance/internal/adapters/web/controllers"
	"github.com/antonkarounis/balance/internal/adapters/web/middleware"
	gorillaHandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// RegisterRoutes sets up all application routes.
// Edit this file to add your pages and API endpoints.
func RegisterRoutes(r *mux.Router, authService AuthService) {
	registry := initTemplates(r)

	r.Use(middleware.NoCache)
	r.Use(middleware.SecurityHeaders)
	r.Use(func(next http.Handler) http.Handler {
		return gorillaHandlers.LoggingHandler(os.Stdout, next)
	})
	r.Use(gorillaHandlers.RecoveryHandler(gorillaHandlers.PrintRecoveryStack(true)))
	cop := http.NewCrossOriginProtection()
	r.Use(func(next http.Handler) http.Handler { return cop.Handler(next) })

	r.Use(authService.OptionalAuth)

	// Public routes
	r.PathPrefix("/static/").Handler(http.StripPrefix("/", StaticHandler())).Name("static")
	r.HandleFunc("/", controllers.Home(registry)).Methods("GET").Name("index")

	// Auth routes
	r.HandleFunc("/login", authService.Login).Methods("GET").Name("login")
	r.HandleFunc("/callback", authService.Callback).Methods("GET")
	r.HandleFunc("/logout", authService.Logout).Methods("POST").Name("logout")

	// Authenticated routes
	u := r.PathPrefix("/u").Subrouter()
	u.Use(authService.RequireAuth)
	u.HandleFunc("/dashboard", controllers.Dashboard(registry)).Methods("GET").Name("dashboard")
	u.HandleFunc("/profile", controllers.Profile(registry)).Methods("GET").Name("profile")
	u.HandleFunc("/time", controllers.Time()).Methods("GET").Name("time")
}
