package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/observability"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/queue"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

func main() {
	repo := flag.String("repo", "", "https://github.com/owner/name")
	branch := flag.String("branch", "", "branch to clone, defaults to the repo default")
	port := flag.Int("port", 0, "container port to publish, defaults to the exposed port")
	healthPath := flag.String("health-path", "/", "path to poll after start")
	flag.Parse()

	if strings.TrimSpace(*repo) != "" {
		runOnce(*repo, *branch, *port, *healthPath)
		return
	}
	runQueue()
}

func runOnce(repo, branch string, port int, healthPath string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, err := deployments.Run(ctx, deployments.Options{
		RepoURL:      repo,
		Branch:       branch,
		Port:         port,
		HealthPath:   healthPath,
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

func runQueue() {
	logger := observability.NewLogger("worker", os.Stdout)
	metrics := observability.NewMetrics()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://launchbase:launchbase@localhost:5433/launchbase?sslmode=disable"
		logger.Info("database_url_default", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := store.Open(ctx, dsn)
	cancel()
	if err != nil {
		logger.Error("database_open_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	if err := db.Migrate(ctx); err != nil {
		cancel()
		logger.Error("migrate_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	cancel()

	stream, err := queue.Open(redisURL())
	if err != nil {
		logger.Error("redis_url_invalid", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer stream.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := stream.Ping(ctx); err != nil {
		cancel()
		logger.Error("redis_unreachable", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	cancel()

	hub, err := logs.NewRedisHub(redisURL())
	if err != nil {
		logger.Error("log_hub_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer hub.Close()
	broker := &logs.Broker{
		Sink:    db,
		Hub:     hub,
		Secrets: logs.SecretsFromEnv(),
	}

	gh, err := github.ClientFromEnv()
	if err != nil {
		logger.Error("github_config_invalid", map[string]any{"error": err.Error()})
		os.Exit(1)
	}

	name := workerName()
	engine := dockerengine.Client{}
	logWriter := observability.Writer{Log: logger}
	proc := &deployments.Processor{
		Store:  db,
		Run:    deployments.Run,
		Remove: engine,
		Images: engine,
		Log:    logWriter,
		Logs:   broker,
		Owner:  name,
		Tokens: github.InstallationTokens{Client: gh},
		Limits: deployments.Options{
			CloneTimeout: envDuration("CLONE_TIMEOUT", time.Minute),
			BuildTimeout: envDuration("BUILD_TIMEOUT", 10*time.Minute),
			StartTimeout: envDuration("START_TIMEOUT", time.Minute),
			Memory:       envString("DEPLOYMENT_MEMORY_LIMIT", "512m"),
			CPUs:         envString("DEPLOYMENT_CPU_LIMIT", "1.0"),
			PIDs:         envString("DEPLOYMENT_PIDS_LIMIT", "128"),
			BaseDomain:   envString("BASE_DOMAIN", dockerengine.DefaultBaseDomain),
			Network:      envString("RUNTIME_NETWORK", dockerengine.DefaultRuntimeNetwork),
		},
	}
	consumer := &queue.Consumer{
		Store:  db,
		Stream: stream,
		Handle: func(ctx context.Context, job queue.Job) (queue.Result, error) {
			start := time.Now()
			res, err := proc.Execute(ctx, job)
			outcome := "ok"
			switch {
			case err != nil:
				outcome = "error"
			case res.Dead:
				outcome = "failed"
			case res.Busy:
				outcome = "busy"
			}
			dur := time.Since(start).Milliseconds()
			metrics.ObserveJob(outcome, dur)
			fields := map[string]any{
				"deploymentId": job.DeploymentID,
				"projectId":    job.ProjectID,
				"outcome":      outcome,
				"durationMs":   dur,
				"attempt":      job.Attempt,
			}
			if err != nil {
				fields["error"] = err.Error()
				logger.Error("job_finished", fields)
			} else {
				logger.Info("job_finished", fields)
			}
			return res, err
		},
		Name: name,
		Log:  logWriter,
	}

	addr := envString("WORKER_ADDR", ":8081")
	gate := &queue.Gate{}
	srv := &http.Server{
		Addr: addr,
		Handler: queue.HandlerWithMetrics(gate, func(ctx context.Context) error {
			if err := stream.Ping(ctx); err != nil {
				return err
			}
			return db.Ping(ctx)
		}, metrics.Handler()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("worker_health_failed", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
	}()
	go sampleWorkerMetrics(db, metrics)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		gate.Drain()
		logger.Info("worker_draining", map[string]any{"worker": name})
	}()
	logger.Info("worker_started", map[string]any{"worker": name, "stream": queue.StreamName, "health": addr})
	if err := consumer.Run(ctx); err != nil {
		logger.Error("worker_stopped", map[string]any{"error": err.Error()})
	}
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
}

func sampleWorkerMetrics(db *store.DB, metrics *observability.Metrics) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if n, err := db.CountPendingOutbox(ctx); err == nil {
			metrics.SetQueueDepth(n)
		}
		if n, err := db.CountDeploymentsByStatus(ctx, "", []string{"READY"}); err == nil {
			metrics.SetActiveContainers(int64(n))
		}
		cancel()
	}
}

func workerName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "worker"
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	return host + "-" + hex.EncodeToString(b[:])
}

func redisURL() string {
	if v := strings.TrimSpace(os.Getenv("REDIS_URL")); v != "" {
		return v
	}
	return "redis://localhost:6379/0"
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
