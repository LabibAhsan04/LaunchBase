package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/observability"
)

func TestReadyOK(t *testing.T) {
	mux := NewMuxWithOptions(nil, nil, Options{
		Ready: func(context.Context) error { return nil },
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("%v", body)
	}
}

func TestReadyDependencyFailure(t *testing.T) {
	mux := NewMuxWithOptions(nil, nil, Options{
		Ready: func(context.Context) error { return errors.New("db down") },
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "db down") {
		t.Fatalf("must not expose dependency detail: %s", rr.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	m := observability.NewMetrics()
	m.ObserveHTTP("GET", "/health", 200, 1)
	mux := NewMuxWithOptions(nil, nil, Options{Metrics: m.Handler()})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("%d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "launchbase_http_requests_total") {
		t.Fatalf("%s", rr.Body.String())
	}
}

func TestGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{
		Handler: NewMux(nil),
	}
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve(ln)
	}()

	url := "http://" + ln.Addr().String() + "/health"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != http.ErrServerClosed {
		t.Fatalf("%v", err)
	}

	_, err = http.Get(url)
	if err == nil {
		t.Fatal("expected connection refused after shutdown")
	}
}
