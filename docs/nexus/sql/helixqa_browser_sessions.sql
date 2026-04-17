-- helixqa_browser_sessions — one row per browser session produced by
-- pkg/nexus/browser/Engine. Consumed by the Grafana dashboard and the
-- evidence vault indexer to join recorded artefacts back to the owning
-- session.
--
-- Compatible with both SQLite (development) and PostgreSQL (production).
-- The schema is intentionally minimal so later phases can extend it
-- without a breaking migration: new columns ship as nullable adds.

CREATE TABLE IF NOT EXISTS helixqa_browser_sessions (
    session_id     TEXT    PRIMARY KEY,
    engine         TEXT    NOT NULL,           -- chromedp | rod | playwright
    platform       TEXT    NOT NULL,           -- web-chromedp | web-rod | ...
    pool_slot      INTEGER,                    -- NULL when not acquired via a Pool
    started_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at       TIMESTAMP,                  -- NULL until Close() completes
    user_data_dir  TEXT,                       -- record of the profile path
    headless       INTEGER NOT NULL DEFAULT 1, -- SQLite bool
    window_w       INTEGER,
    window_h       INTEGER,
    allowed_hosts  TEXT,                       -- JSON array, NULL = no allowlist
    cdp_port       INTEGER,
    result         TEXT,                       -- pass | fail | aborted | NULL
    notes          TEXT                        -- free-form
);

CREATE INDEX IF NOT EXISTS idx_helixqa_browser_sessions_started
    ON helixqa_browser_sessions (started_at);

CREATE INDEX IF NOT EXISTS idx_helixqa_browser_sessions_engine
    ON helixqa_browser_sessions (engine);

CREATE INDEX IF NOT EXISTS idx_helixqa_browser_sessions_result
    ON helixqa_browser_sessions (result);
