CREATE TABLE deployment_logs (
    deployment_id UUID NOT NULL REFERENCES deployments (id) ON DELETE CASCADE,
    seq BIGINT NOT NULL,
    ts TIMESTAMPTZ NOT NULL,
    source TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    PRIMARY KEY (deployment_id, seq)
);

CREATE INDEX deployment_logs_tail_idx ON deployment_logs (deployment_id, seq DESC);
