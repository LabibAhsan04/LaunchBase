package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/httpapi"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := store.Open(ctx, dsn)
	cancel()
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	err = db.Migrate(ctx)
	cancel()
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	stream, err := queue.Open(redisURL())
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	defer stream.Close()
	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	if err := stream.Ping(ctx); err != nil {
		log.Printf("redis is not reachable, queued deployments wait in postgres: %v", err)
	}
	cancel()
	if err := stream.EnsureGroup(context.Background()); err != nil {
		log.Printf("redis: %v", err)
	}

	svc := &service.Service{
		Store:   db,
		Remover: dockerengine.Client{},
		Log:     os.Stdout,
		Flush: func(ctx context.Context) error {
			_, err := db.PublishPending(ctx, func(payload []byte) error {
				return stream.Publish(ctx, payload)
			})
			return err
		},
	}
	go publishLoop(db, stream)

	log.Printf("api listening on %s", addr)
	if err := http.ListenAndServe(addr, httpapi.NewMux(svc)); err != nil {
		log.Fatal(err)
	}
}

func publishLoop(db *store.DB, stream *queue.Stream) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for range t.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := db.PublishPending(ctx, func(payload []byte) error {
			return stream.Publish(ctx, payload)
		})
		cancel()
		if err != nil {
			log.Printf("queue: %v", err)
		}
	}
}

func redisURL() string {
	if v := strings.TrimSpace(os.Getenv("REDIS_URL")); v != "" {
		return v
	}
	return "redis://localhost:6379/0"
}
