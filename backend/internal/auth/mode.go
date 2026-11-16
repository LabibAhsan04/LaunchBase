package auth

import (
	"os"
	"strings"
)

const (
	ModeDev    = "dev"
	ModeGitHub = "github"
)

func Mode() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("AUTH_MODE"))) {
	case ModeGitHub:
		return ModeGitHub
	default:
		return ModeDev
	}
}

func RequiresGitHub() bool {
	return Mode() == ModeGitHub
}
