package security

import (
	"os"
	"strconv"
	"strings"
)

const (
	DefaultMaxBuildsPerUser = 2
	DefaultMaxBuildsGlobal  = 10
	DefaultMaxReadyPerUser  = 5
	DefaultAPIRatePerMin    = 120
	DefaultAuthRatePerMin   = 20
)

type Limits struct {
	MaxBuildsPerUser int
	MaxBuildsGlobal  int
	MaxReadyPerUser  int
	APIRatePerMin    int
	AuthRatePerMin   int
}

func LimitsFromEnv() Limits {
	return Limits{
		MaxBuildsPerUser: intEnv("MAX_BUILDS_PER_USER", DefaultMaxBuildsPerUser),
		MaxBuildsGlobal:  intEnv("MAX_BUILDS_GLOBAL", DefaultMaxBuildsGlobal),
		MaxReadyPerUser:  intEnv("MAX_READY_PER_USER", DefaultMaxReadyPerUser),
		APIRatePerMin:    intEnv("API_RATE_PER_MIN", DefaultAPIRatePerMin),
		AuthRatePerMin:   intEnv("AUTH_RATE_PER_MIN", DefaultAuthRatePerMin),
	}
}

func intEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

// BuildStatuses are counted toward concurrency limits.
var BuildStatuses = []string{
	"QUEUED", "CLONING", "DETECTING", "BUILDING", "STARTING", "HEALTH_CHECKING",
}

var ReadyStatuses = []string{"READY"}
