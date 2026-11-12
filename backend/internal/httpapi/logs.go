package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
)

func (h handlers) listLogs(w http.ResponseWriter, r *http.Request) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.svc.ListLogs(r.Context(), r.PathValue("deploymentId"), after, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]logJSON, 0, len(events))
	for _, e := range events {
		out = append(out, logOut(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": out})
}

func (h handlers) streamLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming is not supported")
		return
	}
	deploymentID := r.PathValue("deploymentId")
	dep, err := h.svc.GetDeployment(r.Context(), deploymentID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	after := parseAfter(r)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	if h.svc.Logs == nil {
		writeSSE(w, flusher, 0, "error", `{"message":"logs are not configured"}`)
		return
	}

	events, err := h.svc.ListLogs(r.Context(), deploymentID, after, 500)
	if err != nil {
		writeSSE(w, flusher, 0, "error", `{"message":"could not load logs"}`)
		return
	}
	last := after
	done := false
	for _, e := range events {
		writeLogSSE(w, flusher, e)
		last = e.Seq
		if e.Level == logs.LevelDone {
			done = true
		}
	}
	if done || isTerminal(dep.Status) {
		if !done {
			writeSSE(w, flusher, last, "done", fmt.Sprintf(`{"status":%q}`, dep.Status))
		}
		return
	}

	subCtx := r.Context()
	live, cancel, err := h.svc.Logs.Subscribe(subCtx, deploymentID)
	if err != nil {
		writeSSE(w, flusher, last, "error", `{"message":"could not subscribe to live logs"}`)
		return
	}
	defer cancel()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	catchup := time.NewTicker(2 * time.Second)
	defer catchup.Stop()

	for {
		select {
		case <-subCtx.Done():
			return
		case <-heartbeat.C:
			writeSSE(w, flusher, 0, "heartbeat", `{}`)
		case e, ok := <-live:
			if !ok {
				return
			}
			if e.Seq <= last {
				continue
			}
			writeLogSSE(w, flusher, e)
			last = e.Seq
			if e.Level == logs.LevelDone {
				return
			}
		case <-catchup.C:
			more, err := h.svc.ListLogs(subCtx, deploymentID, last, 200)
			if err != nil {
				continue
			}
			for _, e := range more {
				writeLogSSE(w, flusher, e)
				last = e.Seq
				if e.Level == logs.LevelDone {
					return
				}
			}
			dep, err := h.svc.GetDeployment(subCtx, deploymentID)
			if err == nil && isTerminal(dep.Status) {
				writeSSE(w, flusher, last, "done", fmt.Sprintf(`{"status":%q}`, dep.Status))
				return
			}
		}
	}
}

func parseAfter(r *http.Request) int64 {
	if v := r.Header.Get("Last-Event-ID"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return n
		}
	}
	n, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	return n
}

func writeLogSSE(w http.ResponseWriter, flusher http.Flusher, e logs.Event) {
	raw, _ := json.Marshal(logOut(e))
	writeSSE(w, flusher, e.Seq, "log", string(raw))
	if e.Level == logs.LevelDone {
		writeSSE(w, flusher, e.Seq, "done", fmt.Sprintf(`{"status":%q,"message":%q}`, e.Message, e.Message))
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, id int64, event, data string) {
	if id > 0 {
		fmt.Fprintf(w, "id: %d\n", id)
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

func isTerminal(status string) bool {
	switch status {
	case "READY", "FAILED", "STOPPED", "SUPERSEDED":
		return true
	default:
		return false
	}
}

type logJSON struct {
	Seq          int64     `json:"seq"`
	DeploymentID string    `json:"deploymentId"`
	TS           time.Time `json:"ts"`
	Source       string    `json:"source"`
	Level        string    `json:"level"`
	Message      string    `json:"message"`
}

func logOut(e logs.Event) logJSON {
	return logJSON{
		Seq: e.Seq, DeploymentID: e.DeploymentID, TS: e.TS.UTC(),
		Source: e.Source, Level: e.Level, Message: e.Message,
	}
}
