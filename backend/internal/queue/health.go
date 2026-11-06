package queue

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
)

// Gate is the worker's liveness flag. Draining means it has stopped reading new jobs.
type Gate struct {
	mu       sync.Mutex
	draining bool
}

func (g *Gate) Drain() {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.draining = true
	g.mu.Unlock()
}

func (g *Gate) Draining() bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.draining
}

func Handler(g *Gate, ready func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		if g.Draining() {
			status = "draining"
		}
		write(w, http.StatusOK, status)
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if g.Draining() {
			write(w, http.StatusServiceUnavailable, "draining")
			return
		}
		if ready != nil {
			if err := ready(r.Context()); err != nil {
				write(w, http.StatusServiceUnavailable, "not ready")
				return
			}
		}
		write(w, http.StatusOK, "ok")
	})
	return mux
}

func write(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}
