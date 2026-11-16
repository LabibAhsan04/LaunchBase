package github

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AppConfig struct {
	AppID         string
	ClientID      string
	ClientSecret  string
	PrivateKeyPEM string
	WebhookSecret string
	AppSlug       string
}

type Client struct {
	cfg        AppConfig
	httpClient *http.Client
	baseURL    string
	privateKey *rsa.PrivateKey
}

type InstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

type RemoteRepo struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
	HTMLURL       string `json:"html_url"`
}

type Branch struct {
	Name string `json:"name"`
}

type InstallationInfo struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
		ID    int64  `json:"id"`
	} `json:"account"`
}

type UserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

func NewClient(cfg AppConfig) (*Client, error) {
	if strings.TrimSpace(cfg.AppID) == "" {
		return nil, fmt.Errorf("github app id is required")
	}
	key, err := parseRSAPrivateKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.github.com",
		privateKey: key,
	}, nil
}

func (c *Client) AppSlug() string {
	return c.cfg.AppSlug
}

func (c *Client) WebhookSecret() string {
	return c.cfg.WebhookSecret
}

func (c *Client) CreateInstallationToken(ctx context.Context, installationID int64) (InstallationToken, error) {
	jwt, err := c.appJWT()
	if err != nil {
		return InstallationToken{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/app/installations/%d/access_tokens", c.baseURL, installationID), nil)
	if err != nil {
		return InstallationToken{}, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return InstallationToken{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return InstallationToken{}, fmt.Errorf("create installation token: status %d", res.StatusCode)
	}
	var parsed struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return InstallationToken{}, err
	}
	return InstallationToken{Token: parsed.Token, ExpiresAt: parsed.ExpiresAt.UTC()}, nil
}

func (c *Client) GetInstallation(ctx context.Context, installationID int64) (InstallationInfo, error) {
	jwt, err := c.appJWT()
	if err != nil {
		return InstallationInfo{}, err
	}
	var info InstallationInfo
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/app/installations/%d", installationID), jwt, nil, &info); err != nil {
		return InstallationInfo{}, err
	}
	return info, nil
}

func (c *Client) ListInstallationRepos(ctx context.Context, installationToken string) ([]RemoteRepo, error) {
	var out struct {
		Repositories []RemoteRepo `json:"repositories"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/installation/repositories?per_page=100", installationToken, nil, &out); err != nil {
		return nil, err
	}
	return out.Repositories, nil
}

func (c *Client) ListRepoBranches(ctx context.Context, installationToken, owner, repo string) ([]Branch, error) {
	var branches []Branch
	path := fmt.Sprintf("/repos/%s/%s/branches?per_page=100", url.PathEscape(owner), url.PathEscape(repo))
	if err := c.doJSON(ctx, http.MethodGet, path, installationToken, nil, &branches); err != nil {
		return nil, err
	}
	return branches, nil
}

func (c *Client) GetUser(ctx context.Context, userToken string) (UserInfo, error) {
	var user UserInfo
	if err := c.doJSON(ctx, http.MethodGet, "/user", userToken, nil, &user); err != nil {
		return UserInfo{}, err
	}
	return user, nil
}

func (c *Client) ListUserInstallations(ctx context.Context, userToken string) ([]InstallationInfo, error) {
	var out struct {
		Installations []InstallationInfo `json:"installations"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/user/installations", userToken, nil, &out); err != nil {
		return nil, err
	}
	return out.Installations, nil
}

func (c *Client) ExchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	form.Set("client_secret", c.cfg.ClientSecret)
	form.Set("code", code)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("oauth exchange: status %d", res.StatusCode)
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("oauth exchange failed")
	}
	return parsed.AccessToken, nil
}

func (c *Client) AuthorizeURL(state, redirectURI string) string {
	q := url.Values{}
	q.Set("client_id", c.cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("scope", "read:user user:email")
	return "https://github.com/login/oauth/authorize?" + q.Encode()
}

func (c *Client) InstallURL() string {
	slug := c.cfg.AppSlug
	if slug == "" {
		return "https://github.com/apps"
	}
	return "https://github.com/apps/" + url.PathEscape(slug) + "/installations/new"
}

func (c *Client) doJSON(ctx context.Context, method, path, token string, body io.Reader, dest any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode >= 300 {
		return fmt.Errorf("github api %s: status %d", path, res.StatusCode)
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func (c *Client) appJWT() (string, error) {
	now := time.Now().UTC()
	iat := now.Add(-30 * time.Second).Unix()
	exp := now.Add(9 * time.Minute).Unix()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(map[string]any{
		"iat": iat,
		"exp": exp,
		"iss": c.cfg.AppID,
	})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func parseRSAPrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return nil, fmt.Errorf("invalid private key pem")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not rsa")
	}
	return key, nil
}
