package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/auth"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/queue"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/security"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

// InputError is a request the client can fix.
type InputError struct {
	Message string
}

func (e InputError) Error() string { return e.Message }

// ConflictError is a request that lost a race or asked for an illegal change.
type ConflictError struct {
	Message string
}

func (e ConflictError) Error() string { return e.Message }

// Remover stops a container. The HTTP handler does not build or start one.
type Remover interface {
	RemoveContainer(ctx context.Context, name string) error
}

type Service struct {
	Store   *store.DB
	Remover Remover
	Log     io.Writer
	Logs    *logs.Broker
	GitHub  *github.Client
	Limits  security.Limits
	// Flush publishes one outbox row. The deployment is already committed if this fails.
	Flush func(context.Context) error
}

// LimitError is returned when concurrency caps are hit.
type LimitError struct {
	Message string
}

func (e LimitError) Error() string { return e.Message }

type CreateProjectInput struct {
	Name     string
	RepoURL  string
	FullName string
	Branch   string
}

func ownerID(ctx context.Context) (string, error) {
	u, ok := auth.UserFrom(ctx)
	if !ok || u.ID == "" {
		return "", auth.ErrUnauthorized
	}
	return u.ID, nil
}

func (s *Service) CreateProject(ctx context.Context, in CreateProjectInput) (store.Project, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return store.Project{}, err
	}
	name := strings.TrimSpace(in.Name)
	if !validProjectName(name) {
		return store.Project{}, InputError{Message: "name must be 1-40 letters, numbers, spaces, dashes, or underscores"}
	}
	branch, err := github.ParseBranch(strings.TrimSpace(in.Branch))
	if err != nil {
		return store.Project{}, InputError{Message: err.Error()}
	}

	var installationID string
	var githubRepoID int64
	var cloneURL string

	fullName := strings.TrimSpace(in.FullName)
	if fullName == "" && strings.TrimSpace(in.RepoURL) != "" {
		parsed, err := github.ParseRepoURL(in.RepoURL)
		if err != nil {
			return store.Project{}, InputError{Message: err.Error()}
		}
		fullName = parsed.Owner + "/" + parsed.Name
		cloneURL = parsed.CloneURL
	}

	if fullName != "" {
		access, err := s.Store.GetRepoForUser(ctx, owner, fullName)
		if err == nil {
			installationID = access.InstallationID
			githubRepoID = access.GitHubRepoID
			cloneURL = "https://github.com/" + access.FullName + ".git"
			if branch == "" {
				branch = access.DefaultBranch
			}
		} else if !errors.Is(err, store.ErrNotFound) {
			return store.Project{}, err
		} else if auth.RequiresGitHub() {
			return store.Project{}, InputError{Message: "repository is not available through your GitHub App installation"}
		}
	}

	if cloneURL == "" && fullName != "" {
		if _, err := github.ParseRepoURL("https://github.com/" + fullName); err != nil {
			return store.Project{}, InputError{Message: "repository full name is invalid"}
		}
		cloneURL = "https://github.com/" + fullName + ".git"
	}
	if cloneURL == "" {
		repo, err := github.ParseRepoURL(in.RepoURL)
		if err != nil {
			return store.Project{}, InputError{Message: err.Error()}
		}
		cloneURL = repo.CloneURL
	}
	if auth.RequiresGitHub() && installationID == "" {
		return store.Project{}, InputError{Message: "select a repository from your GitHub App installation"}
	}

	id, err := newID()
	if err != nil {
		return store.Project{}, err
	}
	slug, err := s.uniqueSlug(ctx, name, id)
	if err != nil {
		return store.Project{}, err
	}
	project, err := s.Store.CreateProject(ctx, store.Project{
		ID:             id,
		OwnerID:        owner,
		Name:           name,
		Slug:           slug,
		RepoURL:        cloneURL,
		Branch:         branch,
		InstallationID: installationID,
		GitHubRepoID:   githubRepoID,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return store.Project{}, ConflictError{Message: "a project with that name already exists"}
		}
		return store.Project{}, err
	}
	_ = s.audit(ctx, owner, "project.created", project.ID, "", project.RepoURL)
	return project, nil
}

