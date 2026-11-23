package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type ctxKey string

const (
	traceKey   ctxKey = "lb.trace"
	spanKey    ctxKey = "lb.span"
	requestKey ctxKey = "lb.request"
)

// Trace design (OpenTelemetry-compatible without requiring the OTel SDK):
//
// 1. Inbound HTTP accepts W3C `traceparent` when present.
// 2. API generates a request ID (also exposed as X-Request-ID) and, if no
//    traceparent exists, a new 16-byte trace ID + 8-byte span ID.
// 3. Downstream worker jobs do not yet carry distributed context in Redis.
//    Job logs include deploymentId/projectId for correlation; adding
//    trace_id to the queue.Job payload is the documented next step for
//    full cross-process traces.
// 4. Exporters (OTLP) are intentionally omitted from the MVP to keep
//    dependencies small. /metrics and JSON logs are the local signal path.

type Trace struct {
	TraceID   string
	SpanID    string
	RequestID string
}

func WithTrace(ctx context.Context, t Trace) context.Context {
	ctx = context.WithValue(ctx, traceKey, t.TraceID)
	ctx = context.WithValue(ctx, spanKey, t.SpanID)
	ctx = context.WithValue(ctx, requestKey, t.RequestID)
	return ctx
}

func TraceFrom(ctx context.Context) Trace {
	t, _ := ctx.Value(traceKey).(string)
	s, _ := ctx.Value(spanKey).(string)
	r, _ := ctx.Value(requestKey).(string)
	return Trace{TraceID: t, SpanID: s, RequestID: r}
}

func ParseTraceparent(h string) (traceID, spanID string, ok bool) {
	// version-traceid-spanid-flags
	parts := strings.Split(strings.TrimSpace(h), "-")
	if len(parts) != 4 || parts[0] != "00" {
		return "", "", false
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func FormatTraceparent(traceID, spanID string) string {
	return "00-" + traceID + "-" + spanID + "-01"
}

func NewIDs() (traceID, spanID string) {
	var tb [16]byte
	var sb [8]byte
	_, _ = rand.Read(tb[:])
	_, _ = rand.Read(sb[:])
	return hex.EncodeToString(tb[:]), hex.EncodeToString(sb[:])
}

func TraceFromRequest(r *http.Request, requestID string) Trace {
	traceID, spanID, ok := ParseTraceparent(r.Header.Get("traceparent"))
	if !ok {
		traceID, spanID = NewIDs()
	}
	if requestID == "" {
		requestID = spanID
	}
	return Trace{TraceID: traceID, SpanID: spanID, RequestID: requestID}
}
