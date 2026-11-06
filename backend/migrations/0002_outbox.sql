CREATE TABLE deployment_outbox (
    id UUID PRIMARY KEY,
    deployment_id UUID NOT NULL UNIQUE REFERENCES deployments (id),
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 1,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    available_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE INDEX deployment_outbox_pending_idx
    ON deployment_outbox (available_at)
    WHERE published_at IS NULL;

CREATE TABLE deployment_locks (
    deployment_id UUID PRIMARY KEY REFERENCES deployments (id),
    owner TEXT NOT NULL,
    locked_at TIMESTAMPTZ NOT NULL
);
