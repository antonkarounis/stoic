package web

import (
	"embed"
	"fmt"
	"html/template"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
)

//go:embed views/include/*
//go:embed views/www/*
var templateFS embed.FS

func initTemplates() *framework.TemplateRegistry {
	registry, err := framework.NewTemplateRegistry(framework.TemplateRegistryOptions{
		FS:                   templateFS,
		RootDir:              "views/www",
		IncludeDir:           "views/include",
		Reload:               false,
		FuncMap:              template.FuncMap{},
		RequestFuncsProvider: loadTemplateFuncs,
	})
	if err != nil {
		panic(fmt.Errorf("error when loading templates: %w", err))
	}

	return registry
}
