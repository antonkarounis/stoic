package app

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"

	"github.com/antonkarounis/stoic/internal/app/views"
	"github.com/antonkarounis/stoic/internal/platform/auth"
	"github.com/antonkarounis/stoic/internal/platform/config"
	"github.com/antonkarounis/stoic/internal/platform/web"

	"github.com/gorilla/mux"
)

//go:embed templates/*/*
var embeddedFS embed.FS

// RegisterRoutes sets up all application routes.
// Edit this file to add your pages and API endpoints.
func RegisterRoutes(r *mux.Router, cfg *config.Config, authService *auth.AuthService) {
	initTemplates(cfg, r)

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

func initTemplates(cfg *config.Config, r *mux.Router) *web.TemplateRegistry {
	var f fs.FS
	var reload bool

	if cfg.IsDev() {
		fmt.Println("WARNING: dev mode")
		f = os.DirFS("internal/app")
		reload = true
	} else {
		f = embeddedFS
		reload = false
	}

	funcMap := template.FuncMap{
		"url": makeURLFunc(r),
	}

	registry, err := web.NewTemplateRegistry(web.TemplateRegistryOptions{
		FS:         f,
		RootDir:    "templates/www",
		IncludeDir: "templates/include",
		Reload:     reload,
		FuncMap:    funcMap,
	})
	if err != nil {
		panic(fmt.Errorf("error when loading templates: %w", err))
	}

	views.Init(registry)

	return registry
}

// makeURLFunc returns a template function that generates URLs from route names.
// Returns an error (which stops template execution) instead of panicking at render time.
func makeURLFunc(router *mux.Router) func(string, ...string) (string, error) {
	return func(name string, pairs ...string) (string, error) {
		route := router.Get(name)
		if route == nil {
			return "", fmt.Errorf("URL generation error: route '%s' not found", name)
		}
		url, err := route.URL(pairs...)
		if err != nil {
			return "", fmt.Errorf("URL generation error for route '%s': %v", name, err)
		}
		return url.Path, nil
	}
}
