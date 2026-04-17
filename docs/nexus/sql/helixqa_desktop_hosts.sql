-- helixqa_desktop_hosts — one row per Windows / macOS / Linux host that
-- HelixQA has ever reached. Powers the orchestrator's host picker and
-- the availability panel on the Grafana dashboard.

CREATE TABLE IF NOT EXISTS helixqa_desktop_hosts (
    hostname      TEXT    PRIMARY KEY,
    platform      TEXT    NOT NULL,           -- windows | macos | linux
    role          TEXT    NOT NULL,           -- runner | farm | ci | dev
    os_version    TEXT,
    arch          TEXT,                       -- x86_64 | arm64 | ...
    last_probe    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    available     INTEGER NOT NULL DEFAULT 1, -- SQLite bool
    waypoint      TEXT,                       -- ssh user@host | rdp://... etc
    notes         TEXT
);

CREATE INDEX IF NOT EXISTS idx_helixqa_desktop_hosts_platform
    ON helixqa_desktop_hosts (platform);
CREATE INDEX IF NOT EXISTS idx_helixqa_desktop_hosts_available
    ON helixqa_desktop_hosts (available);
CREATE INDEX IF NOT EXISTS idx_helixqa_desktop_hosts_last_probe
    ON helixqa_desktop_hosts (last_probe);
