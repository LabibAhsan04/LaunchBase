package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
)

func main() {
	repo := flag.String("repo", "", "https://github.com/owner/name")
	branch := flag.String("branch", "", "branch to clone, defaults to the repo default")
	port := flag.Int("port", 0, "container port to publish, defaults to the exposed port")
	healthPath := flag.String("health-path", "/", "path to poll after start")
	flag.Parse()

	if strings.TrimSpace(*repo) == "" {
		fmt.Fprintln(os.Stderr, "usage: worker --repo https://github.com/owner/name [--branch main]")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, err := deployments.Run(ctx, deployments.Options{
		RepoURL:      *repo,
		Branch:       *branch,
		Port:         *port,
		HealthPath:   *healthPath,
		CloneTimeout: envDuration("CLONE_TIMEOUT", time.Minute),
		BuildTimeout: envDuration("BUILD_TIMEOUT", 10*time.Minute),
		StartTimeout: envDuration("START_TIMEOUT", time.Minute),
		Memory:       envString("DEPLOYMENT_MEMORY_LIMIT", "512m"),
		CPUs:         envString("DEPLOYMENT_CPU_LIMIT", "1.0"),
		PIDs:         envString("DEPLOYMENT_PIDS_LIMIT", "128"),
		Stdout:       os.Stdout,
		Runner:       dockerengine.Local{},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "deploy failed: %v\n", err)
		os.Exit(1)
	}
}

func envString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		fmt.Fprintf(os.Stderr, "%s must be a duration like 60s or 10m\n", key)
		os.Exit(2)
	}
	return d
}
