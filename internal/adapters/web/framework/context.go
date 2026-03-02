package framework

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/antonkarounis/stoic/internal/domain/models"
	"github.com/gorilla/mux"
)

// --- user ---

type userKey string

const userContextKey userKey = "userKey"

// SetUserInContext returns a new request with the domain user stored in context.
func SetUserInContext(r *http.Request, user *models.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

// GetUserFromContext returns the domain user from the request context.
// Returns an error if no user is found (use on authenticated, registered routes).
func GetUserFromContext(r *http.Request) (*models.User, error) {
	val := r.Context().Value(userContextKey)
	if val == nil {
		return nil, errors.New("user not found in request context")
	}
	switch u := val.(type) {
	case *models.User:
		return u, nil
	default:
		return nil, fmt.Errorf("unexpected user type %T", u)
	}
}

// GetLoggedInUser returns the domain user if present, nil otherwise.
func GetLoggedInUser(r *http.Request) *models.User {
	val := r.Context().Value(userContextKey)
	if val == nil {
		return nil
	}
	if u, ok := val.(*models.User); ok {
		return u
	}
	return nil
}

// --- auth session ---

type authSessionKey string

const AuthSessionContextKey authSessionKey = "authSession"

// SetAuthSession returns a new request with the auth session stored in context.
func SetAuthSession(r *http.Request, session *models.SessionData) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), AuthSessionContextKey, session))
}

// GetAuthSession returns the auth session from context, or nil if not present.
func GetAuthSession(r *http.Request) *models.SessionData {
	s, _ := r.Context().Value(AuthSessionContextKey).(*models.SessionData)
	return s
}

// --- urlFor ---

type muxKey string

const muxContextKey muxKey = "muxKey"

func SetUrlFuncInContext(r *http.Request, baseMux *mux.Router) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), muxContextKey, baseMux))
}

func UrlFor(r *http.Request, name string) string {
	untyped := r.Context().Value(muxContextKey)

	if untyped == nil {
		slog.Error("mux not found in request context", "route", name)
		return ""
	}

	// G3: Use switch-bound variable directly
	switch mux := untyped.(type) {
	case *mux.Router:
		route := mux.Get(name)
		if route == nil {
			slog.Error("route name not found", "route", name)
			return ""
		}

		url, err := route.URL()
		if err != nil {
			slog.Error("could not generate url for route", "route", name, "error", err)
			return ""
		}

		return url.Path

	default:
		slog.Error("unexpected mux type in context", "type", fmt.Sprintf("%T", mux))
		return ""
	}
}
