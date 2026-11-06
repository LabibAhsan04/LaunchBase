package deployments

import (
	"errors"
	"fmt"
	"testing"
)

func TestRetryableClassifiesFailures(t *testing.T) {
	if !retryable(Transient(errors.New("nope"))) {
		t.Fatal("explicit transient")
	}
	if !retryable(fmt.Errorf("clone: dial tcp: connection refused")) {
		t.Fatal("clone network")
	}
	if retryable(fmt.Errorf("docker build: dockerfile parse error")) {
		t.Fatal("build failure")
	}
	if retryable(fmt.Errorf("health check: http 500")) {
		t.Fatal("health failure")
	}
	if retryable(fmt.Errorf("cannot go from QUEUED to READY")) {
		t.Fatal("illegal transition")
	}
}
