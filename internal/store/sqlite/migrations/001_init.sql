CREATE TABLE events (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    time       TEXT NOT NULL,            -- normalised UTC, fixed width: sorts as text
    status     TEXT NOT NULL,
    system     TEXT NOT NULL,
    component  TEXT NOT NULL DEFAULT '',
    actor_kind TEXT NOT NULL,
    actor_id   TEXT NOT NULL,
    summary    TEXT NOT NULL DEFAULT '',
    body       TEXT NOT NULL             -- the full event JSON; the source of truth for reads
);
CREATE INDEX events_time        ON events (time);
CREATE INDEX events_system_time ON events (system, time);
CREATE INDEX events_type        ON events (type);

CREATE TABLE links (
    source     TEXT NOT NULL,
    target     TEXT NOT NULL,
    rel        TEXT NOT NULL,
    basis      TEXT NOT NULL,
    rationale  TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY (source, target, rel, basis)
);
CREATE INDEX links_target ON links (target);

CREATE VIRTUAL TABLE events_fts USING fts5 (id UNINDEXED, summary, system, component, type);
