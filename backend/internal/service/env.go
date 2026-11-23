package service

import (
	"context"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/envcrypt"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/store"
)

// EnvVarView is the only env shape returned to clients (never plaintext).
type EnvVarView struct {
	Key         string    `json:"key"`
	Environment string    `json:"environment"`
	Hint        string    `json:"hint"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type UpsertEnvInput struct {
	Key         string
	Value       string
	Environment string
}

func (s *Service) box() (*envcrypt.Box, error) {
	if s.EnvBox != nil {
		return s.EnvBox, nil
	}
	return envcrypt.FromEnv()
}

func (s *Service) ListEnvVars(ctx context.Context, projectID, environment string) ([]EnvVarView, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.Store.GetProject(ctx, owner, projectID); err != nil {
		return nil, err
	}
	if environment == "" {
		environment = "production"
	}
	box, err := s.box()
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListProjectEnv(ctx, projectID, environment)
	if err != nil {
		return nil, err
	}
	out := make([]EnvVarView, 0, len(rows))
	for _, row := range rows {
		plain, err := box.Open(row.Ciphertext)
		if err != nil {
			return nil, err
		}
		out = append(out, EnvVarView{
			Key:         row.Key,
			Environment: row.Environment,
			Hint:        envcrypt.Mask(plain),
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) UpsertEnvVar(ctx context.Context, projectID string, in UpsertEnvInput) (EnvVarView, error) {
	owner, err := ownerID(ctx)
	if err != nil {
		return EnvVarView{}, err
	}
	if _, err := s.Store.GetProject(ctx, owner, projectID); err != nil {
		return EnvVarView{}, err
	}
	key := strings.TrimSpace(in.Key)
	if err := envcrypt.ValidateKey(key); err != nil {
		return EnvVarView{}, InputError{Message: err.Error()}
	}
	if err := envcrypt.ValidateValue(in.Value); err != nil {
		return EnvVarView{}, InputError{Message: err.Error()}
	}
	env := strings.TrimSpace(in.Environment)
	if env == "" {
		env = "production"
	}
	if env != "production" {
		return EnvVarView{}, InputError{Message: "only the production environment is supported"}
	}

	box, err := s.box()
	if err != nil {
		return EnvVarView{}, err
	}

	existing, err := s.Store.GetProjectEnv(ctx, projectID, env, key)
	isNew := err == store.ErrNotFound
	if err != nil && !isNew {
		return EnvVarView{}, err
	}
	if isNew {
		n, err := s.Store.CountProjectEnv(ctx, projectID, env)
		if err != nil {
			return EnvVarView{}, err
		}
		if n >= envcrypt.MaxEnvVarsPerProject {
			return EnvVarView{}, InputError{Message: "too many environment variables for this project"}
		}
	}

	ct, err := box.Seal(in.Value)
	if err != nil {
		return EnvVarView{}, err
	}
	id := existing.ID
	if id == "" {
		id, err = newID()
		if err != nil {
			return EnvVarView{}, err
		}
	}
	row, err := s.Store.UpsertProjectEnv(ctx, store.ProjectEnvRow{
		ID: id, ProjectID: projectID, Environment: env, Key: key, Ciphertext: ct,
	})
	if err != nil {
		return EnvVarView{}, err
	}
	_ = s.audit(ctx, owner, "project.env.set", projectID, "", key)
	return EnvVarView{
		Key: row.Key, Environment: row.Environment, Hint: envcrypt.Mask(in.Value), UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *Service) DeleteEnvVar(ctx context.Context, projectID, environment, key string) error {
	owner, err := ownerID(ctx)
	if err != nil {
		return err
	}
	if _, err := s.Store.GetProject(ctx, owner, projectID); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return InputError{Message: "env key is required"}
	}
	if environment == "" {
		environment = "production"
	}
	if err := s.Store.DeleteProjectEnv(ctx, projectID, environment, key); err != nil {
		return err
	}
	_ = s.audit(ctx, owner, "project.env.delete", projectID, "", key)
	return nil
}

// DecryptEnvRows is used by the worker. Never expose plaintext over HTTP.
func DecryptEnvRows(ctx context.Context, db *store.DB, box *envcrypt.Box, projectID, environment string) (map[string]string, error) {
	if environment == "" {
		environment = "production"
	}
	rows, err := db.ListProjectEnv(ctx, projectID, environment)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		if envcrypt.ReservedKey(row.Key) {
			continue
		}
		plain, err := box.Open(row.Ciphertext)
		if err != nil {
			return nil, err
		}
		out[row.Key] = plain
	}
	return out, nil
}
