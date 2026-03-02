package web

import (
	"html/template"
	"net/http"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
	"github.com/antonkarounis/stoic/internal/domain/models"
)

func loadTemplateFuncs(r *http.Request) template.FuncMap {
	return template.FuncMap{
		"urlFor":      urlFor(r),
		"isLoggedIn":  loggedIn(r),
		"currentUser": currentUser(r),
	}
}

func urlFor(r *http.Request) func(string) string {
	return func(name string) string {
		return framework.UrlFor(r, name)
	}
}

func loggedIn(r *http.Request) func() bool {
	return func() bool {
		return framework.GetLoggedInUser(r) != nil
	}
}

func currentUser(r *http.Request) func() models.User {
	return func() models.User {
		if u := framework.GetLoggedInUser(r); u != nil {
			return *u
		}
		return models.User{}
	}
}
