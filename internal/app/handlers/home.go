package handlers

import (
	"bytes"
	"log"
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

	tmpl, err := manager.GetTemplate("index.html", data)
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
