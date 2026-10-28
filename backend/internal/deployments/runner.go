package deployments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/framework"
	"github.com/LabibAhsan04/LaunchBase/backend/internal/github"
)

type Options struct {
	RepoURL      string
	Branch       string
	ID           string // optional. the api sets this so the row and the container share an id.
	LocalDir     string // integration test only. the cli does not set this.
	OnStatus     func(Status) error
	Port         int
	HealthPath   string
	CloneTimeout time.Duration
	BuildTimeout time.Duration
	StartTimeout time.Duration
	Memory       string
	CPUs         string
	PIDs         string
	Stdout       io.Writer
	Runner       dockerengine.Runner
	Probe        Probe
	NewID        func() (string, error)
	WorkRoot     string
}

type Result struct {
	ID        string
	Image     string
	Container string
	URL       string
}

type Probe interface {
	Probe(ctx context.Context, rawURL string) (int, error)
}

type httpProbe struct{}

func (httpProbe) Probe(ctx context.Context, rawURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode, nil
}

func Run(ctx context.Context, opt Options) (res Result, err error) {
	opt.fill()
	if err = checkHealthPath(opt.HealthPath); err != nil {
		return Result{}, err
	}

	var source string
	var cloneURL string
	if opt.LocalDir != "" && opt.RepoURL != "" {
		return Result{}, fmt.Errorf("pass a repository url or a local directory, not both")
	}
	if opt.LocalDir != "" {
		source = opt.LocalDir
	} else {
		repo, parseErr := github.ParseRepoURL(opt.RepoURL)
		if parseErr != nil {
			return Result{}, parseErr
		}
		branch, parseErr := github.ParseBranch(opt.Branch)
		if parseErr != nil {
			return Result{}, parseErr
		}
		opt.Branch = branch
		cloneURL = repo.CloneURL
	}

	id := opt.ID
	if id == "" {
		id, err = opt.NewID()
		if err != nil {
			return Result{}, err
		}
	}
	image := "launchbase/" + id
	container := "launchbase-" + id
	dep := &Deployment{ID: id, Status: StatusQueued}
	opt.logf("deployment %s", id)
	opt.logf("status %s", dep.Status)

	engine := dockerengine.Client{
		Runner: opt.Runner,
		Memory: opt.Memory,
		CPUs:   opt.CPUs,
		PIDs:   opt.PIDs,
	}

	var cloned string
	var staged string
	started := false
	built := false
	defer func() {
		if cloned != "" {
			_ = os.RemoveAll(cloned)
		}
		if staged != "" {
			_ = os.RemoveAll(staged)
		}
		if err == nil {
			return
		}
		if dep.Status != StatusFailed && dep.Status != StatusReady {
			if advErr := opt.advance(dep, StatusFailed); advErr != nil {
				opt.logf("status update: %v", advErr)
			}
		}
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if started {
			if cerr := engine.RemoveContainer(cleanCtx, container); cerr != nil {
				opt.logf("cleanup container: %v", cerr)
			}
		}
		if built {
			if cerr := engine.RemoveImage(cleanCtx, image); cerr != nil {
				opt.logf("cleanup image: %v", cerr)
			}
		}
	}()

	if err = opt.advance(dep, StatusCloning); err != nil {
		return Result{}, err
	}

	if cloneURL != "" {
		cloned, err = os.MkdirTemp(opt.WorkRoot, "launchbase-")
		if err != nil {
			return Result{}, err
		}
		opt.logf("cloning %s", cloneURL)
		cloneCtx, cancel := context.WithTimeout(ctx, opt.CloneTimeout)
		err = cloneRepo(cloneCtx, engine.Runner, cloneURL, opt.Branch, cloned, opt.Stdout)
		cancel()
		if err != nil {
			return Result{}, err
		}
		source = cloned
	} else {
		opt.logf("using %s", source)
	}

	if err = opt.advance(dep, StatusDetecting); err != nil {
		return Result{}, err
	}
	detected, err := framework.Detect(source)
	if err != nil {
		return Result{}, err
	}
	opt.logf("detected %s (%s)", detected.Framework, detected.Reason)
	if detected.Framework != framework.Dockerfile {
		// a local example directory must stay untouched. clones are already temp dirs.
		if opt.LocalDir != "" {
			staged, err = stageCopy(opt.WorkRoot, source)
			if err != nil {
				return Result{}, err
			}
			source = staged
		}
		if err = framework.WriteBuildFiles(source, detected); err != nil {
			return Result{}, err
		}
	}

	if err = opt.advance(dep, StatusBuilding); err != nil {
		return Result{}, err
	}
	opt.logf("building %s", image)
	buildCtx, cancel := context.WithTimeout(ctx, opt.BuildTimeout)
	err = engine.Build(buildCtx, source, image, opt.Stdout)
	cancel()
	if err != nil {
		return Result{}, err
	}
	built = true

	if err = opt.advance(dep, StatusStarting); err != nil {
		return Result{}, err
	}

	port := opt.Port
	if port == 0 {
		port, err = engine.ExposedPort(ctx, image)
		if err != nil {
			return Result{}, err
		}
	}
	user, err := engine.ImageUser(ctx, image)
	if err != nil {
		return Result{}, err
	}
	user = dockerengine.NonRootUser(user)
	if user == "" {
		opt.logf("image has no non-root user, starting as the image default")
	} else {
		opt.logf("image user %s", user)
	}

	opt.logf("starting %s", container)
	err = engine.Start(ctx, dockerengine.RunSpec{
		Name:   container,
		Image:  image,
		Port:   strconv.Itoa(port),
		Memory: opt.Memory,
		CPUs:   opt.CPUs,
		PIDs:   opt.PIDs,
		User:   user,
		Tmpfs:  detected.Tmpfs,
	})
	if err != nil {
		return Result{}, err
	}
	started = true

	if err = opt.advance(dep, StatusHealthChecking); err != nil {
		return Result{}, err
	}

	host, err := engine.HostPort(ctx, container, port)
	if err != nil {
		return Result{}, err
	}
	rawURL := "http://" + host + opt.HealthPath
	opt.logf("health %s", rawURL)

	healthCtx, cancel := context.WithTimeout(ctx, opt.StartTimeout)
	err = waitReady(healthCtx, opt.Probe, rawURL)
	cancel()
	if err != nil {
		return Result{}, err
	}

	if err = opt.advance(dep, StatusReady); err != nil {
		return Result{}, err
	}
	opt.logf("id=%s", id)
	opt.logf("image=%s", image)
	opt.logf("container=%s", container)
	opt.logf("url=%s", rawURL)

	return Result{ID: id, Image: image, Container: container, URL: rawURL}, nil
}

