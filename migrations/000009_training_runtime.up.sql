ALTER TABLE folders ADD COLUMN training_config JSONB NOT NULL DEFAULT '{}',
    ADD COLUMN training_config_version BIGINT NOT NULL DEFAULT 1 CHECK (training_config_version > 0);
UPDATE folders SET training_config = jsonb_build_object(
    'default_algorithm_key', CASE template_key WHEN 'interview_questions' THEN 'interview_long_term'
      WHEN 'formulas' THEN 'formula_adaptive' ELSE 'english_basic' END,
    'pool_size', CASE template_key WHEN 'english_words' THEN 8 ELSE 5 END);

CREATE TABLE training_sessions (
    id UUID PRIMARY KEY,
    plan_id UUID NOT NULL REFERENCES training_plans(id),
    user_id UUID NOT NULL REFERENCES users(id),
    status TEXT NOT NULL CHECK (status IN ('active','completed','cancelled')),
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    CHECK ((status = 'active') = (finished_at IS NULL))
);
CREATE UNIQUE INDEX one_active_session_per_plan ON training_sessions(plan_id) WHERE status = 'active';
CREATE INDEX sessions_by_user ON training_sessions(user_id, created_at DESC);

CREATE TABLE training_session_items (
    session_id UUID NOT NULL REFERENCES training_sessions(id),
    material_id UUID NOT NULL REFERENCES materials(id),
    position BIGINT NOT NULL CHECK (position >= 0),
    state TEXT NOT NULL CHECK (state IN ('active','completed')),
    presentation JSONB,
    PRIMARY KEY(session_id, material_id)
);
CREATE INDEX session_pool ON training_session_items(session_id, position) WHERE state = 'active';

CREATE TABLE training_events (
    id UUID PRIMARY KEY,
    command_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    plan_id UUID NOT NULL REFERENCES training_plans(id),
    session_id UUID NOT NULL REFERENCES training_sessions(id),
    material_id UUID NOT NULL REFERENCES materials(id),
    presentation_id UUID UNIQUE,
    action TEXT NOT NULL CHECK (action IN ('correct','wrong','advance','rollback','skip_rehab')),
    kind TEXT NOT NULL CHECK (kind IN ('stage','rehab','extra')),
    algorithm_key TEXT NOT NULL,
    algorithm_version INTEGER NOT NULL,
    stage_before INTEGER NOT NULL,
    stage_after INTEGER NOT NULL,
    progress_version_before INTEGER NOT NULL,
    progress_version_after INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(user_id, command_id),
    CHECK (action = 'skip_rehab' OR presentation_id IS NOT NULL)
);
CREATE INDEX events_by_session ON training_events(session_id, created_at, id);

CREATE TABLE training_commands (
    user_id UUID NOT NULL REFERENCES users(id),
    command_id UUID NOT NULL,
    request_hash TEXT NOT NULL,
    response JSONB NOT NULL,
    PRIMARY KEY(user_id, command_id)
);
