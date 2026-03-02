package middleware

import "net/http"

// SecurityHeadersMiddleware returns middleware that adds standard security headers.
// When isDev is false, HSTS is also added.
func SecurityHeadersMiddleware(isDev bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-XSS-Protection", "0")
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self' https://unpkg.com; style-src 'self' https://cdn.jsdelivr.net; connect-src 'self'")
			if !isDev {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}
