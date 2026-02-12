package handlers

import (
	"bytes"
	"log"
	"net/http"

	"github.com/antonkarounis/stoic/internal/platform/auth"
)

func Dashboard(w http.ResponseWriter, r *http.Request) {
	session, err := auth.GetSessionFromContext(r)
	if err != nil {
		log.Printf("Session context error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Email":  session.Email,
		"UserID": session.UserID,
		"Roles":  session.Roles,
	}

	tmpl, err := manager.GetTemplate("dashboard.html", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