func (s *Service) ListProjects(ctx context.Context) ([]store.Project, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	return s.Store.ListProjects(ctx, owner)
}

func (s *Service) GetProject(ctx context.Context, id string) (store.Project, []store.Deployment, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return store.Project{}, nil, err
	}
	if !validUUID(id) {
		return store.Project{}, nil, store.ErrNotFound
	}
	project, err := s.Store.GetProject(ctx, owner, id)
	if err != nil {
		return store.Project{}, nil, err
	}
	deps, err := s.Store.ListDeployments(ctx, owner, id)
	if err != nil {
		return store.Project{}, nil, err
	}
	return project, deps, nil
}

func (s *Service) CreateDeployment(ctx context.Context, projectID, branch string) (store.Deployment, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return store.Deployment{}, err
	}
	if !validUUID(projectID) {
		return store.Deployment{}, store.ErrNotFound
	}
	project, err := s.Store.GetProject(ctx, owner, projectID)
	if err != nil {
		return store.Deployment{}, err
	}
	if auth.RequiresGitHub() && project.InstallationID == "" {
		_ = s.audit(ctx, owner, "permission.denied", project.ID, "", "deploy without installation")
		return store.Deployment{}, InputError{Message: "project is not linked to a GitHub App installation"}
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = project.Branch
	}
	branch, err = github.ParseBranch(branch)
	if err != nil {
		return store.Deployment{}, InputError{Message: err.Error()}
	}
	if err := s.checkConcurrency(ctx, owner); err != nil {
		return store.Deployment{}, err
	}

	var installationGHID int64
	if project.InstallationID != "" {
		inst, err := s.Store.GetInstallation(ctx, project.InstallationID)
		if err != nil {
			return store.Deployment{}, err
		}
		if inst.Suspended {
			return store.Deployment{}, InputError{Message: "GitHub App installation is suspended"}
		}
		installationGHID = inst.InstallationID
	}

	id, err := newID()
	if err != nil {
		return store.Deployment{}, err
	}
	outboxID, err := newID()
	if err != nil {
		return store.Deployment{}, err
	}
	payload, err := json.Marshal(queue.Job{
		Version:        queue.JobVersion,
		DeploymentID:   id,
		ProjectID:      project.ID,
		RepoURL:        project.RepoURL,
		Branch:         branch,
		Attempt:        1,
		InstallationID: installationGHID,
	})
	if err != nil {
		return store.Deployment{}, err
	}
	dep, err := s.Store.CreateDeployment(ctx, store.Deployment{
		ID:        id,
		ProjectID: project.ID,
		Status:    string(deployments.StatusQueued),
		RepoURL:   project.RepoURL,
		Branch:    branch,
	}, outboxID, payload)
	if err != nil {
		return store.Deployment{}, err
	}
	_ = s.audit(ctx, owner, "deployment.created", project.ID, dep.ID, dep.RepoURL)
	if s.Flush != nil {
		if err := s.Flush(ctx); err != nil && s.Log != nil {
			fmt.Fprintf(s.Log, "queue: %v\n", err)
		}
	}
	return dep, nil
}

func (s *Service) GetDeployment(ctx context.Context, id string) (store.Deployment, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return store.Deployment{}, err
	}
	if !validUUID(id) {
		return store.Deployment{}, store.ErrNotFound
	}
	return s.Store.GetDeployment(ctx, owner, id)
}

func (s *Service) ListLogs(ctx context.Context, deploymentID string, afterSeq int64, limit int) ([]logs.Event, error) {
	if _, err := s.GetDeployment(ctx, deploymentID); err != nil {
		return nil, err
	}
	if s.Logs == nil {
		return []logs.Event{}, nil
	}
	return s.Logs.ListAfter(ctx, deploymentID, afterSeq, limit)
}

