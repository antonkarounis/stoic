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
// If OptionalAuth already loaded the session into the context, it reuses it (avoiding duplicate DB queries).
func (s *AuthService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// B4: Check if OptionalAuth already loaded the session
		if session := GetOptionalSession(r); session != nil {
			cookie, _ := r.Cookie("session_id")
			if cookie != nil {
				if err := s.RefreshToken(r.Context(), cookie.Value, session); err != nil {
					http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
					return
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		session, exists := s.GetSession(r.Context(), cookie.Value)
		if !exists || time.Now().After(session.Expires) {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		if err := s.RefreshToken(r.Context(), cookie.Value, session); err != nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, setSessionInContext(r, session))
	})
}

// OptionalAuth adds session to context if logged in, but doesn't require it.
func (s *AuthService) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		session, exists := s.GetSession(r.Context(), cookie.Value)
		if !exists || time.Now().After(session.Expires) {
			next.ServeHTTP(w, r)
			return
		}

		_ = s.RefreshToken(r.Context(), cookie.Value, session)
		next.ServeHTTP(w, setSessionInContext(r, session))
	})
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

	// G3: Use switch-bound variable directly
	switch s := session.(type) {
	case *SessionData:
		return s, nil
	default:
		return nil, fmt.Errorf("unexpected session type %T", s)
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
