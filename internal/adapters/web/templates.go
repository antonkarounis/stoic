package web

import (
	"embed"
	"fmt"
	"html/template"

	"github.com/antonkarounis/balance/internal/adapters/web/framework"
	"github.com/gorilla/mux"
)

//go:embed views/include/*
//go:embed views/www/*
var templateFS embed.FS

func initTemplates(r *mux.Router) *framework.TemplateRegistry {
	funcMap := template.FuncMap{
		"url": makeURLFunc(r),
	}

	registry, err := framework.NewTemplateRegistry(framework.TemplateRegistryOptions{
		FS:                   templateFS,
		RootDir:              "views/www",
		IncludeDir:           "views/include",
		Reload:               false,
		FuncMap:              funcMap,
		RequestFuncsProvider: loadTemplateFuncs,
	})
	if err != nil {
		panic(fmt.Errorf("error when loading templates: %w", err))
	}

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
