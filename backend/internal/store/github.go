package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Installation struct {
	ID             string
	InstallationID int64
	AccountLogin   string
	AccountType    string
	Suspended      bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RepoAccess struct {
	InstallationID   string
	InstallationGHID int64
	GitHubRepoID     int64
	OwnerLogin       string
	Name             string
	FullName         string
	Private          bool
	DefaultBranch    string
}

func (db *DB) UpsertInstallation(ctx context.Context, id string, installationID int64, login, accountType string, suspended bool) (Installation, error) {
	now := time.Now().UTC()
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO github_installations (id, installation_id, account_login, account_type, suspended, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (installation_id) DO UPDATE
		SET account_login = EXCLUDED.account_login,
		    account_type = EXCLUDED.account_type,
		    suspended = EXCLUDED.suspended,
		    updated_at = EXCLUDED.updated_at`,
		id, installationID, login, accountType, suspended, now)
	if err != nil {
		return Installation{}, err
	}
	return db.GetInstallationByGitHubID(ctx, installationID)
}

func (db *DB) GetInstallationByGitHubID(ctx context.Context, installationID int64) (Installation, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, installation_id, account_login, account_type, suspended, created_at, updated_at
		FROM github_installations WHERE installation_id = $1`, installationID)
	return scanInstallation(row)
}

func (db *DB) GetInstallation(ctx context.Context, id string) (Installation, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, installation_id, account_login, account_type, suspended, created_at, updated_at
		FROM github_installations WHERE id = $1`, id)
	return scanInstallation(row)
}

func scanInstallation(row scanner) (Installation, error) {
	var in Installation
	err := row.Scan(&in.ID, &in.InstallationID, &in.AccountLogin, &in.AccountType, &in.Suspended, &in.CreatedAt, &in.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Installation{}, ErrNotFound
	}
	if err != nil {
		return Installation{}, err
	}
	in.CreatedAt = in.CreatedAt.UTC()
	in.UpdatedAt = in.UpdatedAt.UTC()
	return in, nil
}

func (db *DB) LinkUserInstallation(ctx context.Context, userID, installationID string) error {
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO github_user_installations (user_id, installation_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, installationID)
	return err
}

func (db *DB) DeleteInstallation(ctx context.Context, installationGHID int64) error {
	_, err := db.sql.ExecContext(ctx, `DELETE FROM github_installations WHERE installation_id = $1`, installationGHID)
	return err
}

func (db *DB) ReplaceInstallationRepos(ctx context.Context, installationID string, repos []RepoAccess) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM github_repos WHERE installation_id = $1`, installationID); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, r := range repos {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO github_repos (
				installation_id, github_repo_id, owner_login, name, full_name, private, default_branch, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			installationID, r.GitHubRepoID, r.OwnerLogin, r.Name, r.FullName, r.Private, r.DefaultBranch, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (db *DB) UpsertRepos(ctx context.Context, installationID string, repos []RepoAccess) error {
	now := time.Now().UTC()
	for _, r := range repos {
		_, err := db.sql.ExecContext(ctx, `
			INSERT INTO github_repos (
				installation_id, github_repo_id, owner_login, name, full_name, private, default_branch, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (installation_id, github_repo_id) DO UPDATE
			SET owner_login = EXCLUDED.owner_login,
			    name = EXCLUDED.name,
			    full_name = EXCLUDED.full_name,
			    private = EXCLUDED.private,
			    default_branch = EXCLUDED.default_branch,
			    updated_at = EXCLUDED.updated_at`,
			installationID, r.GitHubRepoID, r.OwnerLogin, r.Name, r.FullName, r.Private, r.DefaultBranch, now)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) RemoveRepos(ctx context.Context, installationID string, githubRepoIDs []int64) error {
	for _, id := range githubRepoIDs {
		if _, err := db.sql.ExecContext(ctx, `
			DELETE FROM github_repos WHERE installation_id = $1 AND github_repo_id = $2`,
			installationID, id); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) ListReposForUser(ctx context.Context, userID string) ([]RepoAccess, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT r.installation_id, i.installation_id, r.github_repo_id, r.owner_login, r.name, r.full_name, r.private, r.default_branch
		FROM github_repos r
		JOIN github_installations i ON i.id = r.installation_id
		JOIN github_user_installations ui ON ui.installation_id = r.installation_id
		WHERE ui.user_id = $1 AND i.suspended = FALSE
		ORDER BY r.full_name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RepoAccess{}
	for rows.Next() {
		var r RepoAccess
		if err := rows.Scan(&r.InstallationID, &r.InstallationGHID, &r.GitHubRepoID, &r.OwnerLogin, &r.Name, &r.FullName, &r.Private, &r.DefaultBranch); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *DB) GetRepoForUser(ctx context.Context, userID, fullName string) (RepoAccess, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT r.installation_id, i.installation_id, r.github_repo_id, r.owner_login, r.name, r.full_name, r.private, r.default_branch
		FROM github_repos r
		JOIN github_installations i ON i.id = r.installation_id
		JOIN github_user_installations ui ON ui.installation_id = r.installation_id
		WHERE ui.user_id = $1 AND r.full_name = $2 AND i.suspended = FALSE`, userID, fullName)
	var r RepoAccess
	err := row.Scan(&r.InstallationID, &r.InstallationGHID, &r.GitHubRepoID, &r.OwnerLogin, &r.Name, &r.FullName, &r.Private, &r.DefaultBranch)
	if errors.Is(err, sql.ErrNoRows) {
		return RepoAccess{}, ErrNotFound
	}
	return r, err
}

func (db *DB) ListInstallationsForUser(ctx context.Context, userID string) ([]Installation, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT i.id, i.installation_id, i.account_login, i.account_type, i.suspended, i.created_at, i.updated_at
		FROM github_installations i
		JOIN github_user_installations ui ON ui.installation_id = i.id
		WHERE ui.user_id = $1
		ORDER BY i.account_login ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Installation{}
	for rows.Next() {
		in, err := scanInstallation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}
