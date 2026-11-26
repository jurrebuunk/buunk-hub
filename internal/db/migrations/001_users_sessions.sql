-- People who signed in at least once, keyed by Keycloak's sub. Name, email,
-- locale and picture are a cache of their latest ID token; Keycloak owns them.
CREATE TABLE users (
    sub        text PRIMARY KEY,
    name       text NOT NULL DEFAULT '',
    email      text NOT NULL DEFAULT '',
    locale     text NOT NULL DEFAULT '',
    picture    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- A signed-in browser. id is the SHA-256 of the cookie value, so the table
-- alone doesn't let anyone sign in. The tokens are sealed with the session
-- key (AES-GCM).
CREATE TABLE sessions (
    id               bytea PRIMARY KEY,
    sub              text NOT NULL REFERENCES users (sub) ON DELETE CASCADE,
    roles            jsonb NOT NULL DEFAULT '{}',
    refresh_token    bytea NOT NULL,
    id_token         bytea NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    expires_at       timestamptz NOT NULL,
    roles_checked_at timestamptz NOT NULL
);
CREATE INDEX sessions_sub ON sessions (sub);
CREATE INDEX sessions_expires_at ON sessions (expires_at);
