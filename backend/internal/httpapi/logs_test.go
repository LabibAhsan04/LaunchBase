package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
)

func TestLogHistoryAndStream(t *testing.T) {
	db, _ := testDB(t)
	hub := logs.NewMemoryHub()
	broker := &logs.Broker{Sink: db, Hub: hub}
	svc := &service.Service{Store: db, Logs: broker}
	mux := NewMux(svc)

	res := call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Demo","repoUrl":"https://github.com/acme/web"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create %d %s", res.Code, res.Body.String())
	}
	var project projectJSON
	if err := json.Unmarshal(res.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	res = call(t, mux, http.MethodPost, "/api/v1/projects/"+project.ID+"/deployments", "")
	var dep deploymentJSON
	if err := json.Unmarshal(res.Body.Bytes(), &dep); err != nil {
		t.Fatal(err)
	}

	if _, err := broker.Append(context.Background(), dep.ID, logs.SourceWorker, logs.LevelInfo, "line-one"); err != nil {
		t.Fatal(err)
	}
	if _, err := broker.Append(context.Background(), dep.ID, logs.SourceSystem, logs.LevelDone, "failed"); err != nil {
		t.Fatal(err)
	}

	res = call(t, mux, http.MethodGet, "/api/v1/deployments/"+dep.ID+"/logs?after=0", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "line-one") {
		t.Fatalf("history %d %s", res.Code, res.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/"+dep.ID+"/logs/stream", nil)
	req.Header.Set("Last-Event-ID", "1")
	rr := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rr, req)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not finish")
	}
	body := rr.Body.String()
	if !strings.Contains(body, "event: log") || !strings.Contains(body, "event: done") {
		t.Fatalf("stream body %s", body)
	}
	if strings.Contains(body, "line-one") {
		t.Fatal("reconnect should skip seq 1")
	}
	if !strings.Contains(body, `"seq":2`) {
		t.Fatalf("expected seq 2 in %s", body)
	}
}

func TestLogStreamUnknownDeployment(t *testing.T) {
	db, _ := testDB(t)
	svc := &service.Service{Store: db, Logs: &logs.Broker{Sink: db, Hub: logs.NewMemoryHub()}}
	mux := NewMux(svc)
	res := call(t, mux, http.MethodGet, "/api/v1/deployments/00000000-0000-4000-8000-000000000099/logs", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status %d", res.Code)
	}
}
