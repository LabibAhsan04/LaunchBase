package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHashTokenStable(t *testing.T) {
	a := HashToken("abc")
	b := HashToken("abc")
	if a != b || a == "" {
		t.Fatalf("%q %q", a, b)
	}
	if HashToken("abc") == HashToken("abd") {
		t.Fatal("hashes collided")
	}
}

func TestSessionCookieFlags(t *testing.T) {
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, "tok", time.Now().UTC().Add(time.Hour))
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("%d cookies", len(cookies))
	}
	c := cookies[0]
	if c.Name != CookieName || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("%+v", c)
	}
}
