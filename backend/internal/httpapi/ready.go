package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

func ready(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			if err := check(r.Context()); err != nil {
				writeReady(w, http.StatusServiceUnavailable, "not ready")
				return
			}
		}
		writeReady(w, http.StatusOK, "ok")
	}
}

func writeReady(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}
