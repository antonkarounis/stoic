package controllers

import (
	"log"
	"net/http"

	"github.com/antonkarounis/balance/internal/adapters/web/framework"
)

type ProfileViewModel struct {
	Email  string
	UserID string
	Roles  []string
}

func Profile(registry *framework.TemplateRegistry) http.HandlerFunc {
	return registry.BuildHandler("profile.html", ProfileViewModel{},
		func(w http.ResponseWriter, r *http.Request, te *framework.TemplateRenderer) {
			session, err := framework.GetSessionFromContext(r)
			if err != nil {
				log.Printf("Session context error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			data := ProfileViewModel{
				Email:  session.Email,
				UserID: session.UserID,
				Roles:  session.Roles,
			}

			err = te.WriteTo(w, data)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		})
}
