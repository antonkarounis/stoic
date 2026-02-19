package app

import (
	"embed"
	"net/http"
	"strings"
)

//go:embed static/*
var staticFS embed.FS

func StaticHandler() http.HandlerFunc {
	server := http.FileServerFS(staticFS)

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		server.ServeHTTP(w, r)
	}
}
