-- helixqa_cross_flows — one row per cross-platform flow run.
CREATE TABLE IF NOT EXISTS helixqa_cross_flows (
    flow_id      TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    started_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ended_at     TIMESTAMP,
    result       TEXT,            -- pass | fail | aborted
    failed_step  INTEGER,
    notes        TEXT
);

-- helixqa_flow_steps — one row per step within a flow.
CREATE TABLE IF NOT EXISTS helixqa_flow_steps (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    flow_id      TEXT NOT NULL,
    step_index   INTEGER NOT NULL,
    name         TEXT NOT NULL,
    platform     TEXT NOT NULL,
    started_at   TIMESTAMP NOT NULL,
    ended_at     TIMESTAMP,
    result       TEXT,            -- pass | fail | skipped
    reason       TEXT,
    FOREIGN KEY (flow_id) REFERENCES helixqa_cross_flows(flow_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_helixqa_flow_steps_flow ON helixqa_flow_steps (flow_id);
