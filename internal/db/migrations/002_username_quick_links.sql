-- The Keycloak username, shown in Settings (a cache, like name and email).
ALTER TABLE users ADD COLUMN username text NOT NULL DEFAULT '';

-- A person's quick links on their start page, in their order. Only theirs:
-- every query is by sub.
CREATE TABLE quick_links (
    id         bigserial PRIMARY KEY,
    sub        text NOT NULL REFERENCES users (sub) ON DELETE CASCADE,
    position   integer NOT NULL,
    title      text NOT NULL,
    url        text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quick_links_sub ON quick_links (sub, position);
