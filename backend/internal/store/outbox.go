package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// PublishPending hands one due outbox row to publish and marks it published
// in the same transaction. If publish fails, the row stays unpublished.
// A successful publish followed by a failed commit can deliver the job twice.
// Workers treat a second delivery as the same deployment.
func (db *DB) PublishPending(ctx context.Context, publish func([]byte) error) (bool, error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var id string
	var payload []byte
	err = tx.QueryRowContext(ctx, `
		SELECT id, payload
		FROM deployment_outbox
		WHERE published_at IS NULL AND available_at <= $1
		ORDER BY available_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT 1`, time.Now().UTC()).Scan(&id, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := publish(payload); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE deployment_outbox SET published_at = $2 WHERE id = $1`,
		id, time.Now().UTC()); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (db *DB) HasPendingOutbox(ctx context.Context, deploymentID string) (bool, error) {
	var n int
	err := db.sql.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM deployment_outbox
		WHERE deployment_id = $1 AND published_at IS NULL`, deploymentID).Scan(&n)
	return n > 0, err
}

// ResetToQueued is how a worker recovers a deployment it found mid-build after a crash.
// READY, FAILED, STOPPED, and SUPERSEDED are left alone.
func (db *DB) ResetToQueued(ctx context.Context, id string) error {
	res, err := db.sql.ExecContext(ctx, `
		UPDATE deployments
		SET status = 'QUEUED', error_summary = '', finished_at = NULL, updated_at = $2
		WHERE id = $1
		  AND status IN ('CLONING', 'DETECTING', 'BUILDING', 'STARTING', 'HEALTH_CHECKING')`,
		id, time.Now().UTC())
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

// ScheduleRetry puts the deployment back to QUEUED and the outbox back to unpublished.
// The worker acks the Redis message. The next publisher sends the new payload.
func (db *DB) ScheduleRetry(ctx context.Context, deploymentID string, payload []byte, attempt int, when time.Time, summary string) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE deployments
		SET status = 'QUEUED', error_summary = '', finished_at = NULL, updated_at = $2
		WHERE id = $1
		  AND status IN ('QUEUED', 'CLONING', 'DETECTING', 'BUILDING', 'STARTING', 'HEALTH_CHECKING')`,
		deploymentID, now)
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
	res, err = tx.ExecContext(ctx, `
		UPDATE deployment_outbox
		SET payload = $2::jsonb, attempts = $3, available_at = $4, last_error = $5, published_at = NULL
		WHERE deployment_id = $1`,
		deploymentID, payload, attempt, when.UTC(), summary)
	if err != nil {
		return err
	}
	n, err = res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// TryLock takes the per-deployment lock. A lock older than stale can be taken over.
func (db *DB) TryLock(ctx context.Context, deploymentID, owner string, stale time.Duration) (bool, error) {
	now := time.Now().UTC()
	res, err := db.sql.ExecContext(ctx, `
		INSERT INTO deployment_locks (deployment_id, owner, locked_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (deployment_id) DO UPDATE
		SET owner = EXCLUDED.owner, locked_at = EXCLUDED.locked_at
		WHERE deployment_locks.owner = $2
		   OR deployment_locks.locked_at < $4`,
		deploymentID, owner, now, now.Add(-stale))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (db *DB) RefreshLock(ctx context.Context, deploymentID, owner string) error {
	_, err := db.sql.ExecContext(ctx, `
		UPDATE deployment_locks SET locked_at = $3
		WHERE deployment_id = $1 AND owner = $2`,
		deploymentID, owner, time.Now().UTC())
	return err
}

func (db *DB) Unlock(ctx context.Context, deploymentID, owner string) error {
	_, err := db.sql.ExecContext(ctx, `
		DELETE FROM deployment_locks
		WHERE deployment_id = $1 AND owner = $2`, deploymentID, owner)
	return err
}
