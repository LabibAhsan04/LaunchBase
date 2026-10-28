package httpapi

import (
	"io"
	"net/http"
)

func (h handlers) createDeployment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Branch string `json:"branch"`
	}
	if r.ContentLength != 0 {
		if !decode(w, r, &body) {
			return
		}
	} else {
		_, _ = io.Copy(io.Discard, r.Body)
	}
	dep, err := h.svc.CreateDeployment(r.Context(), r.PathValue("projectId"), body.Branch)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, deploymentOut(dep))
}

func (h handlers) getDeployment(w http.ResponseWriter, r *http.Request) {
	dep, err := h.svc.GetDeployment(r.Context(), r.PathValue("deploymentId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deploymentOut(dep))
}

func (h handlers) stopDeployment(w http.ResponseWriter, r *http.Request) {
	dep, err := h.svc.StopDeployment(r.Context(), r.PathValue("deploymentId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deploymentOut(dep))
}
