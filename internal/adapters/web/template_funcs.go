package web

import (
	"html/template"
	"net/http"

	"github.com/antonkarounis/balance/internal/adapters/web/framework"
	"github.com/antonkarounis/balance/internal/ports"
)

func loadTemplateFuncs(r *http.Request) template.FuncMap {
	return template.FuncMap{
		"isLoggedIn": loggedIn(r),
		"session":    session(r),
	}
}

func loggedIn(r *http.Request) func() bool {
	return func() bool {
		user := framework.GetOptionalSession(r)
		return user != nil
	}
}

func session(r *http.Request) func() ports.SessionData {
	return func() ports.SessionData {
		session, _ := framework.GetSessionFromContext(r)

		if session != nil {
			return *session
		}
		return ports.SessionData{}
	}
}
