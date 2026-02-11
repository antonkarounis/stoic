package handlers

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/platform/auth"
)

func Dashboard(w http.ResponseWriter, r *http.Request) {
	session, err := auth.GetSessionFromContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Email":  session.Email,
		"UserID": session.UserID,
		"Roles":  session.Roles,
	}

	template, err := manager.GetTemplate("dashboard.html", data)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	template.ExecuteTemplate(w, "base.html", data)
}
