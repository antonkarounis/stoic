package controllers

import (
	"net/http"

	"github.com/antonkarounis/balance/internal/adapters/web/framework"
)

func Home(registry *framework.TemplateRegistry) http.HandlerFunc {
	return registry.BuildSimpleHandler("home.html",
		func(w http.ResponseWriter, r *http.Request, te *framework.TemplateRenderer) {
			err := te.WriteTo(w, nil)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		})
}
