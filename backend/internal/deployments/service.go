package deployments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/queue"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

const defaultAttempts = 3

// Remover deletes a container left by an earlier attempt. The name is stable,
// so a retry removes that container instead of starting a second one.
type Remover interface {
	RemoveContainer(ctx context.Context, name string) error
}

// Processor runs one queued deployment. The API process does not call it.
type Processor struct {
	Store       *store.DB
	Run         func(context.Context, Options) (Result, error)
	Remove      Remover
	Log         io.Writer
	Limits      Options
	Owner       string
	MaxAttempts int
	Stale       time.Duration
	Backoff     func(attempt int) time.Duration
}

func (p *Processor) Execute(ctx context.Context, job queue.Job) (queue.Result, error) {
	if err := job.Validate(); err != nil {
		return queue.Result{Dead: true, Reason: err.Error()}, nil
	}
	dep, err := p.Store.GetDeployment(ctx, store.LocalUserID, job.DeploymentID)
	if errors.Is(err, store.ErrNotFound) {
		return queue.Result{Dead: true, Reason: "deployment not found"}, nil
	}
	if err != nil {
		return queue.Result{}, err
	}
	if job.ProjectID != dep.ProjectID || job.RepoURL != dep.RepoURL || job.Branch != dep.Branch {
		return p.dead(ctx, dep, Status(dep.Status), errors.New("job does not match the deployment"))
	}
	if terminal(Status(dep.Status)) {
		return queue.Result{}, nil
	}

	ok, err := p.Store.TryLock(ctx, dep.ID, p.owner(), p.stale())
	if err != nil {
		return queue.Result{}, err
	}
	if !ok {
		return queue.Result{Busy: true}, nil
	}
	defer p.Store.Unlock(context.Background(), dep.ID, p.owner())
	stopRefresh := p.refresh(dep.ID)
	defer stopRefresh()

	if Status(dep.Status) != StatusQueued {
		if err := p.Store.ResetToQueued(ctx, dep.ID); err != nil {
			return queue.Result{}, err
		}
		dep.Status = string(StatusQueued)
	}

	if p.Remove != nil {
		if err := p.Remove.RemoveContainer(ctx, ContainerName(dep.ID)); err != nil {
			return p.fail(ctx, dep, job, StatusQueued, err)
		}
	}

	current := StatusQueued
	run := p.Run
	if run == nil {
		run = Run
	}
	result, runErr := run(ctx, Options{
		ID:           dep.ID,
		RepoURL:      dep.RepoURL,
		Branch:       dep.Branch,
		CloneTimeout: p.Limits.CloneTimeout,
		BuildTimeout: p.Limits.BuildTimeout,
		StartTimeout: p.Limits.StartTimeout,
		Memory:       p.Limits.Memory,
		CPUs:         p.Limits.CPUs,
		PIDs:         p.Limits.PIDs,
		Stdout:       p.Log,
		OnStatus: func(next Status) error {
			if next == StatusFailed {
				return nil
			}
			if !CanTransition(current, next) {
				return fmt.Errorf("cannot go from %s to %s", current, next)
			}
			if err := p.Store.Transition(ctx, dep.ID, string(current), string(next)); err != nil {
				return err
			}
			current = next
			return nil
		},
	})
	if runErr != nil {
		return p.fail(ctx, dep, job, current, runErr)
	}
	if err := p.Store.SetResult(ctx, dep.ID, result.Image, result.Container, result.URL); err != nil {
		return queue.Result{}, err
	}
	_ = p.audit(ctx, "deployment.ready", dep.ProjectID, dep.ID, result.URL)
	p.logf("deployment %s ready %s", dep.ID, result.URL)
	return queue.Result{}, nil
}

func (p *Processor) fail(ctx context.Context, dep store.Deployment, job queue.Job, current Status, runErr error) (queue.Result, error) {
	summary := oneLine(runErr.Error())
	if retryable(runErr) && job.Attempt < p.maxAttempts() {
		job.Attempt++
		raw, err := json.Marshal(job)
		if err != nil {
			return queue.Result{}, err
		}
		when := p.now().Add(p.backoff(job.Attempt))
		if err := p.Store.ScheduleRetry(ctx, dep.ID, raw, job.Attempt, when, summary); err != nil {
			return queue.Result{}, err
		}
		p.logf("deployment %s retry %d: %s", dep.ID, job.Attempt, summary)
		return queue.Result{}, nil
	}
	return p.dead(ctx, dep, current, errors.New(summary))
}

func (p *Processor) dead(ctx context.Context, dep store.Deployment, current Status, runErr error) (queue.Result, error) {
	summary := oneLine(runErr.Error())
	if current != StatusFailed && CanTransition(current, StatusFailed) {
		_ = p.Store.Transition(ctx, dep.ID, string(current), string(StatusFailed))
	}
	_ = p.Store.SetError(ctx, dep.ID, summary)
	_ = p.audit(ctx, "deployment.failed", dep.ProjectID, dep.ID, summary)
	p.logf("deployment %s failed: %s", dep.ID, summary)
	return queue.Result{Dead: true, Reason: summary}, nil
}

func (p *Processor) refresh(id string) func() {
	ctx, cancel := context.WithCancel(context.Background())
	every := p.stale() / 3
	if every < time.Second {
		every = time.Second
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				_ = p.Store.RefreshLock(context.Background(), id, p.owner())
			}
		}
	}()
	return cancel
}

func (p *Processor) owner() string {
	if p.Owner == "" {
		p.Owner = "worker"
	}
	return p.Owner
}

func (p *Processor) maxAttempts() int {
	if p.MaxAttempts <= 0 {
		return defaultAttempts
	}
	return p.MaxAttempts
}

func (p *Processor) stale() time.Duration {
	if p.Stale <= 0 {
		return 30 * time.Second
	}
	return p.Stale
}

func (p *Processor) now() time.Time {
	return time.Now().UTC()
}

func (p *Processor) backoff(attempt int) time.Duration {
	if p.Backoff != nil {
		return p.Backoff(attempt)
	}
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 4 {
		shift = 4
	}
	return time.Second << shift
}

func (p *Processor) audit(ctx context.Context, action, projectID, deploymentID, detail string) error {
	id, err := newID()
	if err != nil {
		return err
	}
	return p.Store.AddAudit(ctx, store.Audit{
		ID:           id,
		ActorID:      store.LocalUserID,
		Action:       action,
		ProjectID:    projectID,
		DeploymentID: deploymentID,
		Detail:       oneLine(detail),
	})
}

func (p *Processor) logf(format string, args ...any) {
	if p.Log == nil {
		return
	}
	fmt.Fprintf(p.Log, format+"\n", args...)
}

func terminal(status Status) bool {
	switch status {
	case StatusReady, StatusFailed, StatusStopped, StatusSuperseded:
		return true
	default:
		return false
	}
}

func retryable(err error) bool {
	if err == nil {
		return false
	}
	if IsTransient(err) {
		return true
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "docker build:") || strings.Contains(s, "health check:") || strings.Contains(s, "cannot go from") {
		return false
	}
	for _, part := range []string{
		"cannot connect to the docker daemon",
		"connection refused",
		"connection reset",
		"could not resolve host",
		"i/o timeout",
		"temporary failure in name resolution",
	} {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 400 {
		s = s[:400]
	}
	return s
}
