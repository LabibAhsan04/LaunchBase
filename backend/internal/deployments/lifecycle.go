package deployments

import (
	"context"
	"fmt"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

// ActivateReady marks the deployment READY after URL is stored, supersedes prior
// READY deployments for the same project, and removes their containers.
//
// Sequence (low-risk cutover):
// 1. New container is already healthy on its Traefik routes (deployment + project host).
// 2. Persist URL/hostname, then mark the new row READY.
// 3. Mark each prior READY row SUPERSEDED and remove its container so Traefik drops old routes.
// A failed replacement never reaches this function, so the previous READY stays up.
func (p *Processor) ActivateReady(ctx context.Context, dep store.Deployment, result Result) error {
	if err := p.Store.SetResult(ctx, dep.ID, result.Image, result.Container, result.URL, result.Hostname); err != nil {
		return err
	}
	if err := p.Store.Transition(ctx, dep.ID, string(StatusHealthChecking), string(StatusReady)); err != nil {
		return err
	}

	prior, err := p.Store.ListReadyDeployments(ctx, dep.ProjectID)
	if err != nil {
		return err
	}
	for _, old := range prior {
		if old.ID == dep.ID {
			continue
		}
		if err := p.Store.Transition(ctx, old.ID, string(StatusReady), string(StatusSuperseded)); err != nil {
			return fmt.Errorf("supersede %s: %w", old.ID, err)
		}
		if old.ContainerName != "" && p.Remove != nil {
			if err := p.Remove.RemoveContainer(ctx, old.ContainerName); err != nil {
				p.logf("cleanup superseded container %s: %v", old.ContainerName, err)
			}
		}
		if old.ImageName != "" && p.Images != nil {
			if err := p.Images.RemoveImage(ctx, old.ImageName); err != nil {
				p.logf("cleanup superseded image %s: %v", old.ImageName, err)
			}
		}
		_ = p.audit(ctx, "deployment.superseded", old.ProjectID, old.ID, dep.ID)
	}
	return nil
}

// ImageRemover deletes images for superseded deployments.
type ImageRemover interface {
	RemoveImage(ctx context.Context, tag string) error
}
