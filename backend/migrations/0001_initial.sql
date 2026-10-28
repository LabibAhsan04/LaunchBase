CREATE TABLE users (
    id UUID PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE projects (
    id UUID PRIMARY KEY,
    owner_id UUID NOT NULL REFERENCES users (id),
    name TEXT NOT NULL,
    repo_url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (owner_id, name)
);

CREATE TABLE deployments (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects (id),
    status TEXT NOT NULL,
    repo_url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT '',
    image_name TEXT NOT NULL DEFAULT '',
    container_name TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    error_summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    CONSTRAINT deployments_status_check CHECK (status IN (
        'QUEUED',
        'CLONING',
        'DETECTING',
        'BUILDING',
        'STARTING',
        'HEALTH_CHECKING',
        'READY',
        'FAILED',
        'STOPPED',
        'SUPERSEDED'
    ))
);

CREATE INDEX deployments_project_idx ON deployments (project_id, created_at DESC);

CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    actor_id UUID REFERENCES users (id),
    action TEXT NOT NULL,
    project_id UUID,
    deployment_id UUID,
    detail TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL
);

INSERT INTO users (id, login, display_name, created_at)
VALUES (
    '00000000-0000-4000-8000-000000000001',
    'local',
    'Local Developer',
    '2026-01-01T00:00:00Z'
);
