package logs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Sink persists log rows. The broker assigns cleaned messages before writing.
type Sink interface {
	AppendLog(ctx context.Context, e Event) (Event, error)
	ListLogsAfter(ctx context.Context, deploymentID string, afterSeq int64, limit int) ([]Event, error)
}

// Hub fans events out to other API processes. Redis is the production hub.
type Hub interface {
	Publish(ctx context.Context, e Event) error
	Subscribe(ctx context.Context, deploymentID string) (<-chan Event, func(), error)
}

// MemoryHub is for tests. One process only.
type MemoryHub struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{}
}

func NewMemoryHub() *MemoryHub {
	return &MemoryHub{subs: map[string]map[chan Event]struct{}{}}
}

func (h *MemoryHub) Publish(_ context.Context, e Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[e.DeploymentID] {
		select {
		case ch <- e:
		default:
		}
	}
	return nil
}

func (h *MemoryHub) Subscribe(_ context.Context, deploymentID string) (<-chan Event, func(), error) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	if h.subs[deploymentID] == nil {
		h.subs[deploymentID] = map[chan Event]struct{}{}
	}
	h.subs[deploymentID][ch] = struct{}{}
	h.mu.Unlock()
	cancel := func() {
		h.mu.Lock()
		delete(h.subs[deploymentID], ch)
		h.mu.Unlock()
		close(ch)
	}
	return ch, cancel, nil
}

// Broker cleans, persists, and publishes deployment log events.
type Broker struct {
	Sink    Sink
	Hub     Hub
	Secrets []string
}

func (b *Broker) Append(ctx context.Context, deploymentID, source, level, message string) (Event, error) {
	if b.Sink == nil {
		return Event{}, fmt.Errorf("log sink is not configured")
	}
	e := Event{
		DeploymentID: deploymentID,
		TS:           time.Now().UTC(),
		Source:       source,
		Level:        level,
		Message:      Clean(message, b.Secrets),
	}
	stored, err := b.Sink.AppendLog(ctx, e)
	if err != nil {
		return Event{}, err
	}
	if b.Hub != nil {
		_ = b.Hub.Publish(ctx, stored)
	}
	return stored, nil
}

func (b *Broker) ListAfter(ctx context.Context, deploymentID string, afterSeq int64, limit int) ([]Event, error) {
	if b.Sink == nil {
		return nil, fmt.Errorf("log sink is not configured")
	}
	return b.Sink.ListLogsAfter(ctx, deploymentID, afterSeq, limit)
}

func (b *Broker) Subscribe(ctx context.Context, deploymentID string) (<-chan Event, func(), error) {
	if b.Hub == nil {
		return nil, func() {}, fmt.Errorf("log hub is not configured")
	}
	return b.Hub.Subscribe(ctx, deploymentID)
}
