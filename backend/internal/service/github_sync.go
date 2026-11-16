package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

func (s *Service) SyncUserInstallations(ctx context.Context, userID, userToken string) error {
	if s.GitHub == nil {
		return fmt.Errorf("github client is not configured")
	}
	installations, err := s.GitHub.ListUserInstallations(ctx, userToken)
	if err != nil {
		return err
	}
	for _, info := range installations {
		id, err := newID()
		if err != nil {
			return err
		}
		inst, err := s.Store.UpsertInstallation(ctx, id, info.ID, info.Account.Login, info.Account.Type, false)
		if err != nil {
			return err
		}
		if err := s.Store.LinkUserInstallation(ctx, userID, inst.ID); err != nil {
			return err
		}
		tok, err := s.GitHub.CreateInstallationToken(ctx, info.ID)
		if err != nil {
			return err
		}
		repos, err := s.GitHub.ListInstallationRepos(ctx, tok.Token)
		if err != nil {
			return err
		}
		access := make([]store.RepoAccess, 0, len(repos))
		for _, r := range repos {
			owner, name := github.OwnerFromFullName(r.FullName)
			if owner == "" {
				owner = r.Name
			}
			if name == "" {
				name = r.Name
			}
			branch := r.DefaultBranch
			if branch == "" {
				branch = "main"
			}
			access = append(access, store.RepoAccess{
				InstallationID: inst.ID,
				GitHubRepoID:   r.ID,
				OwnerLogin:     owner,
				Name:           name,
				FullName:       r.FullName,
				Private:        r.Private,
				DefaultBranch:  branch,
			})
		}
		if err := s.Store.ReplaceInstallationRepos(ctx, inst.ID, access); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) HandleInstallationWebhook(ctx context.Context, event string, body []byte) error {
	switch event {
	case "installation":
		ev, err := github.ParseInstallationEvent(body)
		if err != nil {
			return err
		}
		switch strings.ToLower(ev.Action) {
		case "deleted":
			return s.Store.DeleteInstallation(ctx, ev.Installation.ID)
		case "suspend":
			id, err := newID()
			if err != nil {
				return err
			}
			_, err = s.Store.UpsertInstallation(ctx, id, ev.Installation.ID, ev.Installation.Account.Login, ev.Installation.Account.Type, true)
			return err
		case "unsuspend", "created", "new_permissions_accepted":
			id, err := newID()
			if err != nil {
				return err
			}
			inst, err := s.Store.UpsertInstallation(ctx, id, ev.Installation.ID, ev.Installation.Account.Login, ev.Installation.Account.Type, false)
			if err != nil {
				return err
			}
			if len(ev.Repositories) > 0 {
				return s.Store.UpsertRepos(ctx, inst.ID, webhookRepos(inst.ID, ev.Repositories))
			}
			return nil
		default:
			return nil
		}
	case "installation_repositories":
		ev, err := github.ParseInstallationRepositoriesEvent(body)
		if err != nil {
			return err
		}
		id, err := newID()
		if err != nil {
			return err
		}
		inst, err := s.Store.UpsertInstallation(ctx, id, ev.Installation.ID, ev.Installation.Account.Login, ev.Installation.Account.Type, false)
		if err != nil {
			return err
		}
		if len(ev.RepositoriesAdded) > 0 {
			if err := s.Store.UpsertRepos(ctx, inst.ID, webhookRepos(inst.ID, ev.RepositoriesAdded)); err != nil {
				return err
			}
		}
		if len(ev.RepositoriesRemoved) > 0 {
			ids := make([]int64, 0, len(ev.RepositoriesRemoved))
			for _, r := range ev.RepositoriesRemoved {
				ids = append(ids, r.ID)
			}
			return s.Store.RemoveRepos(ctx, inst.ID, ids)
		}
		return nil
	default:
		return nil
	}
}

func webhookRepos(installationUUID string, repos []github.WebhookRepository) []store.RepoAccess {
	out := make([]store.RepoAccess, 0, len(repos))
	for _, r := range repos {
		owner, name := github.OwnerFromFullName(r.FullName)
		if owner == "" {
			owner = r.Name
		}
		if name == "" {
			name = r.Name
		}
		branch := r.DefaultBranch
		if branch == "" {
			branch = "main"
		}
		out = append(out, store.RepoAccess{
			InstallationID: installationUUID,
			GitHubRepoID:   r.ID,
			OwnerLogin:     owner,
			Name:           name,
			FullName:       r.FullName,
			Private:        r.Private,
			DefaultBranch:  branch,
		})
	}
	return out
}
