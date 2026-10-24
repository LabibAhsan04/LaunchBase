package deployments

import "testing"

func TestHappyPath(t *testing.T) {
	d := &Deployment{ID: "1", Status: StatusQueued}
	steps := []Status{
		StatusCloning,
		StatusDetecting,
		StatusBuilding,
		StatusStarting,
		StatusHealthChecking,
		StatusReady,
		StatusStopped,
	}
	for _, next := range steps {
		if err := d.Advance(next); err != nil {
			t.Fatal(err)
		}
	}
	if d.Status != StatusStopped {
		t.Fatalf("status %s", d.Status)
	}
}

func TestSupersede(t *testing.T) {
	d := &Deployment{Status: StatusReady}
	if err := d.Advance(StatusSuperseded); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidTransitions(t *testing.T) {
	cases := []struct {
		from Status
		to   Status
	}{
		{StatusQueued, StatusBuilding},
		{StatusReady, StatusFailed},
		{StatusFailed, StatusReady},
		{StatusStopped, StatusReady},
		{StatusSuperseded, StatusCloning},
		{StatusReady, StatusCloning},
	}
	for _, tc := range cases {
		if CanTransition(tc.from, tc.to) {
			t.Fatalf("allowed %s -> %s", tc.from, tc.to)
		}
	}
}

func TestFailFromActiveStates(t *testing.T) {
	active := []Status{
		StatusQueued,
		StatusCloning,
		StatusDetecting,
		StatusBuilding,
		StatusStarting,
		StatusHealthChecking,
	}
	for _, from := range active {
		if !CanTransition(from, StatusFailed) {
			t.Fatalf("did not allow %s -> FAILED", from)
		}
	}
	if CanTransition(StatusReady, StatusFailed) {
		t.Fatal("ready should not fail in place")
	}
}
