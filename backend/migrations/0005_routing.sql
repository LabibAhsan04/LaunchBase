ALTER TABLE projects
    ADD COLUMN slug TEXT NOT NULL DEFAULT '';

UPDATE projects
SET slug = lower(regexp_replace(regexp_replace(name, '[^a-zA-Z0-9]+', '-', 'g'), '(^-+|-+$)', '', 'g'))
WHERE slug = '';

UPDATE projects
SET slug = 'project-' || substr(replace(id::text, '-', ''), 1, 8)
WHERE slug = '';

ALTER TABLE projects
    ADD CONSTRAINT projects_slug_unique UNIQUE (slug);

ALTER TABLE deployments
    ADD COLUMN hostname TEXT NOT NULL DEFAULT '';
