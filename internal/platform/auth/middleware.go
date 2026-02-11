package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type contextKey string

const sessionContextKey contextKey = "session"

// RequireAuth is middleware that requires a valid session. Redirects to /login if not authenticated.
func RequireAuth(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		session, exists := GetSession(r.Context(), cookie.Value)
		if !exists || time.Now().After(session.Expires) {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		if err := RefreshToken(r.Context(), cookie.Value, session); err != nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, setSessionInContext(r, session))
	}

	return http.HandlerFunc(fn)
}

// OptionalAuth adds session to context if logged in, but doesn't require it.
func OptionalAuth(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		session, exists := GetSession(r.Context(), cookie.Value)
		if !exists || time.Now().After(session.Expires) {
			next.ServeHTTP(w, r)
			return
		}

		_ = RefreshToken(r.Context(), cookie.Value, session)
		next.ServeHTTP(w, setSessionInContext(r, session))
	}

	return http.HandlerFunc(fn)
}

func setSessionInContext(r *http.Request, session *SessionData) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionContextKey, session))
}

// GetSessionFromContext returns the session from the request context.
// Returns an error if no session is found (use on authenticated routes).
func GetSessionFromContext(r *http.Request) (*SessionData, error) {
	session := r.Context().Value(sessionContextKey)

	if session == nil {
		return nil, errors.New("session not found in request context")
	}

	switch t := session.(type) {
	case *SessionData:
		return session.(*SessionData), nil
	default:
		return nil, fmt.Errorf("unexpected session type %T", t)
	}
}

// GetOptionalSession returns the session if present, nil otherwise.
func GetOptionalSession(r *http.Request) *SessionData {
	session := r.Context().Value(sessionContextKey)
	if session == nil {
		return nil
	}
	if s, ok := session.(*SessionData); ok {
		return s
	}
	return nil
}
