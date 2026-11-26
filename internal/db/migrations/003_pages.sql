-- A person's own pages ("My pages"): each embeds one site, in their order.
-- Only theirs: every query is by sub. refused is the header that stops the
-- site from being framed, seen when the page was saved; empty means it is
-- embedded, otherwise the page is a link card that opens it.
CREATE TABLE pages (
    id         bigserial PRIMARY KEY,
    sub        text NOT NULL REFERENCES users (sub) ON DELETE CASCADE,
    position   integer NOT NULL,
    title      text NOT NULL,
    url        text NOT NULL,
    refused    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pages_sub ON pages (sub, position);