func (o *Options) fill() {
	if o.CloneTimeout == 0 {
		o.CloneTimeout = time.Minute
	}
	if o.BuildTimeout == 0 {
		o.BuildTimeout = 10 * time.Minute
	}
	if o.StartTimeout == 0 {
		o.StartTimeout = time.Minute
	}
	if o.HealthPath == "" {
		o.HealthPath = "/"
	}
	if o.Memory == "" {
		o.Memory = "512m"
	}
	if o.CPUs == "" {
		o.CPUs = "1.0"
	}
	if o.PIDs == "" {
		o.PIDs = "128"
	}
	if o.NewID == nil {
		o.NewID = newID
	}
	if o.Stdout == nil {
		o.Stdout = io.Discard
	}
	if o.Probe == nil {
		o.Probe = httpProbe{}
	}
	if o.Runner == nil {
		o.Runner = dockerengine.Local{}
	}
}

func (o Options) logf(format string, args ...any) {
	fmt.Fprintf(o.Stdout, format+"\n", args...)
}

func (o Options) advance(d *Deployment, to Status) error {
	if err := d.Advance(to); err != nil {
		return err
	}
	o.logf("status %s", d.Status)
	if o.OnStatus == nil {
		return nil
	}
	if err := o.OnStatus(d.Status); err != nil {
		return fmt.Errorf("record status %s: %w", d.Status, err)
	}
	return nil
}

func cloneArgs(cloneURL, branch, dest string) []string {
	args := []string{"clone", "--depth", "1", "--single-branch"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, cloneURL, dest)
	return args
}

func cloneRepo(ctx context.Context, run dockerengine.Runner, cloneURL, branch, dest string, log io.Writer) error {
	if run == nil {
		run = dockerengine.Local{}
	}
	_, err := run.Run(ctx, "git", cloneArgs(cloneURL, branch, dest), log, log)
	if err != nil {
		return fmt.Errorf("clone: %w", err)
	}
	return nil
}

func stageCopy(root, src string) (string, error) {
	dst, err := os.MkdirTemp(root, "launchbase-build-")
	if err != nil {
		return "", err
	}
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == ".git") {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		_ = os.RemoveAll(dst)
		return "", err
	}
	return dst, nil
}

func checkHealthPath(p string) error {
	if !strings.HasPrefix(p, "/") || strings.Contains(p, "://") || strings.ContainsAny(p, " \t") {
		return fmt.Errorf("health path must look like / or /health")
	}
	return nil
}

func waitReady(ctx context.Context, probe Probe, rawURL string) error {
	var last error
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		code, err := probe.Probe(ctx, rawURL)
		if err == nil && code >= 200 && code < 300 {
			return nil
		}
		if err != nil {
			last = err
		} else {
			last = fmt.Errorf("http %d", code)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check: %v", last)
		case <-tick.C:
		}
	}
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}
