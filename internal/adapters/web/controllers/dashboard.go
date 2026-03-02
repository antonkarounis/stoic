package controllers

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
)

func Dashboard(registry *framework.TemplateRegistry) http.HandlerFunc {
	return registry.BuildHandler("dashboard.html", nil,
		func(w http.ResponseWriter, r *http.Request, te *framework.TemplateRenderer) {
			te.WriteTo(w, nil)
		})
}
