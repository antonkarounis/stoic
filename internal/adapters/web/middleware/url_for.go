package middleware

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
	"github.com/gorilla/mux"
)

func UrlForMiddleware(baseMux *mux.Router) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, framework.SetUrlFuncInContext(r, baseMux))
		})
	}
}
