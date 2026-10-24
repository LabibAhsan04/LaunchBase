//go:build integration

package deployments

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
)

func TestExampleNodeDemo(t *testing.T) {
	deployExample(t, "examples/node-demo", "ok")
}

func TestExampleNodeHTTP(t *testing.T) {
	deployExample(t, "examples/node-http", "ok")
}

func TestExampleVite(t *testing.T) {
	deployExample(t, "examples/vite-demo", "vite demo")
}

func TestExampleNext(t *testing.T) {
	deployExample(t, "examples/nextjs-demo", "next demo")
}

func deployExample(t *testing.T, rel, want string) {
	t.Helper()
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatalf("docker is not available: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Clean(filepath.Join(wd, "..", "..", "..", rel))
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("example %s: %v", dir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	res, err := Run(ctx, Options{
		LocalDir:     dir,
		HealthPath:   "/",
		CloneTimeout: time.Minute,
		BuildTimeout: 6 * time.Minute,
		StartTimeout: time.Minute,
		Memory:       "512m",
		CPUs:         "1.0",
		PIDs:         "128",
		Stdout:       os.Stdout,
		Runner:       dockerengine.Local{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", res.Container).Run()
		_ = exec.Command("docker", "image", "rm", res.Image).Run()
	})

	resp, err := http.Get(res.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(body), want) {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
}
