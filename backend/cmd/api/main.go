package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/httpapi"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/observability"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/queue"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://launchbase:launchbase@localhost:5433/launchbase?sslmode=disable"
		log.Printf("DATABASE_URL is unset, using the local compose database")
	}

	logger := observability.NewLogger("api", os.Stdout)
	metrics := observability.NewMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := store.Open(ctx, dsn)
	cancel()
	if err != nil {
		logger.Error("database_open_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	err = db.Migrate(ctx)
	cancel()
	if err != nil {
		logger.Error("migrate_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}

	stream, err := queue.Open(redisURL())
	if err != nil {
		logger.Error("redis_url_invalid", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer stream.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	if err := stream.Ping(ctx); err != nil {
		logger.Warn("redis_unreachable", map[string]any{"error": err.Error()})
	}
	cancel()
	if err := stream.EnsureGroup(context.Background()); err != nil {
		logger.Warn("redis_group", map[string]any{"error": err.Error()})
	}

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
	if gh == nil {
		logger.Info("github_unconfigured", map[string]any{"authMode": os.Getenv("AUTH_MODE")})
	}

	svc := &service.Service{
		Store:   db,
		Remover: dockerengine.Client{},
		Log:     observability.Writer{Log: logger},
		Logs:    broker,
		GitHub:  gh,
		Flush: func(ctx context.Context) error {
			_, err := db.PublishPending(ctx, func(payload []byte) error {
				return stream.Publish(ctx, payload)
			})
			return err
		},
	}
	go publishLoop(db, stream, metrics, logger)

	readyCheck := func(ctx context.Context) error {
		if err := db.Ping(ctx); err != nil {
			return err
		}
		return stream.Ping(ctx)
	}
	mux := httpapi.NewMuxWithOptions(svc, gh, httpapi.Options{
		Ready:   readyCheck,
		Metrics: metrics.Handler(),
	})
	handler := observability.HTTP(mux, logger, metrics)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api_listening", map[string]any{"addr": addr})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api_listen_failed", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("api_shutting_down", nil)
	shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shut); err != nil {
		logger.Error("api_shutdown_failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
}

func publishLoop(db *store.DB, stream *queue.Stream, metrics *observability.Metrics, logger *observability.Logger) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := db.PublishPending(ctx, func(payload []byte) error {
			return stream.Publish(ctx, payload)
		})
		if n, cerr := db.CountPendingOutbox(ctx); cerr == nil {
			metrics.SetQueueDepth(n)
		}
		cancel()
		if err != nil {
			logger.Error("queue_publish_failed", map[string]any{"error": err.Error()})
		}
	}
}

func redisURL() string {
	if v := strings.TrimSpace(os.Getenv("REDIS_URL")); v != "" {
		return v
	}
	return "redis://localhost:6379/0"
}
