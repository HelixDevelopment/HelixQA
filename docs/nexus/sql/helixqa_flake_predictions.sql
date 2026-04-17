-- helixqa_flake_predictions — one row per prediction emitted by the
-- pkg/nexus/ai.Predictor.

CREATE TABLE IF NOT EXISTS helixqa_flake_predictions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    test_id         TEXT NOT NULL,
    platform        TEXT NOT NULL,
    probability     REAL NOT NULL,
    threshold       REAL NOT NULL,
    decision        INTEGER NOT NULL,  -- 0=run, 1=retry-preemptively, 2=skip
    features_json   TEXT NOT NULL,
    predicted_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_helixqa_flake_predictions_test ON helixqa_flake_predictions (test_id);
CREATE INDEX IF NOT EXISTS idx_helixqa_flake_predictions_time ON helixqa_flake_predictions (predicted_at);
