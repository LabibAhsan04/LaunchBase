package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/auth"
)

type User struct {
	ID          string
	Login       string
	DisplayName string
	AvatarURL   string
	GitHubID    *int64
	CreatedAt   time.Time
}

func (db *DB) UpsertGitHubUser(ctx context.Context, id string, githubID int64, login, displayName, avatar string) (User, error) {
	now := time.Now().UTC()
	res, err := db.sql.ExecContext(ctx, `
		UPDATE users
		SET login = $2, display_name = $3, avatar_url = $4
		WHERE github_id = $1`, githubID, login, displayName, avatar)
	if err != nil {
		return User{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return User{}, err
	}
	if n == 0 {
		_, err = db.sql.ExecContext(ctx, `
			INSERT INTO users (id, login, display_name, github_id, avatar_url, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			id, login, displayName, githubID, avatar, now)
		if err != nil {
			if existing, err2 := db.GetUserByGitHubID(ctx, githubID); err2 == nil {
				return existing, nil
			}
			return User{}, err
		}
	}
	return db.GetUserByGitHubID(ctx, githubID)
}

func (db *DB) GetUser(ctx context.Context, id string) (User, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, login, display_name, avatar_url, github_id, created_at
		FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (db *DB) GetUserByGitHubID(ctx context.Context, githubID int64) (User, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, login, display_name, avatar_url, github_id, created_at
		FROM users WHERE github_id = $1`, githubID)
	return scanUser(row)
}

func (db *DB) GetUserByLogin(ctx context.Context, login string) (User, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT id, login, display_name, avatar_url, github_id, created_at
		FROM users WHERE login = $1`, login)
	return scanUser(row)
}

func scanUser(row scanner) (User, error) {
	var u User
	var gh sql.NullInt64
	err := row.Scan(&u.ID, &u.Login, &u.DisplayName, &u.AvatarURL, &gh, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	if gh.Valid {
		v := gh.Int64
		u.GitHubID = &v
	}
	return u, nil
}

func (u User) Auth() auth.User {
	return auth.User{
		ID: u.ID, Login: u.Login, DisplayName: u.DisplayName,
		AvatarURL: u.AvatarURL, GitHubID: u.GitHubID,
	}
}

func (db *DB) CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (string, error) {
	id, err := newUUID()
	if err != nil {
		return "", err
	}
	_, err = db.sql.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`, id, userID, tokenHash, expiresAt.UTC(), time.Now().UTC())
	return id, err
}

func (db *DB) LookupSession(ctx context.Context, tokenHash string, now time.Time) (auth.User, string, error) {
	row := db.sql.QueryRowContext(ctx, `
		SELECT s.id, u.id, u.login, u.display_name, u.avatar_url, u.github_id
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > $2`, tokenHash, now.UTC())
	var sessionID string
	var u auth.User
	var gh sql.NullInt64
	err := row.Scan(&sessionID, &u.ID, &u.Login, &u.DisplayName, &u.AvatarURL, &gh)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, "", ErrNotFound
	}
	if err != nil {
		return auth.User{}, "", err
	}
	if gh.Valid {
		v := gh.Int64
		u.GitHubID = &v
	}
	return u, sessionID, nil
}

func (db *DB) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := db.sql.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (db *DB) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := db.sql.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := readRand(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 36)
	j := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[j] = '-'
			j++
		}
		out[j] = hexdigits[v>>4]
		out[j+1] = hexdigits[v&0x0f]
		j += 2
	}
	return string(out), nil
}

func readRand(b []byte) (int, error) {
	return rand.Read(b)
}
