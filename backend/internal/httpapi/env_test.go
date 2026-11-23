package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/envcrypt"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
)

func TestProjectEnvCRUD(t *testing.T) {
	db, _ := testDB(t)
	box, err := envcrypt.New("test-env-key")
	if err != nil {
		t.Fatal(err)
	}
	svc := &service.Service{Store: db, EnvBox: box}
	mux := NewMux(svc)

	res := call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"EnvApp","repoUrl":"https://github.com/acme/web.git","branch":"main"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create %d %s", res.Code, res.Body.String())
	}
	var created projectJSON
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	res = call(t, mux, http.MethodPut, "/api/v1/projects/"+created.ID+"/env", `{"key":"FINNHUB_API_KEY","value":"super-secret-token"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("put %d %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "super-secret-token") {
		t.Fatal("plaintext leaked")
	}
	if !strings.Contains(res.Body.String(), "FINNHUB_API_KEY") || !strings.Contains(res.Body.String(), "••••") {
		t.Fatalf("mask missing: %s", res.Body.String())
	}

	res = call(t, mux, http.MethodGet, "/api/v1/projects/"+created.ID+"/env", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "FINNHUB_API_KEY") {
		t.Fatalf("list %d %s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "super-secret-token") {
		t.Fatal("list leaked plaintext")
	}

	res = call(t, mux, http.MethodPut, "/api/v1/projects/"+created.ID+"/env", `{"key":"DATABASE_URL","value":"postgres://nope"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("reserved key %d %s", res.Code, res.Body.String())
	}

	res = call(t, mux, http.MethodDelete, "/api/v1/projects/"+created.ID+"/env/FINNHUB_API_KEY", "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", res.Code, res.Body.String())
	}
	res = call(t, mux, http.MethodGet, "/api/v1/projects/"+created.ID+"/env", "")
	if res.Code != http.StatusOK || strings.Contains(res.Body.String(), "FINNHUB_API_KEY") {
		t.Fatalf("after delete %d %s", res.Code, res.Body.String())
	}
}
