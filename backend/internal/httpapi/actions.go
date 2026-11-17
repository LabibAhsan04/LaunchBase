package httpapi

import (
	"net/http"
)

func (h handlers) redeployDeployment(w http.ResponseWriter, r *http.Request) {
	dep, err := h.svc.Redeploy(r.Context(), r.PathValue("deploymentId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, deploymentOut(dep))
}

func (h handlers) rollbackDeployment(w http.ResponseWriter, r *http.Request) {
	dep, err := h.svc.Rollback(r.Context(), r.PathValue("deploymentId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, deploymentOut(dep))
}
