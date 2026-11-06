package queue

import (
	"encoding/json"
	"fmt"
)

const JobVersion = 1

// Job is the only thing Redis stores about a deployment.
// The worker loads the row from Postgres and ignores the job if it does not match.
type Job struct {
	Version      int    `json:"version"`
	DeploymentID string `json:"deploymentId"`
	ProjectID    string `json:"projectId"`
	RepoURL      string `json:"repoUrl"`
	Branch       string `json:"branch"`
	Attempt      int    `json:"attempt"`
}

func (j Job) Validate() error {
	if j.Version != JobVersion {
		return fmt.Errorf("unsupported job version %d", j.Version)
	}
	if j.DeploymentID == "" || j.ProjectID == "" || j.RepoURL == "" {
		return fmt.Errorf("job is missing an id or repository")
	}
	if j.Attempt < 1 {
		return fmt.Errorf("job attempt must start at 1")
	}
	return nil
}

func Decode(payload []byte) (Job, error) {
	var job Job
	if err := json.Unmarshal(payload, &job); err != nil {
		return Job{}, fmt.Errorf("job payload: %w", err)
	}
	if err := job.Validate(); err != nil {
		return Job{}, err
	}
	return job, nil
}

// Result tells the consumer what to do with the Redis message.
type Result struct {
	Dead   bool
	Reason string
	Busy   bool
}
