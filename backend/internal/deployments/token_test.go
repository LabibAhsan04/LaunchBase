package deployments

import (
	"strings"
	"testing"
)

func TestCloneArgsKeepsTokenOutOfRemoteURL(t *testing.T) {
	url := "https://github.com/acme/private.git"
	args := cloneArgs(url, "main", "/tmp/src", "ghs_secret_token")
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "ghs_secret_token@") {
		t.Fatalf("token leaked into url form: %s", joined)
	}
	if args[len(args)-2] != url {
		t.Fatalf("clone url changed: %v", args)
	}
	if !strings.Contains(args[1], "Authorization: Bearer ghs_secret_token") {
		t.Fatalf("missing auth header: %v", args)
	}
}
