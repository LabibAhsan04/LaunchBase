package auth

import (
	"net/http"
	"time"
)

type Middleware struct {
	Sessions SessionStore
	DevUser  User
}

func (m Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := ReadCookie(r, CookieName)
		if raw != "" && m.Sessions != nil {
			user, _, err := m.Sessions.LookupSession(r.Context(), HashToken(raw), time.Now().UTC())
			if err == nil {
				next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
				return
			}
		}
		if !RequiresGitHub() && m.DevUser.ID != "" {
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), m.DevUser)))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFrom(r.Context()); !ok {
			http.Error(w, `{"error":{"code":"unauthorized","message":"sign in required"}}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
