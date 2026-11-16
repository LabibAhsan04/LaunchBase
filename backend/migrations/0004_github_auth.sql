ALTER TABLE users
    ADD COLUMN github_id BIGINT UNIQUE,
    ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';

CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);

CREATE TABLE github_installations (
    id UUID PRIMARY KEY,
    installation_id BIGINT NOT NULL UNIQUE,
    account_login TEXT NOT NULL,
    account_type TEXT NOT NULL DEFAULT 'User',
    suspended BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE github_user_installations (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    installation_id UUID NOT NULL REFERENCES github_installations (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, installation_id)
);

CREATE TABLE github_repos (
    installation_id UUID NOT NULL REFERENCES github_installations (id) ON DELETE CASCADE,
    github_repo_id BIGINT NOT NULL,
    owner_login TEXT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    private BOOLEAN NOT NULL DEFAULT FALSE,
    default_branch TEXT NOT NULL DEFAULT 'main',
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (installation_id, github_repo_id)
);

CREATE INDEX github_repos_full_name_idx ON github_repos (full_name);

ALTER TABLE projects
    ADD COLUMN installation_id UUID REFERENCES github_installations (id),
    ADD COLUMN github_repo_id BIGINT;
