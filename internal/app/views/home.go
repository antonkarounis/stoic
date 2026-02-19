package views

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/platform/auth"
	"github.com/antonkarounis/stoic/internal/platform/web"
)

type HomeViewModel struct {
	Email string
}

func Home() http.HandlerFunc {
	return registry.BuildHandler("index.html", HomeViewModel{},
		func(w http.ResponseWriter, r *http.Request, te *web.TemplateRenderer) {
			session := auth.GetOptionalSession(r)

			data := HomeViewModel{}

			if session != nil {
				data.Email = session.Email
			}

			err := te.WriteTo(w, data)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		})
}
