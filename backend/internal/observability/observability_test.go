package observability

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	log := NewLogger("api", &buf)
	log.Info("hello", map[string]any{"requestId": "abc", "deploymentId": "d1"})
	var row map[string]any
	if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	if row["msg"] != "hello" || row["service"] != "api" || row["requestId"] != "abc" {
		t.Fatalf("%v", row)
	}
}

func TestTraceparentRoundTrip(t *testing.T) {
	tid, sid := NewIDs()
	h := FormatTraceparent(tid, sid)
	gotT, gotS, ok := ParseTraceparent(h)
	if !ok || gotT != tid || gotS != sid {
		t.Fatalf("%s %s %v", gotT, gotS, ok)
	}
}

func TestHTTPMiddlewareMetrics(t *testing.T) {
	m := NewMetrics()
	var buf bytes.Buffer
	log := NewLogger("api", &buf)
	h := HTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}), log, m)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("%d", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" || rec.Header().Get("traceparent") == "" {
		t.Fatalf("%v", rec.Header())
	}
	body := httptest.NewRecorder()
	m.Handler().ServeHTTP(body, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(body.Body.String(), "launchbase_http_requests_total") {
		t.Fatalf("%s", body.Body.String())
	}
}

func TestMetricPathScrubsUUIDs(t *testing.T) {
	got := metricPath("/api/v1/deployments/7a2d4c3b-2222-4222-8222-222222222222/logs")
	if got != "/api/v1/deployments/:id/logs" {
		t.Fatalf("%s", got)
	}
}
