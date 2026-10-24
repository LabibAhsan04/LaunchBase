package github

import (
	"fmt"
	"net/url"
	"strings"
)

type Repo struct {
	Owner    string
	Name     string
	CloneURL string
}

func ParseRepoURL(raw string) (Repo, error) {
	if strings.TrimSpace(raw) == "" {
		return Repo{}, fmt.Errorf("repository url is empty")
	}
	if strings.ContainsAny(raw, " \t\r\n") {
		return Repo{}, fmt.Errorf("repository url has whitespace")
	}

	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "git@"),
		strings.HasPrefix(lower, "ssh://"),
		strings.HasPrefix(lower, "file:"),
		strings.HasPrefix(raw, "/"),
		strings.HasPrefix(raw, "."):
		return Repo{}, fmt.Errorf("only https://github.com/owner/repo urls are allowed")
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return Repo{}, fmt.Errorf("repository url is malformed")
	}
	if u.Scheme != "https" {
		return Repo{}, fmt.Errorf("only https://github.com/owner/repo urls are allowed")
	}
	if u.User != nil {
		return Repo{}, fmt.Errorf("repository url must not include credentials")
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return Repo{}, fmt.Errorf("host must be github.com")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Repo{}, fmt.Errorf("repository url must not include a query or fragment")
	}

	path := strings.Trim(u.EscapedPath(), "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, fmt.Errorf("expected https://github.com/owner/repo")
	}
	owner, err := url.PathUnescape(parts[0])
	if err != nil {
		return Repo{}, fmt.Errorf("owner name is invalid")
	}
	name, err := url.PathUnescape(parts[1])
	if err != nil {
		return Repo{}, fmt.Errorf("repo name is invalid")
	}
	if !validName(owner) || !validName(name) {
		return Repo{}, fmt.Errorf("owner or repo name is invalid")
	}

	return Repo{
		Owner:    owner,
		Name:     name,
		CloneURL: "https://github.com/" + owner + "/" + name + ".git",
	}, nil
}

func ParseBranch(branch string) (string, error) {
	if branch == "" {
		return "", nil
	}
	if strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") {
		return "", fmt.Errorf("branch %q is invalid", branch)
	}
	for _, r := range branch {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == '/':
		default:
			return "", fmt.Errorf("branch %q is invalid", branch)
		}
	}
	return branch, nil
}

func validName(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}
