package middleware

import "net/http"

// S7: securityHeaders adds standard security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self' https://unpkg.com; style-src 'self' https://cdn.jsdelivr.net; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}
