package security

import (
	"net/http/httptest"
	"testing"
)

func TestLimiterWindow(t *testing.T) {
	l := NewLimiter(2)
	if !l.Allow("a") || !l.Allow("a") {
		t.Fatal("first two should pass")
	}
	if l.Allow("a") {
		t.Fatal("third should fail")
	}
	if !l.Allow("b") {
		t.Fatal("other key should pass")
	}
}

func TestClientKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.9:1234"
	if ClientKey(r) != "203.0.113.9" {
		t.Fatalf("%s", ClientKey(r))
	}
	r.Header.Set("X-Forwarded-For", "198.51.100.2, 203.0.113.9")
	if ClientKey(r) != "198.51.100.2" {
		t.Fatalf("%s", ClientKey(r))
	}
}