func (s *Service) StopDeployment(ctx context.Context, id string) (store.Deployment, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return store.Deployment{}, err
	}
	dep, err := s.GetDeployment(ctx, id)
	if err != nil {
		return store.Deployment{}, err
	}
	if !deployments.CanTransition(deployments.Status(dep.Status), deployments.StatusStopped) {
		return store.Deployment{}, store.ErrConflict
	}
	if dep.ContainerName != "" && s.Remover != nil {
		if err := s.Remover.RemoveContainer(ctx, dep.ContainerName); err != nil {
			return store.Deployment{}, fmt.Errorf("stop container: %w", err)
		}
	}
	if err := s.Store.Transition(ctx, dep.ID, dep.Status, string(deployments.StatusStopped)); err != nil {
		return store.Deployment{}, err
	}
	_ = s.audit(ctx, owner, "deployment.stopped", dep.ProjectID, dep.ID, dep.ContainerName)
	return s.Store.GetDeployment(ctx, owner, dep.ID)
}

// Redeploy queues a new deployment from the project's current repo/branch settings.
func (s *Service) Redeploy(ctx context.Context, deploymentID string) (store.Deployment, error) {
	dep, err := s.GetDeployment(ctx, deploymentID)
	if err != nil {
		return store.Deployment{}, err
	}
	branch := dep.Branch
	out, err := s.CreateDeployment(ctx, dep.ProjectID, branch)
	if err != nil {
		return store.Deployment{}, err
	}
	owner, _ := ownerID(ctx)
	_ = s.audit(ctx, owner, "deployment.redeployed", dep.ProjectID, out.ID, dep.ID)
	return out, nil
}

// Rollback queues a new deployment using the immutable repo/branch of a previous deployment.
func (s *Service) Rollback(ctx context.Context, deploymentID string) (store.Deployment, error) {
	dep, err := s.GetDeployment(ctx, deploymentID)
	if err != nil {
		return store.Deployment{}, err
	}
	switch deployments.Status(dep.Status) {
	case deployments.StatusReady, deployments.StatusStopped, deployments.StatusSuperseded, deployments.StatusFailed:
	default:
		return store.Deployment{}, InputError{Message: "rollback requires a finished deployment"}
	}
	owner, err := ownerID(ctx)
	if err != nil {
		return store.Deployment{}, err
	}
	project, err := s.Store.GetProject(ctx, owner, dep.ProjectID)
	if err != nil {
		return store.Deployment{}, err
	}

	if err := s.checkConcurrency(ctx, owner); err != nil {
		return store.Deployment{}, err
	}
	id, err := newID()
	if err != nil {
		return store.Deployment{}, err
	}
	outboxID, err := newID()
	if err != nil {
		return store.Deployment{}, err
	}
	var installationGHID int64
	if project.InstallationID != "" {
		inst, err := s.Store.GetInstallation(ctx, project.InstallationID)
		if err == nil && !inst.Suspended {
			installationGHID = inst.InstallationID
		}
	}
	payload, err := json.Marshal(queue.Job{
		Version:        queue.JobVersion,
		DeploymentID:   id,
		ProjectID:      project.ID,
		RepoURL:        dep.RepoURL,
		Branch:         dep.Branch,
		Attempt:        1,
		InstallationID: installationGHID,
	})
	if err != nil {
		return store.Deployment{}, err
	}
	out, err := s.Store.CreateDeployment(ctx, store.Deployment{
		ID:        id,
		ProjectID: project.ID,
		Status:    string(deployments.StatusQueued),
		RepoURL:   dep.RepoURL,
		Branch:    dep.Branch,
	}, outboxID, payload)
	if err != nil {
		return store.Deployment{}, err
	}
	_ = s.audit(ctx, owner, "deployment.rollback", project.ID, out.ID, dep.ID)
	if s.Flush != nil {
		if err := s.Flush(ctx); err != nil && s.Log != nil {
			fmt.Fprintf(s.Log, "queue: %v\n", err)
		}
	}
	return out, nil
}

