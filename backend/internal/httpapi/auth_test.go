package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/auth"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
)

func TestOwnershipIsolation(t *testing.T) {
	db, _ := testDB(t)
	svc := &service.Service{Store: db}
	mux := NewMux(svc)

	res := call(t, mux, http.MethodPost, "/api/v1/projects", `{"name":"Mine","repoUrl":"https://github.com/acme/web"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create %d %s", res.Code, res.Body.String())
	}
	var project projectJSON
	if err := json.Unmarshal(res.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}

	otherID := "00000000-0000-4000-8000-000000000099"
	if _, err := db.UpsertGitHubUser(context.Background(), otherID, 99, "other", "Other", ""); err != nil {
		t.Fatal(err)
	}
	raw, err := auth.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	if _, err := db.CreateSession(context.Background(), otherID, auth.HashToken(raw), expires); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+project.ID, nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: raw})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected not found for other user, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestGitHubModeRequiresSession(t *testing.T) {
	t.Setenv("AUTH_MODE", "github")
	db, _ := testDB(t)
	svc := &service.Service{Store: db}
	mux := NewMux(svc)
	res := call(t, mux, http.MethodGet, "/api/v1/projects", "")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("got %d %s", res.Code, res.Body.String())
	}
}

func TestWebhookSignatureRequired(t *testing.T) {
	db, _ := testDB(t)
	gh, err := github.NewClient(github.AppConfig{
		AppID: "1", ClientID: "cid", PrivateKeyPEM: mustTestKeyPEM(t), WebhookSecret: "whsec",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &service.Service{Store: db, GitHub: gh}
	mux := NewMuxWithGitHub(svc, gh)

	body := []byte(`{"action":"deleted","installation":{"id":42,"account":{"login":"acme","type":"User"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "installation")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned %d", rec.Code)
	}

	mac := hmac.New(sha256.New, []byte("whsec"))
	_, _ = mac.Write(body)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "installation")
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed %d %s", rec.Code, rec.Body.String())
	}
}

func TestSessionMe(t *testing.T) {
	db, _ := testDB(t)
	svc := &service.Service{Store: db}
	mux := NewMux(svc)
	res := call(t, mux, http.MethodGet, "/api/v1/auth/me", "")
	if res.Code != http.StatusOK {
		t.Fatalf("%d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"login":"local"`) {
		t.Fatalf("%s", res.Body.String())
	}
}

func mustTestKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
}
