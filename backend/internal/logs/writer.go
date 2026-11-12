package logs

import (
	"bytes"
	"context"
	"io"
	"sync"
	"time"
)

// LineWriter turns runner stdout into structured log events.
type LineWriter struct {
	Broker       *Broker
	DeploymentID string
	Source       string
	Level        string

	mu  sync.Mutex
	buf []byte
}

func (w *LineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		w.emit(line)
	}
	return len(p), nil
}

func (w *LineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return
	}
	line := string(w.buf)
	w.buf = nil
	w.emit(line)
}

func (w *LineWriter) emit(line string) {
	if w.Broker == nil || w.DeploymentID == "" {
		return
	}
	level := w.Level
	if level == "" {
		level = LevelInfo
	}
	source := w.Source
	if source == "" {
		source = SourceWorker
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = w.Broker.Append(ctx, w.DeploymentID, source, level, line)
}

type teeWriter struct {
	console io.Writer
	line    *LineWriter
}

func (t *teeWriter) Write(p []byte) (int, error) {
	if t.console != nil {
		_, _ = t.console.Write(p)
	}
	return t.line.Write(p)
}

func (t *teeWriter) Flush() {
	t.line.Flush()
}

// Tee writes to both the console and the log broker.
func Tee(console io.Writer, broker *Broker, deploymentID string) io.Writer {
	line := &LineWriter{Broker: broker, DeploymentID: deploymentID, Source: SourceWorker, Level: LevelInfo}
	return &teeWriter{console: console, line: line}
}
