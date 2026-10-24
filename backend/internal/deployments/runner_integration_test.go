//go:build integration

package deployments

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
)

func TestExampleNodeDemo(t *testing.T) {
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Fatalf("docker is not available: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Clean(filepath.Join(wd, "..", "..", "..", "examples", "node-demo"))
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	res, err := Run(ctx, Options{
		LocalDir:     dir,
		HealthPath:   "/",
		CloneTimeout: time.Minute,
		BuildTimeout: 3 * time.Minute,
		StartTimeout: 45 * time.Second,
		Memory:       "256m",
		CPUs:         "1.0",
		PIDs:         "64",
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
	if resp.StatusCode != 200 || string(body) != "ok\n" {
		t.Fatalf("status %d body %q", resp.StatusCode, body)
	}
}
