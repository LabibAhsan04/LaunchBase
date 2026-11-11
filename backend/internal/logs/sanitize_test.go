package logs

import "testing"

func TestCleanStripsANSIAndSecrets(t *testing.T) {
	in := "\x1b[32mok\x1b[0m password=supersecret token: abcdefghijkl"
	out := Clean(in, nil)
	if out != "ok password=[redacted] token=[redacted]" {
		t.Fatalf("%q", out)
	}
}

func TestCleanRedactsConfiguredValues(t *testing.T) {
	out := Clean("dsn=postgres://launchbase:s3cret@localhost:5433/db", []string{"s3cret"})
	if out != "dsn=postgres://launchbase:[redacted]@localhost:5433/db" {
		t.Fatalf("%q", out)
	}
}

func TestSecretsFromEnvIncludesPasswordPart(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://launchbase:hunter2pass@localhost:5433/launchbase")
	t.Setenv("SESSION_SECRET", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("LOG_REDACT_ENV", "")
	secrets := SecretsFromEnv()
	found := false
	for _, s := range secrets {
		if s == "hunter2pass" {
			found = true
		}
	}
	if !found {
		t.Fatalf("secrets %#v", secrets)
	}
}
