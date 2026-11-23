package observability

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Logger writes one JSON object per line.
type Logger struct {
	mu  sync.Mutex
	out io.Writer
	svc string
}

func NewLogger(service string, out io.Writer) *Logger {
	if out == nil {
		out = os.Stdout
	}
	return &Logger{out: out, svc: service}
}

func (l *Logger) Info(msg string, fields map[string]any)  { l.write("info", msg, fields) }
func (l *Logger) Error(msg string, fields map[string]any) { l.write("error", msg, fields) }
func (l *Logger) Warn(msg string, fields map[string]any)  { l.write("warn", msg, fields) }

func (l *Logger) write(level, msg string, fields map[string]any) {
	row := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"level":   level,
		"service": l.svc,
		"msg":     msg,
	}
	for k, v := range fields {
		if v == nil {
			continue
		}
		row[k] = v
	}
	b, err := json.Marshal(row)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(append(b, '\n'))
}

// Writer adapts Logger to io.Writer for packages that log plain lines.
type Writer struct {
	Log    *Logger
	Fields map[string]any
}

func (w Writer) Write(p []byte) (int, error) {
	if w.Log == nil {
		return len(p), nil
	}
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	w.Log.Info(msg, w.Fields)
	return len(p), nil
}
