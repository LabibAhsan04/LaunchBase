package github

import (
	"fmt"
	"os"
	"strings"
)

func ClientFromEnv() (*Client, error) {
	appID := strings.TrimSpace(os.Getenv("GITHUB_APP_ID"))
	clientID := strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID"))
	if appID == "" || clientID == "" {
		return nil, nil
	}
	pemData, err := loadPrivateKeyPEM()
	if err != nil {
		return nil, err
	}
	if pemData == "" {
		return nil, fmt.Errorf("GITHUB_APP_PRIVATE_KEY or GITHUB_APP_PRIVATE_KEY_PATH is required when GITHUB_APP_ID is set")
	}
	return NewClient(AppConfig{
		AppID:         appID,
		ClientID:      clientID,
		ClientSecret:  strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
		PrivateKeyPEM: pemData,
		WebhookSecret: strings.TrimSpace(os.Getenv("GITHUB_WEBHOOK_SECRET")),
		AppSlug:       strings.TrimSpace(os.Getenv("GITHUB_APP_SLUG")),
	})
}

func loadPrivateKeyPEM() (string, error) {
	if v := strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY")); v != "" {
		return strings.ReplaceAll(v, `\n`, "\n"), nil
	}
	path := strings.TrimSpace(os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"))
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
