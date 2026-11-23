package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/auth"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
)

func (h handlers) authLogin(w http.ResponseWriter, r *http.Request) {
	if h.gh == nil {
		writeError(w, http.StatusServiceUnavailable, "github_unconfigured", "GitHub App is not configured")
		return
	}
	state, err := auth.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	auth.SetStateCookie(w, state)
	redirect := github.OAuthRedirectURI(apiPublicURL())
	http.Redirect(w, r, h.gh.AuthorizeURL(state, redirect), http.StatusFound)
}

func (h handlers) authCallback(w http.ResponseWriter, r *http.Request) {
	if h.gh == nil || h.svc == nil || h.svc.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "github_unconfigured", "GitHub App is not configured")
		return
	}
	want := auth.ReadCookie(r, auth.StateCookie)
	auth.ClearStateCookie(w)
	got := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if want == "" || got == "" || want != got || code == "" {
		writeError(w, http.StatusBadRequest, "invalid_state", "oauth state mismatch")
		return
	}
	token, err := h.gh.ExchangeCode(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth_failed", "could not exchange oauth code")
		return
	}
	info, err := h.gh.GetUser(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth_failed", "could not load github user")
		return
	}
	if !auth.AllowedLogin(info.Login) {
		writeError(w, http.StatusForbidden, "not_invited", "this GitHub account is not on the demo allowlist")
		return
	}
	display := strings.TrimSpace(info.Name)
	if display == "" {
		display = info.Login
	}
	userID, err := newUserID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	user, err := h.svc.Store.UpsertGitHubUser(r.Context(), userID, info.ID, info.Login, display, info.AvatarURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	_ = h.svc.SyncUserInstallations(r.Context(), user.ID, token)

	raw, err := auth.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	expires := time.Now().UTC().Add(auth.SessionTTL)
	if _, err := h.svc.Store.CreateSession(r.Context(), user.ID, auth.HashToken(raw), expires); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	auth.SetSessionCookie(w, raw, expires)
	_ = h.svc.Audit(r.Context(), user.ID, "auth.login", "", "", user.Login)
	http.Redirect(w, r, webOrigin()+"/", http.StatusFound)
}

func (h handlers) authLogout(w http.ResponseWriter, r *http.Request) {
	raw := auth.ReadCookie(r, auth.CookieName)
	actor := ""
	if u, ok := auth.UserFrom(r.Context()); ok {
		actor = u.ID
	}
	if raw != "" && h.svc != nil && h.svc.Store != nil {
		_ = h.svc.Store.DeleteSession(r.Context(), auth.HashToken(raw))
	}
	auth.ClearSessionCookie(w)
	if h.svc != nil {
		_ = h.svc.Audit(r.Context(), actor, "auth.logout", "", "", "")
	}
	if r.Header.Get("Accept") == "application/json" || r.URL.Query().Get("json") == "1" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	http.Redirect(w, r, webOrigin()+"/login", http.StatusFound)
}

func (h handlers) authMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "sign in required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          user.ID,
		"login":       user.Login,
		"displayName": user.DisplayName,
		"avatarUrl":   user.AvatarURL,
		"authMode":    auth.Mode(),
	})
}

func (h handlers) authInstall(w http.ResponseWriter, r *http.Request) {
	if h.gh == nil {
		writeError(w, http.StatusServiceUnavailable, "github_unconfigured", "GitHub App is not configured")
		return
	}
	http.Redirect(w, r, h.gh.InstallURL(), http.StatusFound)
}

func (h handlers) listGitHubRepos(w http.ResponseWriter, r *http.Request) {
	repos, err := h.svc.ListAccessibleRepos(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(repos))
	for _, repo := range repos {
		out = append(out, map[string]any{
			"installationId":   repo.InstallationID,
			"installationGhid": repo.InstallationGHID,
			"githubRepoId":     repo.GitHubRepoID,
			"owner":            repo.OwnerLogin,
			"name":             repo.Name,
			"fullName":         repo.FullName,
			"private":          repo.Private,
			"defaultBranch":    repo.DefaultBranch,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"repos": out})
}

func (h handlers) listGitHubBranches(w http.ResponseWriter, r *http.Request) {
	fullName := strings.Trim(r.PathValue("owner"), "/") + "/" + strings.Trim(r.PathValue("repo"), "/")
	branches, err := h.svc.ListRepoBranches(r.Context(), fullName)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	names := make([]string, 0, len(branches))
	for _, b := range branches {
		names = append(names, b.Name)
	}
	writeJSON(w, http.StatusOK, map[string]any{"branches": names})
}

func (h handlers) listInstallations(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListInstallations(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, in := range items {
		out = append(out, map[string]any{
			"id":             in.ID,
			"installationId": in.InstallationID,
			"accountLogin":   in.AccountLogin,
			"accountType":    in.AccountType,
			"suspended":      in.Suspended,
		})
	}
	installURL := ""
	if h.gh != nil {
		installURL = h.gh.InstallURL()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"installations": out,
		"installUrl":    installURL,
	})
}

func (h handlers) githubWebhook(w http.ResponseWriter, r *http.Request) {
	if h.gh == nil {
		writeError(w, http.StatusServiceUnavailable, "github_unconfigured", "GitHub App is not configured")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "could not read body")
		return
	}
	if err := github.VerifyWebhookSignature(h.gh.WebhookSecret(), body, r.Header.Get("X-Hub-Signature-256")); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_signature", "webhook signature mismatch")
		return
	}
	event := r.Header.Get("X-GitHub-Event")
	if err := h.svc.HandleInstallationWebhook(r.Context(), event, body); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "something went wrong")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func apiPublicURL() string {
	if v := strings.TrimSpace(os.Getenv("API_PUBLIC_URL")); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func webOrigin() string {
	if v := strings.TrimSpace(os.Getenv("WEB_ORIGIN")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://localhost:3000"
}

func newUserID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}
