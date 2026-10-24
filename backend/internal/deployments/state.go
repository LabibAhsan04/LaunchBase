package deployments

import "fmt"

type Status string

const (
	StatusQueued         Status = "QUEUED"
	StatusCloning        Status = "CLONING"
	StatusDetecting      Status = "DETECTING"
	StatusBuilding       Status = "BUILDING"
	StatusStarting       Status = "STARTING"
	StatusHealthChecking Status = "HEALTH_CHECKING"
	StatusReady          Status = "READY"
	StatusFailed         Status = "FAILED"
	StatusStopped        Status = "STOPPED"
	StatusSuperseded     Status = "SUPERSEDED"
)

func CanTransition(from, to Status) bool {
	if to == StatusFailed {
		switch from {
		case StatusQueued, StatusCloning, StatusDetecting, StatusBuilding, StatusStarting, StatusHealthChecking:
			return true
		default:
			return false
		}
	}

	switch from {
	case StatusQueued:
		return to == StatusCloning
	case StatusCloning:
		return to == StatusDetecting
	case StatusDetecting:
		return to == StatusBuilding
	case StatusBuilding:
		return to == StatusStarting
	case StatusStarting:
		return to == StatusHealthChecking
	case StatusHealthChecking:
		return to == StatusReady
	case StatusReady:
		return to == StatusStopped || to == StatusSuperseded
	default:
		return false
	}
}

type Deployment struct {
	ID     string
	Status Status
}

func (d *Deployment) Advance(to Status) error {
	if !CanTransition(d.Status, to) {
		return fmt.Errorf("cannot go from %s to %s", d.Status, to)
	}
	d.Status = to
	return nil
}
