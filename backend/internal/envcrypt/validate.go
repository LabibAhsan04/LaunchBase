package envcrypt

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxEnvVarsPerProject = 50
	MaxEnvValueBytes     = 4096
)

var keyRE = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func ValidateKey(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("env key is required")
	}
	if len(key) > 64 {
		return fmt.Errorf("env key is too long")
	}
	if !keyRE.MatchString(key) {
		return fmt.Errorf("env key must look like NAME or NAME_WITH_UNDERSCORES")
	}
	if ReservedKey(key) {
		return fmt.Errorf("env key %s is reserved", key)
	}
	return nil
}

func ValidateValue(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("env value must be valid utf-8")
	}
	if len(value) > MaxEnvValueBytes {
		return fmt.Errorf("env value is too large")
	}
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("env value must not contain null bytes")
	}
	return nil
}

func ReservedKey(key string) bool {
	k := strings.ToUpper(strings.TrimSpace(key))
	switch k {
	case "DATABASE_URL", "REDIS_URL", "SESSION_SECRET", "ENV_ENCRYPTION_KEY",
		"GITHUB_CLIENT_SECRET", "GITHUB_APP_PRIVATE_KEY", "GITHUB_WEBHOOK_SECRET",
		"OPS_TOKEN", "PATH", "LD_PRELOAD", "LD_LIBRARY_PATH":
		return true
	default:
		return strings.HasPrefix(k, "LAUNCHBASE_") || strings.HasPrefix(k, "DOCKER_")
	}
}

// Mask returns a UI-safe hint (never the full secret).
func Mask(value string) string {
	n := utf8.RuneCountInString(value)
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return "••••"
	}
	runes := []rune(value)
	return "••••" + string(runes[len(runes)-4:])
}
