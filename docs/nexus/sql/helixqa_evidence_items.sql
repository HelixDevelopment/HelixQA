-- helixqa_evidence_items — index of artefacts stored in the evidence
-- vault. One row per item; the actual bytes live in the EvidenceStore
-- backend (file, S3, MinIO, ...).

CREATE TABLE IF NOT EXISTS helixqa_evidence_items (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   TEXT NOT NULL,
    step_id      TEXT,
    name         TEXT NOT NULL,
    url          TEXT NOT NULL,
    size_bytes   INTEGER NOT NULL DEFAULT 0,
    kind         TEXT NOT NULL,    -- screenshot | video | log | trace | bank | other
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_helixqa_evidence_session ON helixqa_evidence_items (session_id);
CREATE INDEX IF NOT EXISTS idx_helixqa_evidence_kind    ON helixqa_evidence_items (kind);
CREATE INDEX IF NOT EXISTS idx_helixqa_evidence_created ON helixqa_evidence_items (created_at);
