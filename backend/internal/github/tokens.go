package github

import "context"

// InstallationTokens adapts Client to the worker TokenSource interface.
type InstallationTokens struct {
	Client *Client
}

func (t InstallationTokens) CreateInstallationToken(ctx context.Context, installationID int64) (string, error) {
	if t.Client == nil {
		return "", nil
	}
	tok, err := t.Client.CreateInstallationToken(ctx, installationID)
	if err != nil {
		return "", err
	}
	return tok.Token, nil
}
