package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/auth"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/security"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

type handlers struct {
	svc *service.Service
	gh  *github.Client
}

func buildMux(svc *service.Service, gh *github.Client, opt Options) http.Handler {
	limits := security.LimitsFromEnv()
	if svc != nil && svc.Limits.MaxBuildsPerUser == 0 {
		svc.Limits = limits
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /ready", ready(opt.Ready))
	if opt.Metrics != nil {
		mux.Handle("GET /metrics", opt.Metrics)
	}
	if svc != nil {
		h := handlers{svc: svc, gh: gh}
		mux.HandleFunc("GET /api/v1/auth/login", h.authLogin)
		mux.HandleFunc("GET /api/v1/auth/callback", h.authCallback)
		mux.HandleFunc("POST /api/v1/auth/logout", h.authLogout)
		mux.HandleFunc("GET /api/v1/auth/logout", h.authLogout)
		mux.HandleFunc("GET /api/v1/auth/me", h.authMe)
		mux.HandleFunc("GET /api/v1/auth/install", h.authInstall)
		mux.HandleFunc("POST /api/v1/webhooks/github", h.githubWebhook)

		mux.HandleFunc("GET /api/v1/github/repos", h.requireAuth(h.listGitHubRepos))
		mux.HandleFunc("GET /api/v1/github/repos/{owner}/{repo}/branches", h.requireAuth(h.listGitHubBranches))
		mux.HandleFunc("GET /api/v1/github/installations", h.requireAuth(h.listInstallations))

		mux.HandleFunc("POST /api/v1/projects", h.requireAuth(h.createProject))
		mux.HandleFunc("GET /api/v1/projects", h.requireAuth(h.listProjects))
		mux.HandleFunc("GET /api/v1/projects/{projectId}", h.requireAuth(h.getProject))
		mux.HandleFunc("POST /api/v1/projects/{projectId}/deployments", h.requireAuth(h.createDeployment))
		mux.HandleFunc("GET /api/v1/deployments/{deploymentId}", h.requireAuth(h.getDeployment))
		mux.HandleFunc("POST /api/v1/deployments/{deploymentId}/stop", h.requireAuth(h.stopDeployment))
		mux.HandleFunc("POST /api/v1/deployments/{deploymentId}/redeploy", h.requireAuth(h.redeployDeployment))
		mux.HandleFunc("POST /api/v1/deployments/{deploymentId}/rollback", h.requireAuth(h.rollbackDeployment))
		mux.HandleFunc("GET /api/v1/deployments/{deploymentId}/logs", h.requireAuth(h.listLogs))
		mux.HandleFunc("GET /api/v1/deployments/{deploymentId}/logs/stream", h.requireAuth(h.streamLogs))
		mux.HandleFunc("GET /api/v1/ops/status", h.opsStatus)
	}

	devUser := auth.User{
		ID: store.LocalUserID, Login: "local", DisplayName: "Local Developer",
	}
	sessions := SessionStore(nil)
	if svc != nil && svc.Store != nil {
		sessions = svc.Store
	}
	withAuth := auth.Middleware{Sessions: sessions, DevUser: devUser}.Wrap(mux)
	apiLimit := security.NewLimiter(limits.APIRatePerMin)
	authLimit := security.NewLimiter(limits.AuthRatePerMin)
	return securityHeaders(cors(rateLimit(apiLimit, authLimit)(requestIDs(withAuth))))
}

type SessionStore = auth.SessionStore

func cors(next http.Handler) http.Handler {
	origin := strings.TrimSpace(os.Getenv("WEB_ORIGIN"))
	if origin == "" {
		origin = "http://localhost:3000"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID, Last-Event-ID, traceparent")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, traceparent")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
	var limited service.LimitError
	switch {
	case errors.Is(err, auth.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "sign in required")
	case errors.As(err, &limited):
		writeError(w, http.StatusTooManyRequests, "limit_exceeded", limited.Message)
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
	Slug        string           `json:"slug"`
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
	Hostname   string     `json:"hostname,omitempty"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

func projectOut(p store.Project, deps []store.Deployment) projectJSON {
	out := projectJSON{
		ID: p.ID, Name: p.Name, Slug: p.Slug, RepoURL: p.RepoURL, Branch: p.Branch,
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
		Image: d.ImageName, Container: d.ContainerName, URL: d.URL, Hostname: d.Hostname, Error: d.ErrorSummary,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, FinishedAt: d.FinishedAt,
	}
}
