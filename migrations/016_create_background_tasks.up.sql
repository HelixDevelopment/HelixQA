CREATE TYPE background_task_status AS ENUM ('pending', 'running', 'completed', 'failed', 'dead');

CREATE TABLE background_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status background_task_status NOT NULL DEFAULT 'pending',
    priority SMALLINT NOT NULL DEFAULT 0,
    attempts SMALLINT NOT NULL DEFAULT 0,
    max_attempts SMALLINT NOT NULL DEFAULT 5,
    last_error TEXT NULL,
    next_run_at TIMESTAMPTZ NOT NULL,
    locked_by UUID NULL,
    locked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_background_tasks_status_next_run ON background_tasks (status, next_run_at);
CREATE INDEX idx_background_tasks_locked_by ON background_tasks (locked_by);
CREATE INDEX idx_background_tasks_type ON background_tasks (type);

CREATE TRIGGER set_background_tasks_updated_at
    BEFORE UPDATE ON background_tasks
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
