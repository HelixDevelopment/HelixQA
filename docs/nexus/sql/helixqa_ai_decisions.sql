-- helixqa_ai_decisions — one row per LLM call driven by pkg/nexus/ai.
-- Powers the cost dashboard and the operator-visible decision log.

CREATE TABLE IF NOT EXISTS helixqa_ai_decisions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     TEXT NOT NULL,
    step           INTEGER NOT NULL,
    capability     TEXT NOT NULL,        -- navigator | healer | generator | other
    provider       TEXT NOT NULL,
    model          TEXT NOT NULL,
    action_kind    TEXT,                 -- click | type | scroll | wait | done | heal | generate
    target         TEXT,
    reasoning      TEXT,
    confidence     REAL,
    tokens_in      INTEGER NOT NULL DEFAULT 0,
    tokens_out     INTEGER NOT NULL DEFAULT 0,
    cost_cents     INTEGER NOT NULL DEFAULT 0,
    outcome        TEXT,                 -- pass | fail | aborted | refused
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_helixqa_ai_decisions_session ON helixqa_ai_decisions (session_id);
CREATE INDEX IF NOT EXISTS idx_helixqa_ai_decisions_cap     ON helixqa_ai_decisions (capability);
CREATE INDEX IF NOT EXISTS idx_helixqa_ai_decisions_created ON helixqa_ai_decisions (created_at);
