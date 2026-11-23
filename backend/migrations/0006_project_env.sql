CREATE TABLE project_env_vars (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    environment TEXT NOT NULL DEFAULT 'production',
    key TEXT NOT NULL,
    ciphertext BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (project_id, environment, key)
);

CREATE INDEX project_env_vars_project_idx ON project_env_vars (project_id, environment);
