package dockerengine

import (
	"strings"
	"testing"
)

func TestContainerRunArgs(t *testing.T) {
	args := containerRunArgs(RunSpec{
		Name:   "launchbase-abc",
		Image:  "launchbase/abc",
		Port:   "3000",
		Memory: "512m",
		CPUs:   "1.0",
		PIDs:   "128",
		User:   "node",
	})
	want := []string{
		"run", "-d",
		"--name", "launchbase-abc",
		"--memory", "512m",
		"--cpus", "1.0",
		"--pids-limit", "128",
		"--read-only",
		"--tmpfs", "/tmp",
		"-p", "127.0.0.1::3000",
		"--user", "node",
		"launchbase/abc",
	}
	if strings.Join(args, "\n") != strings.Join(want, "\n") {
		t.Fatalf("args:\n%s", strings.Join(args, "\n"))
	}
	for _, arg := range args {
		if arg == "--privileged" || strings.Contains(arg, "docker.sock") {
			t.Fatalf("unsafe arg %s", arg)
		}
	}
}

func TestContainerRunArgsSkipRoot(t *testing.T) {
	args := containerRunArgs(RunSpec{
		Name: "launchbase-abc", Image: "launchbase/abc", Port: "80",
		Memory: "256m", CPUs: "0.5", PIDs: "64",
	})
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--user" {
			t.Fatalf("forced user %s", args[i+1])
		}
	}
}

func TestNonRootUser(t *testing.T) {
	if NonRootUser("node") != "node" || NonRootUser("1000:1000") != "1000:1000" {
		t.Fatal("expected non-root users to be kept")
	}
	for _, user := range []string{"", "root", "0", "0:0", "0:root"} {
		if NonRootUser(user) != "" {
			t.Fatalf("kept %q", user)
		}
	}
}

func TestBuildArgs(t *testing.T) {
	args := buildArgs("/tmp/src", "launchbase/abc")
	if strings.Join(args, " ") != "build -t launchbase/abc /tmp/src" {
		t.Fatalf("%v", args)
	}
}

func TestParseExposedPorts(t *testing.T) {
	ports, err := parseExposedPorts(`{"80/tcp":{},"3000/tcp":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 2 || ports[0] != 80 || ports[1] != 3000 {
		t.Fatalf("%v", ports)
	}
	ports, err = parseExposedPorts("null")
	if err != nil || ports != nil {
		t.Fatalf("%v %v", ports, err)
	}
	if _, err := parseExposedPorts("nope"); err == nil {
		t.Fatal("accepted garbage")
	}
}

func TestParsePublished(t *testing.T) {
	got, err := parsePublished("127.0.0.1:34567\n")
	if err != nil || got != "127.0.0.1:34567" {
		t.Fatalf("%s %v", got, err)
	}
	got, err = parsePublished("0.0.0.0:34567")
	if err != nil || got != "127.0.0.1:34567" {
		t.Fatalf("%s %v", got, err)
	}
	if _, err := parsePublished("nope"); err == nil {
		t.Fatal("accepted garbage")
	}
}
