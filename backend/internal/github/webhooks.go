package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func VerifyWebhookSignature(secret string, body []byte, header string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return fmt.Errorf("webhook secret is not configured")
	}
	header = strings.TrimSpace(header)
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return fmt.Errorf("missing webhook signature")
	}
	want, err := hex.DecodeString(strings.TrimPrefix(header, prefix))
	if err != nil {
		return fmt.Errorf("invalid webhook signature")
	}
	sum := hmac.New(sha256.New, []byte(secret))
	_, _ = sum.Write(body)
	got := sum.Sum(nil)
	if !hmac.Equal(got, want) {
		return fmt.Errorf("webhook signature mismatch")
	}
	return nil
}

type WebhookInstallation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
		Type  string `json:"type"`
		ID    int64  `json:"id"`
	} `json:"account"`
	SuspendedAt *string `json:"suspended_at"`
}

type WebhookRepository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Private       bool   `json:"private"`
	DefaultBranch string `json:"default_branch"`
}

type InstallationEvent struct {
	Action       string              `json:"action"`
	Installation WebhookInstallation `json:"installation"`
	Repositories []WebhookRepository `json:"repositories"`
}

type InstallationRepositoriesEvent struct {
	Action              string              `json:"action"`
	Installation        WebhookInstallation `json:"installation"`
	RepositoriesAdded   []WebhookRepository `json:"repositories_added"`
	RepositoriesRemoved []WebhookRepository `json:"repositories_removed"`
}

func ParseInstallationEvent(body []byte) (InstallationEvent, error) {
	var ev InstallationEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return InstallationEvent{}, err
	}
	return ev, nil
}

func ParseInstallationRepositoriesEvent(body []byte) (InstallationRepositoriesEvent, error) {
	var ev InstallationRepositoriesEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return InstallationRepositoriesEvent{}, err
	}
	return ev, nil
}

func OwnerFromFullName(fullName string) (owner, name string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", fullName
	}
	return parts[0], parts[1]
}
