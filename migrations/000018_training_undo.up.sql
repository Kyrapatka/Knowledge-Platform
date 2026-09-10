ALTER TABLE training_events ADD COLUMN undone_at TIMESTAMPTZ;
CREATE TABLE training_undo (
    id BIGSERIAL PRIMARY KEY,
    event_id UUID NOT NULL UNIQUE REFERENCES training_events(id),
    user_id UUID NOT NULL REFERENCES users(id),
    plan_id UUID NOT NULL REFERENCES training_plans(id),
    session_id UUID NOT NULL REFERENCES training_sessions(id),
    material_id UUID NOT NULL REFERENCES materials(id),
    expected_version INTEGER NOT NULL,
    snapshot JSONB NOT NULL
);
CREATE INDEX training_undo_user_idx ON training_undo(user_id,id DESC);
