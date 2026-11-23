package auth

import "testing"

func TestAllowlistEmptyAllowsAll(t *testing.T) {
	t.Setenv("AUTH_ALLOWLIST", "")
	if !AllowedLogin("anyone") {
		t.Fatal("empty allowlist should allow")
	}
}

func TestAllowlistRestricts(t *testing.T) {
	t.Setenv("AUTH_ALLOWLIST", "Alice, bob")
	if !AllowedLogin("alice") || !AllowedLogin("BOB") {
		t.Fatal("expected allowlisted logins")
	}
	if AllowedLogin("eve") {
		t.Fatal("eve must be denied")
	}
}
