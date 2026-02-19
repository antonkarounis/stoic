package app

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/app/views"
	"github.com/antonkarounis/stoic/internal/platform/auth"
	"github.com/antonkarounis/stoic/internal/platform/config"

	"github.com/gorilla/mux"
)

// RegisterRoutes sets up all application routes.
// Edit this file to add your pages and API endpoints.
func RegisterRoutes(r *mux.Router, cfg *config.Config, authService *auth.AuthService) {
	initTemplates(cfg, r)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/", StaticHandler())).Name("static")

	// Public routes
	r.HandleFunc("/", views.Home()).Methods("GET").Name("index")

	// Auth routes (provided by platform)
	r.HandleFunc("/login", authService.Login).Methods("GET").Name("login")
	r.HandleFunc("/callback", authService.Callback).Methods("GET")
	r.HandleFunc("/logout", authService.Logout).Methods("POST").Name("logout")

	// Authenticated routes
	u := r.PathPrefix("/u").Subrouter()
	u.Use(authService.RequireAuth)
	u.HandleFunc("/dashboard", views.Dashboard()).Methods("GET").Name("dashboard")
	u.HandleFunc("/events/time", views.SSE()).Methods("GET").Name("time")

	// Add your routes here...
}
