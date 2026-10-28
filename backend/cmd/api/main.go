package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/httpapi"
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

	svc := &service.Service{
		Store:   db,
		Remover: dockerengine.Client{},
		Log:     os.Stdout,
		Limits: deployments.Options{
			CloneTimeout: envDuration("CLONE_TIMEOUT", time.Minute),
			BuildTimeout: envDuration("BUILD_TIMEOUT", 10*time.Minute),
			StartTimeout: envDuration("START_TIMEOUT", time.Minute),
			Memory:       envString("DEPLOYMENT_MEMORY_LIMIT", "512m"),
			CPUs:         envString("DEPLOYMENT_CPU_LIMIT", "1.0"),
			PIDs:         envString("DEPLOYMENT_PIDS_LIMIT", "128"),
		},
	}
	log.Printf("api listening on %s", addr)
	if err := http.ListenAndServe(addr, httpapi.NewMux(svc)); err != nil {
		log.Fatal(err)
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
		log.Fatalf("%s must be a duration like 60s or 10m", key)
	}
	return d
}
