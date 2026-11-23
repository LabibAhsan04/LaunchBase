package security

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Limiter is a simple fixed-window counter per key.
type Limiter struct {
	mu      sync.Mutex
	window  time.Duration
	limit   int
	buckets map[string]*bucket
}

type bucket struct {
	count int
	start time.Time
}

func NewLimiter(perMinute int) *Limiter {
	if perMinute < 1 {
		perMinute = DefaultAPIRatePerMin
	}
	return &Limiter{
		window:  time.Minute,
		limit:   perMinute,
		buckets: map[string]*bucket{},
	}
}

func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		l.buckets[key] = &bucket{count: 1, start: now}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}

func ClientKey(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
