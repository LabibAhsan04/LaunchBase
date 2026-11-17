package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/migrations"
	"github.com/jackc/pgx/v5/pgconn"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const LocalUserID = "00000000-0000-4000-8000-000000000001"

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

type Project struct {
	ID             string
	OwnerID        string
	Name           string
	Slug           string
	RepoURL        string
	Branch         string
	InstallationID string
	GitHubRepoID   int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Deployment struct {
	ID            string
	ProjectID     string
	Status        string
	RepoURL       string
	Branch        string
	ImageName     string
	ContainerName string
	URL           string
	Hostname      string
	ErrorSummary  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FinishedAt    *time.Time
}

type Audit struct {
	ID           string
	ActorID      string
	Action       string
	ProjectID    string
	DeploymentID string
	Detail       string
	CreatedAt    time.Time
}

type DB struct {
	sql *sql.DB
}

func Open(ctx context.Context, dsn string) (*DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{sql: db}, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.sql.PingContext(ctx)
}

func (db *DB) Migrate(ctx context.Context) error {
	_, err := db.sql.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("migration table: %w", err)
	}
	for _, file := range migrations.All() {
		var found string
		err = db.sql.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = $1`, file.Version).Scan(&found)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := db.apply(ctx, file); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) apply(ctx context.Context, file migrations.File) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range splitSQL(file.SQL) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply %s: %w", file.Version, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`, file.Version, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) CreateProject(ctx context.Context, p Project) (Project, error) {
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	var installation any
	var githubRepo any
	if p.InstallationID != "" {
		installation = p.InstallationID
	}
	if p.GitHubRepoID != 0 {
		githubRepo = p.GitHubRepoID
	}
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO projects (id, owner_id, name, slug, repo_url, branch, installation_id, github_repo_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		p.ID, p.OwnerID, p.Name, p.Slug, p.RepoURL, p.Branch, installation, githubRepo, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		if isUnique(err) {
			return Project{}, ErrConflict
		}
		return Project{}, err
	}
	return p, nil
}

func (db *DB) ListProjects(ctx context.Context, ownerID string) ([]Project, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT id, owner_id, name, slug, repo_url, branch, COALESCE(installation_id::text, ''), COALESCE(github_repo_id, 0), created_at, updated_at
		FROM projects
		WHERE owner_id = $1
		ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (db *DB) GetProject(ctx context.Context, ownerID, id string) (Project, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, owner_id, name, slug, repo_url, branch, COALESCE(installation_id::text, ''), COALESCE(github_repo_id, 0), created_at, updated_at
		FROM projects
		WHERE owner_id = $1 AND id = $2`, ownerID, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

// CreateDeployment inserts the QUEUED row and its outbox payload in one transaction.
// The payload is what a worker will publish to Redis. A crash after this commit
// still leaves the row for a later publisher.
func (db *DB) CreateDeployment(ctx context.Context, d Deployment, outboxID string, payload []byte) (Deployment, error) {
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return Deployment{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO deployments (
			id, project_id, status, repo_url, branch, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.ID, d.ProjectID, d.Status, d.RepoURL, d.Branch, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return Deployment{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO deployment_outbox (
			id, deployment_id, payload, attempts, created_at, available_at
		) VALUES ($1, $2, $3::jsonb, 1, $4, $4)`,
		outboxID, d.ID, payload, now)
	if err != nil {
		return Deployment{}, err
	}
	if err := tx.Commit(); err != nil {
		return Deployment{}, err
	}
	return d, nil
}

func (db *DB) ListDeployments(ctx context.Context, ownerID, projectID string) ([]Deployment, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT d.id, d.project_id, d.status, d.repo_url, d.branch,
			d.image_name, d.container_name, d.url, d.hostname, d.error_summary,
			d.created_at, d.updated_at, d.finished_at
		FROM deployments d
		JOIN projects p ON p.id = d.project_id
		WHERE p.owner_id = $1 AND d.project_id = $2
		ORDER BY d.created_at DESC`, ownerID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deployment{}
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (db *DB) GetDeployment(ctx context.Context, ownerID, id string) (Deployment, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT d.id, d.project_id, d.status, d.repo_url, d.branch,
			d.image_name, d.container_name, d.url, d.hostname, d.error_summary,
			d.created_at, d.updated_at, d.finished_at
		FROM deployments d
		JOIN projects p ON p.id = d.project_id
		WHERE p.owner_id = $1 AND d.id = $2`, ownerID, id)
	d, err := scanDeployment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, ErrNotFound
	}
	return d, err
}

