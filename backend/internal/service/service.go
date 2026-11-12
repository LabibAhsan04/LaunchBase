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

	"github.com/LabibAhsan04/LaunchBase/backend/internal/deployments"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/queue"
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
	// Flush publishes one outbox row. The deployment is already committed if this fails.
	Flush func(context.Context) error
}

func (s *Service) CreateProject(ctx context.Context, name, repoURL, branch string) (store.Project, error) {
	name = strings.TrimSpace(name)
	if !validProjectName(name) {
		return store.Project{}, InputError{Message: "name must be 1-40 letters, numbers, spaces, dashes, or underscores"}
	}
	repo, err := github.ParseRepoURL(repoURL)
	if err != nil {
		return store.Project{}, InputError{Message: err.Error()}
	}
	branch, err = github.ParseBranch(strings.TrimSpace(branch))
	if err != nil {
		return store.Project{}, InputError{Message: err.Error()}
	}
	id, err := newID()
	if err != nil {
		return store.Project{}, err
	}
	project, err := s.Store.CreateProject(ctx, store.Project{
		ID:      id,
		OwnerID: store.LocalUserID,
		Name:    name,
		RepoURL: repo.CloneURL,
		Branch:  branch,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			return store.Project{}, ConflictError{Message: "a project with that name already exists"}
		}
		return store.Project{}, err
	}
	_ = s.audit(ctx, "project.created", project.ID, "", project.RepoURL)
	return project, nil
}

func (s *Service) ListProjects(ctx context.Context) ([]store.Project, error) {
	return s.Store.ListProjects(ctx, store.LocalUserID)
}

func (s *Service) GetProject(ctx context.Context, id string) (store.Project, []store.Deployment, error) {
	if !validUUID(id) {
		return store.Project{}, nil, store.ErrNotFound
	}
	project, err := s.Store.GetProject(ctx, store.LocalUserID, id)
	if err != nil {
		return store.Project{}, nil, err
	}
	deps, err := s.Store.ListDeployments(ctx, store.LocalUserID, id)
	if err != nil {
		return store.Project{}, nil, err
	}
	return project, deps, nil
}

func (s *Service) CreateDeployment(ctx context.Context, projectID, branch string) (store.Deployment, error) {
	if !validUUID(projectID) {
		return store.Deployment{}, store.ErrNotFound
	}
	project, err := s.Store.GetProject(ctx, store.LocalUserID, projectID)
	if err != nil {
		return store.Deployment{}, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = project.Branch
	}
	branch, err = github.ParseBranch(branch)
	if err != nil {
		return store.Deployment{}, InputError{Message: err.Error()}
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
		Version:      queue.JobVersion,
		DeploymentID: id,
		ProjectID:    project.ID,
		RepoURL:      project.RepoURL,
		Branch:       branch,
		Attempt:      1,
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
	_ = s.audit(ctx, "deployment.created", project.ID, dep.ID, dep.RepoURL)
	if s.Flush != nil {
		if err := s.Flush(ctx); err != nil && s.Log != nil {
			fmt.Fprintf(s.Log, "queue: %v\n", err)
		}
	}
	return dep, nil
}

func (s *Service) GetDeployment(ctx context.Context, id string) (store.Deployment, error) {
	if !validUUID(id) {
		return store.Deployment{}, store.ErrNotFound
	}
	return s.Store.GetDeployment(ctx, store.LocalUserID, id)
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
	_ = s.audit(ctx, "deployment.stopped", dep.ProjectID, dep.ID, dep.ContainerName)
	return s.Store.GetDeployment(ctx, store.LocalUserID, dep.ID)
}

func (s *Service) audit(ctx context.Context, action, projectID, deploymentID, detail string) error {
	id, err := newID()
	if err != nil {
		return err
	}
	return s.Store.AddAudit(ctx, store.Audit{
		ID:           id,
		ActorID:      store.LocalUserID,
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
