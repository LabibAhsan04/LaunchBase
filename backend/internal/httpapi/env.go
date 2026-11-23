package httpapi

import (
	"net/http"
	"strings"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
)

func (h handlers) listEnvVars(w http.ResponseWriter, r *http.Request) {
	env := strings.TrimSpace(r.URL.Query().Get("environment"))
	vars, err := h.svc.ListEnvVars(r.Context(), r.PathValue("projectId"), env)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"vars": vars})
}

func (h handlers) upsertEnvVar(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key         string `json:"key"`
		Value       string `json:"value"`
		Environment string `json:"environment"`
	}
	if !decode(w, r, &body) {
		return
	}
	view, err := h.svc.UpsertEnvVar(r.Context(), r.PathValue("projectId"), service.UpsertEnvInput{
		Key: body.Key, Value: body.Value, Environment: body.Environment,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h handlers) deleteEnvVar(w http.ResponseWriter, r *http.Request) {
	env := strings.TrimSpace(r.URL.Query().Get("environment"))
	err := h.svc.DeleteEnvVar(r.Context(), r.PathValue("projectId"), env, r.PathValue("key"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
