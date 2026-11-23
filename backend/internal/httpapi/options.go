package httpapi

import (
	"context"
	"net/http"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/service"
)

// Options configures readiness and metrics endpoints on the API mux.
type Options struct {
	Ready   func(context.Context) error
	Metrics http.Handler
}

func NewMux(svc *service.Service) http.Handler {
	return NewMuxWithOptions(svc, nil, Options{})
}

func NewMuxWithGitHub(svc *service.Service, gh *github.Client) http.Handler {
	return NewMuxWithOptions(svc, gh, Options{})
}

func NewMuxWithOptions(svc *service.Service, gh *github.Client, opt Options) http.Handler {
	return buildMux(svc, gh, opt)
}
