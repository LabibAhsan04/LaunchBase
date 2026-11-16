package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	CookieName  = "lb_session"
	SessionTTL  = 7 * 24 * time.Hour
	StateCookie = "lb_oauth_state"
	StateTTL    = 10 * time.Minute
)

type contextKey string

const userKey contextKey = "launchbase.user"

var ErrUnauthorized = errors.New("unauthorized")

type User struct {
	ID          string
	Login       string
	DisplayName string
	AvatarURL   string
	GitHubID    *int64
}

type SessionStore interface {
	CreateSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (id string, err error)
	LookupSession(ctx context.Context, tokenHash string, now time.Time) (user User, sessionID string, err error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteUserSessions(ctx context.Context, userID string) error
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func NewToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func WithUser(ctx context.Context, user User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey).(User)
	return u, ok
}

func CookieSecure() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}

func SetSessionCookie(w http.ResponseWriter, rawToken string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func SetStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   int(StateTTL.Seconds()),
		HttpOnly: true,
		Secure:   CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     StateCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   CookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func ReadCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
