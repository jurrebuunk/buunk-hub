-- Kept by the hub, not Keycloak (so not overwritten at sign-in):
-- lang: the language the person chose, 'en' or 'nl'; '' follows their
--   token's locale and then the browser.
-- welcomed_at: when they finished or skipped the first-visit profile
--   dialog. People who already set a picture had their welcome.
-- start_css: their own style for /start, cleaned (internal/startcss).
ALTER TABLE users
    ADD COLUMN lang text NOT NULL DEFAULT '',
    ADD COLUMN welcomed_at timestamptz,
    ADD COLUMN start_css text NOT NULL DEFAULT '';
UPDATE users SET welcomed_at = now() WHERE picture <> '';
