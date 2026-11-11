package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
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
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://launchbase:launchbase@localhost:5433/launchbase?sslmode=disable"
		log.Printf("DATABASE_URL is unset, using the local compose database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := store.Open(ctx, dsn)
	cancel()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	if err := db.Migrate(ctx); err != nil {
		cancel()
		log.Fatalf("migrate: %v", err)
	}
	cancel()

	stream, err := queue.Open(redisURL())
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	defer stream.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	if err := stream.Ping(ctx); err != nil {
		cancel()
		log.Fatalf("redis: %v", err)
	}
	cancel()

	hub, err := logs.NewRedisHub(redisURL())
	if err != nil {
		log.Fatalf("log hub: %v", err)
	}
	defer hub.Close()
	broker := &logs.Broker{
		Sink:    db,
		Hub:     hub,
		Secrets: logs.SecretsFromEnv(),
	}

	name := workerName()
	proc := &deployments.Processor{
		Store:  db,
		Run:    deployments.Run,
		Remove: dockerengine.Client{},
		Log:    os.Stdout,
		Logs:   broker,
		Owner:  name,
		Limits: deployments.Options{
			CloneTimeout: envDuration("CLONE_TIMEOUT", time.Minute),
			BuildTimeout: envDuration("BUILD_TIMEOUT", 10*time.Minute),
			StartTimeout: envDuration("START_TIMEOUT", time.Minute),
			Memory:       envString("DEPLOYMENT_MEMORY_LIMIT", "512m"),
			CPUs:         envString("DEPLOYMENT_CPU_LIMIT", "1.0"),
			PIDs:         envString("DEPLOYMENT_PIDS_LIMIT", "128"),
		},
	}
	consumer := &queue.Consumer{
		Store:  db,
		Stream: stream,
		Handle: proc.Execute,
		Name:   name,
		Log:    os.Stdout,
	}

	addr := envString("WORKER_ADDR", ":8081")
	gate := &queue.Gate{}
	srv := &http.Server{
		Addr: addr,
		Handler: queue.Handler(gate, func(ctx context.Context) error {
			if err := stream.Ping(ctx); err != nil {
				return err
			}
			return db.Ping(ctx)
		}),
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("worker health: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		gate.Drain()
		log.Printf("worker shutting down after the current job")
	}()
	log.Printf("worker %s consuming %s, health on %s", name, queue.StreamName, addr)
	if err := consumer.Run(ctx); err != nil {
		log.Printf("worker: %v", err)
	}
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
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
