package deployments

import (
	"strings"
	"testing"

	"github.com/LabibAhsan04/LaunchBase/backend/internal/dockerengine"
)

func TestTraefikRoutingLabelsDeterministic(t *testing.T) {
	labels := dockerengine.TraefikLabels(
		dockerengine.ProjectHost("Demo App", "launchbase.localhost"),
		dockerengine.DeploymentHost("Demo App", "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", "launchbase.localhost"),
		"8080",
	)
	if labels["traefik.enable"] != "true" {
		t.Fatal(labels)
	}
	rule := labels["traefik.http.routers.lb-demo-app-launchbase-localhost.rule"]
	if rule != "Host(`demo-app.launchbase.localhost`)" {
		t.Fatalf("%s", rule)
	}
	joined := ""
	for k, v := range labels {
		joined += k + "=" + v + "\n"
	}
	if strings.Contains(joined, "docker.sock") {
		t.Fatal(joined)
	}
}

func TestCanSupersedeFromReady(t *testing.T) {
	if !CanTransition(StatusReady, StatusSuperseded) {
		t.Fatal("READY should allow SUPERSEDED")
	}
	if CanTransition(StatusFailed, StatusSuperseded) {
		t.Fatal("FAILED should not supersede")
	}
}
