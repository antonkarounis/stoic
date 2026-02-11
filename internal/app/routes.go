package app

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/antonkarounis/stoic/internal/app/handlers"
	"github.com/antonkarounis/stoic/internal/platform/auth"
	"github.com/antonkarounis/stoic/internal/platform/config"
	"github.com/antonkarounis/stoic/internal/platform/web"

	"github.com/gorilla/mux"
)

//go:embed templates/*/*
var embeddedFS embed.FS

// RegisterRoutes sets up all application routes.
// Edit this file to add your pages and API endpoints.
func RegisterRoutes(r *mux.Router, cfg *config.Config) {
	initTemplates(cfg)

	// Public routes
	r.HandleFunc("/", handlers.Home).Methods("GET")

	// Auth routes (provided by platform)
	r.HandleFunc("/login", auth.Login).Methods("GET")
	r.HandleFunc("/callback", auth.Callback).Methods("GET")
	r.HandleFunc("/logout", auth.Logout).Methods("GET")

	// Authenticated routes
	u := r.PathPrefix("/u").Subrouter()
	u.Use(auth.RequireAuth)
	u.HandleFunc("/dashboard", handlers.Dashboard).Methods("GET")
	u.HandleFunc("/events/time", handlers.SSE()).Methods("GET")

	// Add your routes here...
}

func initTemplates(cfg *config.Config) {
	dev := cfg.Environment == "dev"

	var f fs.FS
	var reload bool

	if dev {
		fmt.Println("WARNING: dev mode")
		f = os.DirFS("internal/app")
		reload = true
	} else {
		f = embeddedFS
		reload = false
	}

	manager, err := web.NewTemplateManager(web.TemplateManagerOptions{
		FS:         f,
		RootDir:    "templates/www",
		IncludeDir: "templates/include",
		Reload:     reload,
	})
	if err != nil {
		panic(fmt.Errorf("error when loading templates: %w", err))
	}

	handlers.Init(manager)
}
