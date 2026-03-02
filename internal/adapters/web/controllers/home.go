package controllers

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
)

func Home(registry *framework.TemplateRegistry) http.HandlerFunc {
	return registry.BuildSimpleHandler("home.html",
		func(w http.ResponseWriter, r *http.Request, te *framework.TemplateRenderer) {
			te.WriteTo(w, nil)
		})
}
