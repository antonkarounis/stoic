package controllers

import (
	"log/slog"
	"net/http"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
)

type MemberViewModel struct {
	ID   string
	Name string
	Role string
}

type ProfileViewModel struct {
	Name  string
	Email string
}

func Profile(registry *framework.TemplateRegistry) http.HandlerFunc {
	return registry.BuildHandler("profile.html", ProfileViewModel{},
		func(w http.ResponseWriter, r *http.Request, te *framework.TemplateRenderer) {
			user, err := framework.GetUserFromContext(r)
			if err != nil {
				slog.Error("user context error", "error", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if err != nil {
				httpError(w, err)
				return
			}

			te.WriteTo(w, ProfileViewModel{
				Name:  user.Name,
				Email: user.Email,
			})
		})
}
