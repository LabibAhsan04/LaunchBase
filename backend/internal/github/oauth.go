package github

import "strings"

// OAuthRedirectURI is the API callback GitHub redirects to after sign-in.
func OAuthRedirectURI(apiPublicURL string) string {
	base := strings.TrimRight(strings.TrimSpace(apiPublicURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/api/v1/auth/callback"
}
