package logs

import (
	"os"
	"regexp"
	"strings"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

var secretKV = regexp.MustCompile(`(?i)(password|passwd|token|secret|api[_-]?key|authorization)\s*[=:]\s*([^\s,;]+)`)

// Clean strips ANSI and redacts configured secret values from a log line.
func Clean(message string, extra []string) string {
	message = ansiRE.ReplaceAllString(message, "")
	message = strings.ReplaceAll(message, "\r", "")
	message = strings.TrimRight(message, "\n")
	if len(message) > 4000 {
		message = message[:4000]
	}
	message = secretKV.ReplaceAllString(message, "$1=[redacted]")
	for _, secret := range extra {
		secret = strings.TrimSpace(secret)
		if secret == "" || len(secret) < 6 {
			continue
		}
		message = strings.ReplaceAll(message, secret, "[redacted]")
	}
	return message
}

// SecretsFromEnv returns values that must never appear in logs.
func SecretsFromEnv() []string {
	keys := []string{
		"SESSION_SECRET",
		"DATABASE_URL",
		"REDIS_URL",
		"GITHUB_CLIENT_SECRET",
		"GITHUB_APP_PRIVATE_KEY",
	}
	if extra := strings.TrimSpace(os.Getenv("LOG_REDACT_ENV")); extra != "" {
		for _, key := range strings.Split(extra, ",") {
			key = strings.TrimSpace(key)
			if key != "" {
				keys = append(keys, key)
			}
		}
	}
	out := make([]string, 0, len(keys))
	seen := map[string]bool{}
	for _, key := range keys {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if u := stripURLUserInfo(v); u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

func stripURLUserInfo(raw string) string {
	// postgres://user:pass@host/db -> user:pass@host fragment for password part
	if i := strings.Index(raw, "://"); i >= 0 {
		rest := raw[i+3:]
		if at := strings.Index(rest, "@"); at > 0 {
			cred := rest[:at]
			if colon := strings.Index(cred, ":"); colon >= 0 {
				return cred[colon+1:]
			}
		}
	}
	return ""
}
