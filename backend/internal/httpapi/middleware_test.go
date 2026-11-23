package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/security"
)

func TestSecurityHeaders(t *testing.T) {
	h := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("%v", rec.Header())
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("%v", rec.Header())
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	api := security.NewLimiter(1)
	auth := security.NewLimiter(1)
	h := rateLimit(api, auth)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.RemoteAddr = "203.0.113.10:9"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}
