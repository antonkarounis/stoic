package web

import (
	"embed"
	"fmt"
	"net/http"
	"strings"
)

func StaticHandler(fs embed.FS) http.HandlerFunc {
	server := http.FileServerFS(fs)

	fmt.Println("static files:")
	walkFS(fs, ".")

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		server.ServeHTTP(w, r)
	}
}

func walkFS(fs embed.FS, path string) {
	dirs, err := fs.ReadDir(path)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, item := range dirs {
		newPath := path + "/" + item.Name()

		if path == "." {
			newPath = item.Name()
		}

		fmt.Println("  " + newPath)

		if item.Type().IsDir() {
			walkFS(fs, newPath)
		}
	}
}
