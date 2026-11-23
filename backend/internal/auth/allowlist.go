package auth

import (
	"os"
	"strings"
)

// Allowlist returns GitHub logins permitted to sign in when AUTH_ALLOWLIST is set.
// Empty means no extra restriction beyond AUTH_MODE.
func Allowlist() map[string]struct{} {
	raw := strings.TrimSpace(os.Getenv("AUTH_ALLOWLIST"))
	if raw == "" {
		return nil
	}
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		login := strings.ToLower(strings.TrimSpace(part))
		if login != "" {
			out[login] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func AllowedLogin(login string) bool {
	list := Allowlist()
	if list == nil {
		return true
	}
	_, ok := list[strings.ToLower(strings.TrimSpace(login))]
	return ok
}
