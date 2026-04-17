-- helixqa_mobile_devices — one row per iOS / Android / Android TV device
-- or emulator that Nexus has ever connected to. Powers the dashboard
-- "available devices" panel and the orchestrator's device picker.

CREATE TABLE IF NOT EXISTS helixqa_mobile_devices (
    udid          TEXT    PRIMARY KEY,
    platform      TEXT    NOT NULL,           -- android | androidtv | ios
    device_name   TEXT    NOT NULL,
    os_version    TEXT,
    model         TEXT,
    is_emulator   INTEGER NOT NULL DEFAULT 0,
    last_seen     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    availability  TEXT    NOT NULL DEFAULT 'unknown', -- available | busy | offline | quarantined
    quarantine_reason TEXT,
    device_farm   TEXT,                       -- local | browserstack | saucelabs | ...
    notes         TEXT
);

CREATE INDEX IF NOT EXISTS idx_helixqa_mobile_devices_platform
    ON helixqa_mobile_devices (platform);
CREATE INDEX IF NOT EXISTS idx_helixqa_mobile_devices_avail
    ON helixqa_mobile_devices (availability);
CREATE INDEX IF NOT EXISTS idx_helixqa_mobile_devices_last_seen
    ON helixqa_mobile_devices (last_seen);
