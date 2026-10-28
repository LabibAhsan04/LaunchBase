package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

type handlers struct {
	svc *service.Service
}

func NewMux(svc *service.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	if svc != nil {
		h := handlers{svc: svc}
		mux.HandleFunc("POST /api/v1/projects", h.createProject)
		mux.HandleFunc("GET /api/v1/projects", h.listProjects)
		mux.HandleFunc("GET /api/v1/projects/{projectId}", h.getProject)
		mux.HandleFunc("POST /api/v1/projects/{projectId}/deployments", h.createDeployment)
		mux.HandleFunc("GET /api/v1/deployments/{deploymentId}", h.getDeployment)
		mux.HandleFunc("POST /api/v1/deployments/{deploymentId}/stop", h.stopDeployment)
	}
	return requestIDs(mux)
}

func requestIDs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 80 {
			id = requestID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func requestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func writeServiceError(w http.ResponseWriter, err error) {
	var input service.InputError
	var conflict service.ConflictError
	switch {
	case errors.As(err, &input):
		writeError(w, http.StatusBadRequest, "invalid_request", input.Message)
	case errors.As(err, &conflict):
		writeError(w, http.StatusConflict, "conflict", conflict.Message)
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "not found")
	case errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", "that status change is not allowed")
	default:
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body is not valid json")
		return false
	}
	return true
}

type projectJSON struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	RepoURL     string           `json:"repoUrl"`
	Branch      string           `json:"branch"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	Deployments []deploymentJSON `json:"deployments,omitempty"`
}

type deploymentJSON struct {
	ID         string     `json:"id"`
	ProjectID  string     `json:"projectId"`
	Status     string     `json:"status"`
	RepoURL    string     `json:"repoUrl"`
	Branch     string     `json:"branch"`
	Image      string     `json:"image,omitempty"`
	Container  string     `json:"container,omitempty"`
	URL        string     `json:"url,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

func projectOut(p store.Project, deps []store.Deployment) projectJSON {
	out := projectJSON{
		ID: p.ID, Name: p.Name, RepoURL: p.RepoURL, Branch: p.Branch,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	if deps != nil {
		out.Deployments = make([]deploymentJSON, 0, len(deps))
		for _, d := range deps {
			out.Deployments = append(out.Deployments, deploymentOut(d))
		}
	}
	return out
}

func deploymentOut(d store.Deployment) deploymentJSON {
	return deploymentJSON{
		ID: d.ID, ProjectID: d.ProjectID, Status: d.Status, RepoURL: d.RepoURL, Branch: d.Branch,
		Image: d.ImageName, Container: d.ContainerName, URL: d.URL, Error: d.ErrorSummary,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, FinishedAt: d.FinishedAt,
	}
}
