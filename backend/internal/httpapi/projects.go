package httpapi

import (
	"net/http"
)

func (h handlers) createProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		RepoURL string `json:"repoUrl"`
		Branch  string `json:"branch"`
	}
	if !decode(w, r, &body) {
		return
	}
	project, err := h.svc.CreateProject(r.Context(), body.Name, body.RepoURL, body.Branch)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, projectOut(project, nil))
}

func (h handlers) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.svc.ListProjects(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]projectJSON, 0, len(projects))
	for _, p := range projects {
		out = append(out, projectOut(p, nil))
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": out})
}

func (h handlers) getProject(w http.ResponseWriter, r *http.Request) {
	project, deps, err := h.svc.GetProject(r.Context(), r.PathValue("projectId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projectOut(project, deps))
}
