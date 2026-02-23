package framework

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/antonkarounis/balance/internal/ports"
)

type contextKey string

const sessionContextKey contextKey = "session"

// SetSessionInContext returns a new request with the session stored in context (exported for use by adapters)
func SetSessionInContext(r *http.Request, session *ports.SessionData) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionContextKey, session))
}

// GetSessionFromContext returns the session from the request context.
// Returns an error if no session is found (use on authenticated routes).
func GetSessionFromContext(r *http.Request) (*ports.SessionData, error) {
	session := r.Context().Value(sessionContextKey)

	if session == nil {
		return nil, errors.New("session not found in request context")
	}

	// G3: Use switch-bound variable directly
	switch s := session.(type) {
	case *ports.SessionData:
		return s, nil
	default:
		return nil, fmt.Errorf("unexpected session type %T", s)
	}
}

// GetOptionalSession returns the session if present, nil otherwise.
func GetOptionalSession(r *http.Request) *ports.SessionData {
	session := r.Context().Value(sessionContextKey)
	if session == nil {
		return nil
	}
	if s, ok := session.(*ports.SessionData); ok {
		return s
	}
	return nil
}
