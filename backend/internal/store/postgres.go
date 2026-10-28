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
	ID        string
	OwnerID   string
	Name      string
	RepoURL   string
	Branch    string
	CreatedAt time.Time
	UpdatedAt time.Time
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

func (db *DB) Migrate(ctx context.Context) error {
	_, err := db.sql.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("migration table: %w", err)
	}
	var found string
	err = db.sql.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = $1`, migrations.Version).Scan(&found)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stmt := range splitSQL(migrations.Initial) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply %s: %w", migrations.Version, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`, migrations.Version, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) CreateProject(ctx context.Context, p Project) (Project, error) {
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO projects (id, owner_id, name, repo_url, branch, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		p.ID, p.OwnerID, p.Name, p.RepoURL, p.Branch, p.CreatedAt, p.UpdatedAt)
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
		SELECT id, owner_id, name, repo_url, branch, created_at, updated_at
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
		SELECT id, owner_id, name, repo_url, branch, created_at, updated_at
		FROM projects
		WHERE owner_id = $1 AND id = $2`, ownerID, id)
	p, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	return p, err
}

func (db *DB) CreateDeployment(ctx context.Context, d Deployment) (Deployment, error) {
	now := time.Now().UTC()
	d.CreatedAt = now
	d.UpdatedAt = now
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO deployments (
			id, project_id, status, repo_url, branch, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.ID, d.ProjectID, d.Status, d.RepoURL, d.Branch, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return Deployment{}, err
	}
	return d, nil
}

func (db *DB) ListDeployments(ctx context.Context, ownerID, projectID string) ([]Deployment, error) {
	rows, err := db.sql.QueryContext(ctx, `
		SELECT d.id, d.project_id, d.status, d.repo_url, d.branch,
			d.image_name, d.container_name, d.url, d.error_summary,
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
			d.image_name, d.container_name, d.url, d.error_summary,
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

func (db *DB) SetResult(ctx context.Context, id, image, container, rawURL string) error {
	_, err := db.sql.ExecContext(ctx, `
		UPDATE deployments
		SET image_name = $1, container_name = $2, url = $3, updated_at = $4
		WHERE id = $5`, image, container, rawURL, time.Now().UTC(), id)
	return err
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
	err := s.Scan(&p.ID, &p.OwnerID, &p.Name, &p.RepoURL, &p.Branch, &p.CreatedAt, &p.UpdatedAt)
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, err
}

func scanDeployment(s scanner) (Deployment, error) {
	var d Deployment
	err := s.Scan(
		&d.ID, &d.ProjectID, &d.Status, &d.RepoURL, &d.Branch,
		&d.ImageName, &d.ContainerName, &d.URL, &d.ErrorSummary,
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
