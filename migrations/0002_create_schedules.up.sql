-- Schedules are recurrence templates. Tasks are the materialised instances.
CREATE TABLE IF NOT EXISTS schedules (
    id             BIGSERIAL    PRIMARY KEY,
    title          TEXT         NOT NULL,
    description    TEXT         NOT NULL DEFAULT '',
    default_status TEXT         NOT NULL,
    kind           TEXT         NOT NULL
                   CHECK (kind IN ('daily_every_n', 'monthly_days', 'specific_dates', 'even_odd')),
    params         JSONB        NOT NULL,
    start_date     DATE         NOT NULL,
    end_date       DATE,
    timezone       TEXT         NOT NULL DEFAULT 'Europe/Moscow',
    active         BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_schedules_end_after_start CHECK (end_date IS NULL OR end_date >= start_date)
);

-- Partial index: only active schedules are scanned by the generator on every tick.
CREATE INDEX IF NOT EXISTS idx_schedules_active
    ON schedules (id)
    WHERE active = TRUE;

-- Add recurrence columns to tasks.
--   schedule_id  links a materialised task back to its template (SET NULL on delete
--                preserves historical tasks when a schedule is removed).
--   due_date     the calendar date this instance represents; NULL for one-off tasks.
ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS schedule_id BIGINT REFERENCES schedules (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS due_date    DATE;

-- Idempotency guarantee for the generator worker: at most one task per
-- (schedule, due_date) pair so that re-runs and restarts are safe.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_tasks_schedule_due
    ON tasks (schedule_id, due_date)
    WHERE schedule_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tasks_schedule_id ON tasks (schedule_id);
CREATE INDEX IF NOT EXISTS idx_tasks_due_date    ON tasks (due_date);
