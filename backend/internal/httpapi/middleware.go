package httpapi

import (
	"net/http"
	"strings"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/auth"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/security"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func rateLimit(api, authLimit *security.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := security.ClientKey(r)
			lim := api
			if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
				lim = authLimit
			}
			if lim != nil && !lim.Allow(key) {
				writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (h handlers) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.UserFrom(r.Context()); !ok {
			if h.svc != nil {
				_ = h.svc.Audit(r.Context(), "", "permission.denied", "", "", r.Method+" "+r.URL.Path)
			}
			writeError(w, http.StatusUnauthorized, "unauthorized", "sign in required")
			return
		}
		next(w, r)
	}
}
