package dockerengine

import (
	"strings"
	"testing"
)

func TestAppendSecurityArgs(t *testing.T) {
	args := AppendSecurityArgs([]string{"run", "-d"}, "512m", DefaultSecurityOpts())
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--security-opt no-new-privileges:true",
		"--cap-drop ALL",
		"--memory-swap 512m",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
}

func TestAssertSafeRunArgs(t *testing.T) {
	safe := containerRunArgs(RunSpec{
		Name: "launchbase-x", Image: "launchbase/x", Port: "80",
		Memory: "512m", CPUs: "1", PIDs: "64", Network: "launchbase-runtime",
	})
	if err := AssertSafeRunArgs(safe); err != nil {
		t.Fatal(err)
	}
	bad := append(append([]string{}, safe...), "-v", "/var/run/docker.sock:/var/run/docker.sock")
	if err := AssertSafeRunArgs(bad); err == nil {
		t.Fatal("accepted docker.sock mount")
	}
	envBad := append(append([]string{}, safe...), "-e", "DATABASE_URL=postgres://x")
	if err := AssertSafeRunArgs(envBad); err == nil {
		t.Fatal("accepted LaunchBase env")
	}
}

func TestNetworkSeparationInArgs(t *testing.T) {
	args := containerRunArgs(RunSpec{
		Name: "c", Image: "i", Port: "80", Memory: "256m", CPUs: "0.5", PIDs: "32",
		Network: "launchbase-runtime",
	})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--network launchbase-runtime") {
		t.Fatalf("%s", joined)
	}
	if strings.Contains(joined, "--network launchbase ") || strings.Contains(joined, "--network=launchbase") {
		t.Fatalf("infra network leaked: %s", joined)
	}
}
