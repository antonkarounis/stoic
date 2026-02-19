package app

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"os"

	"github.com/antonkarounis/stoic/internal/app/views"
	"github.com/antonkarounis/stoic/internal/platform/config"
	"github.com/antonkarounis/stoic/internal/platform/web"
	"github.com/gorilla/mux"
)

//go:embed templates/*/*
var templateFS embed.FS

func initTemplates(cfg *config.Config, r *mux.Router) *web.TemplateRegistry {
	var f fs.FS
	var reload bool

	if cfg.IsDev() {
		fmt.Println("WARNING: dev mode")
		f = os.DirFS("internal/app")
		reload = true
	} else {
		f = templateFS
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
