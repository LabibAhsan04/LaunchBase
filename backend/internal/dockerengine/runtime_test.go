package dockerengine

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Demo App":    "demo-app",
		"Hello_World": "hello-world",
		"---Weird!!!": "weird",
		"":            "app",
		"Already-Ok":  "already-ok",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestHosts(t *testing.T) {
	id := "7a2d4c3b-2222-4222-8222-222222222222"
	if got := DeploymentHost("Demo", id, "launchbase.localhost"); got != "demo-7a2d4c3b.launchbase.localhost" {
		t.Fatalf("%s", got)
	}
	if got := ProjectHost("Demo", "launchbase.localhost"); got != "demo.launchbase.localhost" {
		t.Fatalf("%s", got)
	}
	if got := PublicURL("demo.launchbase.localhost"); got != "http://demo.launchbase.localhost/" {
		t.Fatalf("%s", got)
	}
}

func TestLocalhostDial(t *testing.T) {
	dial, host, err := LocalhostDial("http://app-99a15a16.launchbase.localhost/health")
	if err != nil {
		t.Fatal(err)
	}
	if host != "app-99a15a16.launchbase.localhost" {
		t.Fatalf("host %q", host)
	}
	if dial != "http://127.0.0.1:80/health" {
		t.Fatalf("dial %q", dial)
	}
	dial, host, err = LocalhostDial("https://example.com/x")
	if err != nil || host != "example.com" || !strings.Contains(dial, "example.com") {
		t.Fatalf("%s %s %v", dial, host, err)
	}
}

func TestTraefikLabels(t *testing.T) {
	labels := TraefikLabels("demo.launchbase.localhost", "demo-7a2d4c3b.launchbase.localhost", "3000")
	if labels["traefik.enable"] != "true" {
		t.Fatalf("%v", labels)
	}
	if labels["traefik.http.routers.lb-demo-launchbase-localhost.rule"] != "Host(`demo.launchbase.localhost`)" {
		t.Fatalf("%v", labels)
	}
	if labels["traefik.http.services.lb-demo-7a2d4c3b-launchbase-localhost.loadbalancer.server.port"] != "3000" {
		t.Fatalf("%v", labels)
	}
	for k, v := range labels {
		if k == "" || v == "" {
			t.Fatalf("empty label %q=%q", k, v)
		}
	}
}
