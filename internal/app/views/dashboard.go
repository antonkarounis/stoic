package views

import (
	"log"
	"net/http"

	"github.com/antonkarounis/stoic/internal/platform/auth"
	"github.com/antonkarounis/stoic/internal/platform/web"
)

type DashboardViewModel struct {
	Email  string
	UserID string
	Roles  []string
}

func Dashboard() http.HandlerFunc {
	return registry.BuildHandler("dashboard.html", DashboardViewModel{},
		func(w http.ResponseWriter, r *http.Request, te *web.TemplateRenderer) {
			session, err := auth.GetSessionFromContext(r)
			if err != nil {
				log.Printf("Session context error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			data := DashboardViewModel{
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
