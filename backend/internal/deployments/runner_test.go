package deployments

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recorded struct {
	bin  string
	args []string
}

type fakeRunner struct {
	calls        []recorded
	failArg      string
	noDockerfile bool
	user         string
	portsJSON    string
	published    string
}

func (f *fakeRunner) Run(_ context.Context, bin string, args []string, _, _ io.Writer) ([]byte, error) {
	cp := append([]string(nil), args...)
	f.calls = append(f.calls, recorded{bin, cp})
	if f.failArg != "" && contains(args, f.failArg) {
		return []byte("boom"), errors.New("boom")
	}
	if bin == "git" {
		dest := args[len(args)-1]
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return nil, err
		}
		if !f.noDockerfile {
			if err := os.WriteFile(filepath.Join(dest, "Dockerfile"), []byte("FROM scratch\n"), 0o644); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if bin != "docker" || len(args) == 0 {
		return nil, nil
	}
	switch args[0] {
	case "image":
		format := flagValue(args, "--format")
		if strings.Contains(format, "Config.User") {
			user := f.user
			if user == "" {
				user = "node"
			}
			return []byte(user + "\n"), nil
		}
		raw := f.portsJSON
		if raw == "" {
			raw = `{"3000/tcp":{}}`
		}
		return []byte(raw), nil
	case "port":
		pub := f.published
		if pub == "" {
			pub = "127.0.0.1:34567"
		}
		return []byte(pub + "\n"), nil
	default:
		return nil, nil
	}
}

type staticProbe struct {
	code int
}

func (p staticProbe) Probe(context.Context, string) (int, error) {
	if p.code == 0 {
		return 200, nil
	}
	return p.code, nil
}

func TestCloneArgs(t *testing.T) {
	withBranch := strings.Join(cloneArgs("https://github.com/acme/web.git", "main", "/tmp/src"), " ")
	if withBranch != "clone --depth 1 --single-branch --branch main https://github.com/acme/web.git /tmp/src" {
		t.Fatalf("%s", withBranch)
	}
	noBranch := cloneArgs("https://github.com/acme/web.git", "", "/tmp/src")
	if contains(noBranch, "--branch") {
		t.Fatalf("%v", noBranch)
	}
}

func TestRejectsUnsafeURL(t *testing.T) {
	f := &fakeRunner{}
	_, err := Run(context.Background(), baseOpt(f, "git@github.com:acme/web.git"))
	if err == nil {
		t.Fatal("accepted ssh url")
	}
	if len(f.calls) != 0 {
		t.Fatalf("ran commands %#v", f.calls)
	}
}

func TestDeployHappyPath(t *testing.T) {
	f := &fakeRunner{}
	root := t.TempDir()
	var log bytes.Buffer
	res, err := Run(context.Background(), Options{
		RepoURL:    "https://github.com/acme/web",
		Branch:     "main",
		HealthPath: "/health",
		Stdout:     &log,
		Runner:     f,
		Probe:      staticProbe{},
		NewID:      func() (string, error) { return "11111111-1111-4111-8111-111111111111", nil },
		WorkRoot:   root,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.URL != "http://127.0.0.1:34567/health" {
		t.Fatalf("url %s", res.URL)
	}
	if res.Image != "launchbase/11111111-1111-4111-8111-111111111111" {
		t.Fatalf("image %s", res.Image)
	}
	if res.Container != "launchbase-11111111-1111-4111-8111-111111111111" {
		t.Fatalf("container %s", res.Container)
	}

	git := f.calls[0]
	if git.bin != "git" {
		t.Fatalf("first command %s", git.bin)
	}
	joined := strings.Join(git.args, " ")
	if !strings.Contains(joined, "--depth 1") || !strings.Contains(joined, "--branch main") || !strings.Contains(joined, "https://github.com/acme/web.git") {
		t.Fatalf("clone args %s", joined)
	}

	var runArgs []string
	for _, call := range f.calls {
		if call.bin == "docker" && len(call.args) > 0 && call.args[0] == "run" {
			runArgs = call.args
		}
		if call.bin == "docker" && contains(call.args, "rm") {
			t.Fatalf("removed a container on success: %v", call.args)
		}
	}
	if !contains(runArgs, "--read-only") || !contains(runArgs, "--memory") || !contains(runArgs, "512m") {
		t.Fatalf("run args %v", runArgs)
	}
	if !contains(runArgs, "--cpus") || !contains(runArgs, "--pids-limit") || !contains(runArgs, "--user") {
		t.Fatalf("run args %v", runArgs)
	}
	if !contains(runArgs, "127.0.0.1::3000") {
		t.Fatalf("run args %v", runArgs)
	}
	if contains(runArgs, "--privileged") {
		t.Fatal("privileged")
	}

	text := log.String()
	for _, status := range []string{"QUEUED", "CLONING", "DETECTING", "BUILDING", "STARTING", "HEALTH_CHECKING", "READY"} {
		if !strings.Contains(text, "status "+status) {
			t.Fatalf("log missing %s\n%s", status, text)
		}
	}
	left, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("clone dir left behind: %v", left)
	}
}

func TestMissingDockerfile(t *testing.T) {
	f := &fakeRunner{noDockerfile: true}
	root := t.TempDir()
	_, err := Run(context.Background(), Options{
		RepoURL:  "https://github.com/acme/web",
		Runner:   f,
		Probe:    staticProbe{},
		NewID:    fixedID,
		WorkRoot: root,
	})
	if err == nil || !strings.Contains(err.Error(), "Dockerfile") {
		t.Fatalf("%v", err)
	}
	for _, call := range f.calls {
		if call.bin == "docker" {
			t.Fatalf("docker ran: %v", call.args)
		}
	}
	left, _ := os.ReadDir(root)
	if len(left) != 0 {
		t.Fatal("temp dir not removed")
	}
}

func TestBuildFailureDoesNotStart(t *testing.T) {
	f := &fakeRunner{failArg: "build"}
	_, err := Run(context.Background(), Options{
		RepoURL: "https://github.com/acme/web",
		Runner:  f,
		Probe:   staticProbe{},
		NewID:   fixedID,
	})
	if err == nil {
		t.Fatal("expected build error")
	}
	for _, call := range f.calls {
		if call.bin == "docker" && len(call.args) > 0 && (call.args[0] == "run" || call.args[0] == "rm") {
			t.Fatalf("container command after failed build: %v", call.args)
		}
	}
}

func TestHealthFailureCleansUp(t *testing.T) {
	f := &fakeRunner{}
	var log bytes.Buffer
	_, err := Run(context.Background(), Options{
		RepoURL:      "https://github.com/acme/web",
		Port:         3000,
		StartTimeout: 200 * time.Millisecond,
		Stdout:       &log,
		Runner:       f,
		Probe:        staticProbe{code: 500},
		NewID:        fixedID,
	})
	if err == nil || !strings.Contains(err.Error(), "health check") {
		t.Fatalf("%v", err)
	}
	var removedContainer, removedImage bool
	for _, call := range f.calls {
		if call.bin == "docker" && contains(call.args, "rm") && contains(call.args, "-f") {
			removedContainer = true
		}
		if call.bin == "docker" && contains(call.args, "image") && contains(call.args, "rm") {
			removedImage = true
		}
	}
	if !removedContainer || !removedImage {
		t.Fatalf("cleanup calls %#v", f.calls)
	}
	if !strings.Contains(log.String(), "status FAILED") {
		t.Fatalf("log:\n%s", log.String())
	}
}

func TestBadBranchNeverClones(t *testing.T) {
	f := &fakeRunner{}
	_, err := Run(context.Background(), baseOpt(f, "https://github.com/acme/web"))
	if err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	opt := baseOpt(f, "https://github.com/acme/web")
	opt.Branch = "main;rm -rf /"
	if _, err := Run(context.Background(), opt); err == nil {
		t.Fatal("accepted bad branch")
	}
	if len(f.calls) != 0 {
		t.Fatalf("ran %#v", f.calls)
	}
}

func baseOpt(f *fakeRunner, repo string) Options {
	return Options{
		RepoURL: repo,
		Runner:  f,
		Probe:   staticProbe{},
		NewID:   fixedID,
	}
}

func fixedID() (string, error) {
	return "11111111-1111-4111-8111-111111111111", nil
}

func contains(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func flagValue(args []string, name string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}
