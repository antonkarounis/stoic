package middleware

import (
	"net/http"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
	"github.com/antonkarounis/stoic/internal/domain/ports"
)

// ResolveUser loads the domain User into context if the session has a linked UserID.
// Silent no-op when there is no session or the identity is not yet linked to a user.
func ResolveUser(userRepo ports.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := framework.GetAuthSession(r)
			if session != nil && session.UserID != nil {
				if user, err := userRepo.FindByID(r.Context(), *session.UserID); err == nil {
					r = framework.SetUserInContext(r, &user)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
