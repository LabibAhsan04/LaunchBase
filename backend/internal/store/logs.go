package store

import (
	"context"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/logs"
)

// AppendLog assigns the next sequence number, inserts the row, and trims old lines.
func (db *DB) AppendLog(ctx context.Context, e logs.Event) (logs.Event, error) {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	} else {
		e.TS = e.TS.UTC()
	}
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return logs.Event{}, err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(seq), 0) + 1
		FROM deployment_logs
		WHERE deployment_id = $1`, e.DeploymentID).Scan(&e.Seq)
	if err != nil {
		return logs.Event{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO deployment_logs (deployment_id, seq, ts, source, level, message)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		e.DeploymentID, e.Seq, e.TS, e.Source, e.Level, e.Message)
	if err != nil {
		return logs.Event{}, err
	}
	if e.Seq > logs.MaxLines {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM deployment_logs
			WHERE deployment_id = $1 AND seq <= $2`,
			e.DeploymentID, e.Seq-logs.MaxLines)
		if err != nil {
			return logs.Event{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return logs.Event{}, err
	}
	return e, nil
}

func (db *DB) ListLogsAfter(ctx context.Context, deploymentID string, afterSeq int64, limit int) ([]logs.Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := db.sql.QueryContext(ctx, `
		SELECT deployment_id, seq, ts, source, level, message
		FROM deployment_logs
		WHERE deployment_id = $1 AND seq > $2
		ORDER BY seq ASC
		LIMIT $3`, deploymentID, afterSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []logs.Event{}
	for rows.Next() {
		var e logs.Event
		if err := rows.Scan(&e.DeploymentID, &e.Seq, &e.TS, &e.Source, &e.Level, &e.Message); err != nil {
			return nil, err
		}
		e.TS = e.TS.UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}

func (db *DB) LatestLogSeq(ctx context.Context, deploymentID string) (int64, error) {
	var seq int64
	err := db.sql.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(seq), 0) FROM deployment_logs WHERE deployment_id = $1`, deploymentID).Scan(&seq)
	return seq, err
}
