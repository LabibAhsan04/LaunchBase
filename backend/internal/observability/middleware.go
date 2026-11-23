package observability

import (
	"net/http"
	"strings"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (w *statusWriter) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// HTTP wraps handlers with structured access logs, metrics, and trace context.
func HTTP(next http.Handler, log *Logger, metrics *Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := r.Header.Get("X-Request-ID")
		tr := TraceFromRequest(r, reqID)
		if reqID == "" {
			reqID = tr.RequestID
		}
		r.Header.Set("X-Request-ID", reqID)
		w.Header().Set("X-Request-ID", reqID)
		w.Header().Set("traceparent", FormatTraceparent(tr.TraceID, tr.SpanID))
		sw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		ctx := WithTrace(r.Context(), Trace{TraceID: tr.TraceID, SpanID: tr.SpanID, RequestID: reqID})
		next.ServeHTTP(sw, r.WithContext(ctx))
		path := metricPath(r.URL.Path)
		dur := time.Since(start).Milliseconds()
		if metrics != nil {
			metrics.ObserveHTTP(r.Method, path, sw.code, dur)
		}
		if log != nil && !strings.HasPrefix(path, "/health") && path != "/metrics" && path != "/ready" {
			log.Info("http_request", map[string]any{
				"requestId":  reqID,
				"traceId":    tr.TraceID,
				"spanId":     tr.SpanID,
				"method":     r.Method,
				"path":       path,
				"status":     sw.code,
				"durationMs": dur,
			})
		}
	})
}

func metricPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if len(p) == 36 && strings.Count(p, "-") == 4 {
			parts[i] = ":id"
		}
	}
	out := strings.Join(parts, "/")
	if out == "" {
		return "/"
	}
	return out
}
