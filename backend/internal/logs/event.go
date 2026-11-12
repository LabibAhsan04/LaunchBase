package logs

import (
	"encoding/json"
	"time"
)

const (
	SourceWorker = "worker"
	SourceSystem = "system"

	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelDone  = "done"
)

// MaxLines is the hard cap kept per deployment. Older rows are deleted.
const MaxLines = 2000

// Event is one append-only log line for a deployment.
type Event struct {
	Seq          int64     `json:"seq"`
	DeploymentID string    `json:"deploymentId"`
	TS           time.Time `json:"ts"`
	Source       string    `json:"source"`
	Level        string    `json:"level"`
	Message      string    `json:"message"`
}

func (e Event) Marshal() []byte {
	raw, err := json.Marshal(e)
	if err != nil {
		return []byte(`{}`)
	}
	return raw
}

func Decode(raw []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(raw, &e); err != nil {
		return Event{}, err
	}
	if !e.TS.IsZero() {
		e.TS = e.TS.UTC()
	}
	return e, nil
}