func (s *Service) checkConcurrency(ctx context.Context, owner string) error {
	limits := s.Limits
	if limits.MaxBuildsPerUser == 0 {
		limits = security.LimitsFromEnv()
	}
	userBuilds, err := s.Store.CountDeploymentsByStatus(ctx, owner, security.BuildStatuses)
	if err != nil {
		return err
	}
	if userBuilds >= limits.MaxBuildsPerUser {
		return LimitError{Message: "too many active builds for this user"}
	}
	globalBuilds, err := s.Store.CountDeploymentsByStatus(ctx, "", security.BuildStatuses)
	if err != nil {
		return err
	}
	if globalBuilds >= limits.MaxBuildsGlobal {
		return LimitError{Message: "platform build capacity is full, try again shortly"}
	}
	ready, err := s.Store.CountDeploymentsByStatus(ctx, owner, security.ReadyStatuses)
	if err != nil {
		return err
	}
	if ready >= limits.MaxReadyPerUser {
		return LimitError{Message: "too many ready deployments; stop one before starting another"}
	}
	if limits.MaxReadyGlobal > 0 {
		globalReady, err := s.Store.CountDeploymentsByStatus(ctx, "", security.ReadyStatuses)
		if err != nil {
			return err
		}
		if globalReady >= limits.MaxReadyGlobal {
			return LimitError{Message: "platform ready capacity is full; stop an old deployment first"}
		}
	}
	return nil
}

func (s *Service) Audit(ctx context.Context, actorID, action, projectID, deploymentID, detail string) error {
	return s.audit(ctx, actorID, action, projectID, deploymentID, detail)
}

func (s *Service) uniqueSlug(ctx context.Context, name, id string) (string, error) {
	base := dockerengine.Slugify(name)
	slug := base
	for i := 0; i < 8; i++ {
		taken, err := s.Store.SlugTaken(ctx, slug, id)
		if err != nil {
			return "", err
		}
		if !taken {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%s", base, dockerengine.ShortID(id)[:2+i])
	}
	return fmt.Sprintf("%s-%s", base, dockerengine.ShortID(id)), nil
}

func (s *Service) ListAccessibleRepos(ctx context.Context) ([]store.RepoAccess, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	return s.Store.ListReposForUser(ctx, owner)
}

func (s *Service) ListRepoBranches(ctx context.Context, fullName string) ([]github.Branch, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	access, err := s.Store.GetRepoForUser(ctx, owner, fullName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, err
	}
	if s.GitHub == nil {
		if access.DefaultBranch != "" {
			return []github.Branch{{Name: access.DefaultBranch}}, nil
		}
		return []github.Branch{}, nil
	}
	tok, err := s.GitHub.CreateInstallationToken(ctx, access.InstallationGHID)
	if err != nil {
		return nil, fmt.Errorf("installation token: %w", err)
	}
	return s.GitHub.ListRepoBranches(ctx, tok.Token, access.OwnerLogin, access.Name)
}

func (s *Service) ListInstallations(ctx context.Context) ([]store.Installation, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	return s.Store.ListInstallationsForUser(ctx, owner)
}

func (s *Service) audit(ctx context.Context, actorID, action, projectID, deploymentID, detail string) error {
	id, err := newID()
	if err != nil {
		return err
	}
	return s.Store.AddAudit(ctx, store.Audit{
		ID:           id,
		ActorID:      actorID,
		Action:       action,
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		Detail:       oneLine(detail),
	})
}

func validProjectName(name string) bool {
	if len(name) < 1 || len(name) > 40 {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validUUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 400 {
		s = s[:400]
	}
	return s
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}
