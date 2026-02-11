package handlers

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/platform/auth"
)

func Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	session := auth.GetOptionalSession(r)

	data := struct {
		Email string
	}{
		Email: "",
	}

	if session != nil {
		data.Email = session.Email
	}

	template, err := manager.GetTemplate("index.html", data)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	template.ExecuteTemplate(w, "base.html", data)
}
