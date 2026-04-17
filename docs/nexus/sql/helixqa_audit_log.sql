-- helixqa_audit_log — append-only RBAC decision history.

CREATE TABLE IF NOT EXISTS helixqa_audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     TEXT NOT NULL,
    user_email  TEXT,
    team        TEXT,
    role        TEXT NOT NULL,
    action      TEXT NOT NULL,
    resource    TEXT,
    allowed     INTEGER NOT NULL, -- SQLite bool
    reason      TEXT,
    at          TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_helixqa_audit_user   ON helixqa_audit_log (user_id);
CREATE INDEX IF NOT EXISTS idx_helixqa_audit_action ON helixqa_audit_log (action);
CREATE INDEX IF NOT EXISTS idx_helixqa_audit_at     ON helixqa_audit_log (at);
