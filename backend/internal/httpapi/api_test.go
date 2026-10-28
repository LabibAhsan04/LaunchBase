package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

func TestProjectAndDeploymentFlow(t *testing.T) {
	remover := &fakeRemover{}
	db, dsn := testDB(t)
	svc := &service.Service{Store: db, Run: readyRun, Remover: remover}
	mux := NewMux(svc)

	res := call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Demo","repoUrl":"https://github.com/acme/web.git","branch":"main"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create %d %s", res.Code, res.Body.String())
	}
	if res.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request id")
	}
	var created projectJSON
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.RepoURL != "https://github.com/acme/web.git" || created.Branch != "main" {
		t.Fatalf("%+v", created)
	}
	if err := svc.Store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	svc.Store = reopened
	res = call(t, mux, http.MethodGet, "/api/v1/projects/"+created.ID, "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"Demo"`) {
		t.Fatalf("after reconnect %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodGet, "/api/v1/projects", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), created.ID) {
		t.Fatalf("list %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodPost, "/api/v1/projects/"+created.ID+"/deployments", `{"branch":"dev"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("deploy %d %s", res.Code, res.Body.String())
	}
	var dep deploymentJSON
	if err := json.Unmarshal(res.Body.Bytes(), &dep); err != nil {
		t.Fatal(err)
	}
	if dep.Status != "READY" || dep.URL != "http://127.0.0.1:34567/" || dep.Branch != "dev" {
		t.Fatalf("%+v", dep)
	}
	if dep.FinishedAt == nil || dep.FinishedAt.Location() != time.UTC {
		t.Fatalf("finished %#v", dep.FinishedAt)
	}

	res = call(t, mux, http.MethodGet, "/api/v1/projects/"+created.ID, "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), dep.ID) {
		t.Fatalf("get project %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodPost, "/api/v1/deployments/"+dep.ID+"/stop", "")
	if res.Code != http.StatusOK {
		t.Fatalf("stop %d %s", res.Code, res.Body.String())
	}
	if len(remover.names) != 1 || remover.names[0] != "launchbase-test" {
		t.Fatalf("removed %#v", remover.names)
	}
	var stopped deploymentJSON
	if err := json.Unmarshal(res.Body.Bytes(), &stopped); err != nil {
		t.Fatal(err)
	}
	if stopped.Status != "STOPPED" {
		t.Fatalf("%s", stopped.Status)
	}
}

func TestRejectsBadInputAndIllegalStop(t *testing.T) {
	db, _ := testDB(t)
	svc := &service.Service{Store: db, Run: failRun, Remover: &fakeRemover{}}
	mux := NewMux(svc)

	res := call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Demo","repoUrl":"git@github.com:acme/web.git"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("ssh url %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Demo","repoUrl":"https://github.com/acme/web","extra":1}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("unknown field %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Demo","repoUrl":"https://github.com/acme/web"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create %d %s", res.Code, res.Body.String())
	}
	var created projectJSON
	_ = json.Unmarshal(res.Body.Bytes(), &created)

	res = call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Demo","repoUrl":"https://github.com/acme/other"}`)
	if res.Code != http.StatusConflict {
		t.Fatalf("duplicate %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodPost, "/api/v1/projects/"+created.ID+"/deployments", "")
	if res.Code != http.StatusCreated {
		t.Fatalf("deploy %d %s", res.Code, res.Body.String())
	}
	var dep deploymentJSON
	_ = json.Unmarshal(res.Body.Bytes(), &dep)
	if dep.Status != "FAILED" || !strings.Contains(dep.Error, "repository not found") {
		t.Fatalf("%+v", dep)
	}
	res = call(t, mux, http.MethodPost, "/api/v1/deployments/"+dep.ID+"/stop", "")
	if res.Code != http.StatusConflict {
		t.Fatalf("stop failed deploy %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodGet, "/api/v1/projects/not-a-uuid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("bad id %d", res.Code)
	}
}

func TestStatusJumpIsNotStored(t *testing.T) {
	db, _ := testDB(t)
	svc := &service.Service{Store: db, Run: jumpRun}
	mux := NewMux(svc)
	res := call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Demo","repoUrl":"https://github.com/acme/web"}`)
	var created projectJSON
	_ = json.Unmarshal(res.Body.Bytes(), &created)
	res = call(t, mux, http.MethodPost, "/api/v1/projects/"+created.ID+"/deployments", "")
	var dep deploymentJSON
	_ = json.Unmarshal(res.Body.Bytes(), &dep)
	if dep.Status != "FAILED" || dep.URL != "" {
		t.Fatalf("jump stored as %+v", dep)
	}
}

func readyRun(_ context.Context, opt deployments.Options) (deployments.Result, error) {
	for _, st := range []deployments.Status{
		deployments.StatusCloning,
		deployments.StatusDetecting,
		deployments.StatusBuilding,
		deployments.StatusStarting,
		deployments.StatusHealthChecking,
		deployments.StatusReady,
	} {
		if err := opt.OnStatus(st); err != nil {
			return deployments.Result{}, err
		}
	}
	return deployments.Result{Image: "launchbase/test", Container: "launchbase-test", URL: "http://127.0.0.1:34567/"}, nil
}

func failRun(_ context.Context, opt deployments.Options) (deployments.Result, error) {
	if err := opt.OnStatus(deployments.StatusCloning); err != nil {
		return deployments.Result{}, err
	}
	return deployments.Result{}, fmt.Errorf("clone: repository not found")
}

func jumpRun(_ context.Context, opt deployments.Options) (deployments.Result, error) {
	if err := opt.OnStatus(deployments.StatusReady); err != nil {
		return deployments.Result{}, err
	}
	return deployments.Result{URL: "http://127.0.0.1:1/"}, nil
}

type fakeRemover struct {
	names []string
}

func (f *fakeRemover) RemoveContainer(_ context.Context, name string) error {
	f.names = append(f.names, name)
	return nil
}

func call(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body == "" {
		rdr = bytes.NewReader(nil)
	} else {
		rdr = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func testDB(t *testing.T) (*store.DB, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://launchbase:launchbase@localhost:5433/launchbase?sslmode=disable"
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		t.Fatalf("postgres is not running (%v). start it with make dev-infra", err)
	}
	var raw [4]byte
	_, _ = rand.Read(raw[:])
	schema := "lb_" + hex.EncodeToString(raw[:])
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+store.QuoteIdent(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + store.QuoteIdent(schema) + " CASCADE")
		_ = admin.Close()
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := store.Open(context.Background(), u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db, u.String()
}
