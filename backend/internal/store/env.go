package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ProjectEnvRow is ciphertext only. Decryption happens in the service/worker.
type ProjectEnvRow struct {
	ID          string
	ProjectID   string
	Environment string
	Key         string
	Ciphertext  []byte
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (db *DB) ListProjectEnv(ctx context.Context, projectID, environment string) ([]ProjectEnvRow, error) {
	if environment == "" {
		environment = "production"
	}
	rows, err := db.sql.QueryContext(ctx, `
		SELECT id, project_id, environment, key, ciphertext, created_at, updated_at
		FROM project_env_vars
		WHERE project_id = $1 AND environment = $2
		ORDER BY key ASC`, projectID, environment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectEnvRow
	for rows.Next() {
		var r ProjectEnvRow
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Environment, &r.Key, &r.Ciphertext, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *DB) CountProjectEnv(ctx context.Context, projectID, environment string) (int, error) {
	if environment == "" {
		environment = "production"
	}
	var n int
	err := db.sql.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM project_env_vars
		WHERE project_id = $1 AND environment = $2`, projectID, environment).Scan(&n)
	return n, err
}

func (db *DB) UpsertProjectEnv(ctx context.Context, row ProjectEnvRow) (ProjectEnvRow, error) {
	if row.Environment == "" {
		row.Environment = "production"
	}
	now := time.Now().UTC()
	err := db.sql.QueryRowContext(ctx, `
		INSERT INTO project_env_vars (id, project_id, environment, key, ciphertext, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (project_id, environment, key) DO UPDATE
		SET ciphertext = EXCLUDED.ciphertext, updated_at = EXCLUDED.updated_at
		RETURNING id, project_id, environment, key, ciphertext, created_at, updated_at`,
		row.ID, row.ProjectID, row.Environment, row.Key, row.Ciphertext, now,
	).Scan(&row.ID, &row.ProjectID, &row.Environment, &row.Key, &row.Ciphertext, &row.CreatedAt, &row.UpdatedAt)
	return row, err
}

func (db *DB) DeleteProjectEnv(ctx context.Context, projectID, environment, key string) error {
	if environment == "" {
		environment = "production"
	}
	res, err := db.sql.ExecContext(ctx, `
		DELETE FROM project_env_vars
		WHERE project_id = $1 AND environment = $2 AND key = $3`, projectID, environment, key)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (db *DB) GetProjectEnv(ctx context.Context, projectID, environment, key string) (ProjectEnvRow, error) {
	if environment == "" {
		environment = "production"
	}
	var r ProjectEnvRow
	err := db.sql.QueryRowContext(ctx, `
		SELECT id, project_id, environment, key, ciphertext, created_at, updated_at
		FROM project_env_vars
		WHERE project_id = $1 AND environment = $2 AND key = $3`,
		projectID, environment, key,
	).Scan(&r.ID, &r.ProjectID, &r.Environment, &r.Key, &r.Ciphertext, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectEnvRow{}, ErrNotFound
	}
	return r, err
}