// GetDeploymentByID is for the worker. It does not check ownership.
func (db *DB) GetDeploymentByID(ctx context.Context, id string) (Deployment, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, project_id, status, repo_url, branch,
			image_name, container_name, url, hostname, error_summary,
			created_at, updated_at, finished_at
		FROM deployments
		WHERE id = $1`, id)
	d, err := scanDeployment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Deployment{}, ErrNotFound
	}
	return d, err
}

func (db *DB) GetProjectByID(ctx context.Context, id string) (Project, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, owner_id, name, slug, repo_url, branch, COALESCE(installation_id::text, ''), COALESCE(github_repo_id, 0), created_at, updated_at
		FROM projects
		WHERE id = $1`, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

func (db *DB) Transition(ctx context.Context, id, from, to string) error {
	now := time.Now().UTC()
	finished := to == "READY" || to == "FAILED" || to == "STOPPED" || to == "SUPERSEDED"
	res, err := db.sql.ExecContext(ctx, `
		UPDATE deployments
		SET status = $1,
			updated_at = $2,
			finished_at = CASE WHEN $3 THEN $2 ELSE finished_at END
		WHERE id = $4 AND status = $5`, to, now, finished, id, from)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}

func (db *DB) SetResult(ctx context.Context, id, image, container, rawURL, hostname string) error {
	_, err := db.sql.ExecContext(ctx, `
		UPDATE deployments
		SET image_name = $1, container_name = $2, url = $3, hostname = $4, updated_at = $5
		WHERE id = $6`, image, container, rawURL, hostname, time.Now().UTC(), id)
	return err
}

// ListReadyDeployments returns READY deployments for a project, newest first.
func (db *DB) ListReadyDeployments(ctx context.Context, projectID string) ([]Deployment, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT id, project_id, status, repo_url, branch,
			image_name, container_name, url, hostname, error_summary,
			created_at, updated_at, finished_at
		FROM deployments
		WHERE project_id = $1 AND status = 'READY'
		ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Deployment{}
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (db *DB) SlugTaken(ctx context.Context, slug string, exceptID string) (bool, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT 1 FROM projects WHERE slug = $1 AND id <> $2 LIMIT 1`, slug, exceptID)
	var n int
	err := row.Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) SetError(ctx context.Context, id, summary string) error {
	_, err := db.sql.ExecContext(ctx, `
		UPDATE deployments
		SET error_summary = $1, updated_at = $2
		WHERE id = $3`, summary, time.Now().UTC(), id)
	return err
}

func (db *DB) AddAudit(ctx context.Context, a Audit) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO audit_events (id, actor_id, action, project_id, deployment_id, detail, created_at)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6, $7)`,
		a.ID, a.ActorID, a.Action, a.ProjectID, a.DeploymentID, a.Detail, a.CreatedAt)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(s scanner) (Project, error) {
	var p Project
	err := s.Scan(&p.ID, &p.OwnerID, &p.Name, &p.Slug, &p.RepoURL, &p.Branch, &p.InstallationID, &p.GitHubRepoID, &p.CreatedAt, &p.UpdatedAt)
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, err
}

func scanDeployment(s scanner) (Deployment, error) {
	var d Deployment
	err := s.Scan(
		&d.ID, &d.ProjectID, &d.Status, &d.RepoURL, &d.Branch,
		&d.ImageName, &d.ContainerName, &d.URL, &d.Hostname, &d.ErrorSummary,
		&d.CreatedAt, &d.UpdatedAt, &d.FinishedAt,
	)
	d.CreatedAt = d.CreatedAt.UTC()
	d.UpdatedAt = d.UpdatedAt.UTC()
	if d.FinishedAt != nil {
		t := d.FinishedAt.UTC()
		d.FinishedAt = &t
	}
	return d, err
}

func isUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func QuoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func splitSQL(sql string) []string {
	parts := strings.Split(sql, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
