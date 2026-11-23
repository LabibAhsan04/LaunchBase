package dockerengine

import (
	"os"
	"strconv"
	"strings"
)

// SecurityOpts are applied to every user runtime container.
// They never mount the Docker socket, never use host networking, and never
// attach to the infrastructure network that holds Postgres/Redis.
type SecurityOpts struct {
	NoNewPrivileges bool
	DropAllCaps     bool
	MemorySwapMatch bool
	ReadOnlyRoot    bool
}

func DefaultSecurityOpts() SecurityOpts {
	return SecurityOpts{
		NoNewPrivileges: true,
		DropAllCaps:     true,
		MemorySwapMatch: true,
		ReadOnlyRoot:    true,
	}
}

// AppendSecurityArgs adds hardening flags. Callers must still set --network to
// the runtime network (never launchbase / host / none while Traefik must route).
func AppendSecurityArgs(args []string, memory string, opts SecurityOpts) []string {
	if opts.NoNewPrivileges {
		args = append(args, "--security-opt", "no-new-privileges:true")
	}
	if opts.DropAllCaps {
		args = append(args, "--cap-drop", "ALL")
	}
	if opts.MemorySwapMatch && memory != "" {
		// Cap swap to the memory limit so a container cannot expand via swap.
		args = append(args, "--memory-swap", memory)
	}
	return args
}

// AssertSafeRunArgs fails tests when forbidden options appear.
func AssertSafeRunArgs(args []string) error {
	joined := strings.Join(args, " ")
	forbidden := []string{
		"--privileged",
		"docker.sock",
		"--network host",
		"--network=host",
		"--pid host",
		"--ipc host",
		"--volume /",
		"-v /var/run/docker.sock",
	}
	for _, f := range forbidden {
		if strings.Contains(joined, f) {
			return errUnsafe(f)
		}
	}
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "-e" || args[i] == "--env" {
			key := strings.SplitN(args[i+1], "=", 2)[0]
			if isLaunchBaseEnv(key) {
				return errUnsafe("env " + key)
			}
		}
	}
	return nil
}

func isLaunchBaseEnv(key string) bool {
	k := strings.ToUpper(key)
	switch k {
	case "DATABASE_URL", "REDIS_URL", "SESSION_SECRET", "ENV_ENCRYPTION_KEY",
		"GITHUB_CLIENT_SECRET", "GITHUB_APP_PRIVATE_KEY", "GITHUB_WEBHOOK_SECRET",
		"OPS_TOKEN", "PATH", "LD_PRELOAD", "LD_LIBRARY_PATH":
		return true
	default:
		return strings.HasPrefix(k, "LAUNCHBASE_") || strings.HasPrefix(k, "DOCKER_")
	}
}

type unsafeArgError string

func errUnsafe(s string) error { return unsafeArgError(s) }

func (e unsafeArgError) Error() string { return "unsafe docker arg: " + string(e) }

// StorageLimitBytes is an optional image/layer soft documentation helper.
// Local Docker Desktop does not reliably enforce --storage-opt size= for overlay2;
// see docs/security.md. Kept for env discovery and future drivers.
func StorageLimitBytes() int64 {
	v := strings.TrimSpace(os.Getenv("DEPLOYMENT_STORAGE_LIMIT"))
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
