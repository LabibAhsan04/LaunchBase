package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
)

func TestOpsStatusUnauthorizedAndOK(t *testing.T) {
	t.Setenv("OPS_TOKEN", "secret-ops")
	db, _ := testDB(t)
	mux := NewMux(&service.Service{Store: db})

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/ops/status", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized %d %s", rr.Code, rr.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/status", nil)
	req.Header.Set("Authorization", "Bearer secret-ops")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("ok %d %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("%v", body)
	}
}

func TestOpsStatusHiddenWithoutToken(t *testing.T) {
	t.Setenv("OPS_TOKEN", "")
	db, _ := testDB(t)
	mux := NewMux(&service.Service{Store: db})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/ops/status", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
}
