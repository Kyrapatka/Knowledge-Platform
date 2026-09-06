-- Plan configuration is a snapshot; horizon_days applies separately to each
-- material. There is deliberately no shared target_at on training_plans.
CREATE TABLE training_plans (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    track TEXT NOT NULL CHECK (track IN ('default', 'long_term', 'cram')),
    algorithm_key TEXT NOT NULL CHECK (algorithm_key <> ''),
    algorithm_version INTEGER NOT NULL CHECK (algorithm_version > 0),
    status TEXT NOT NULL CHECK (status IN ('active', 'completed', 'cancelled')),
    config JSONB NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (id, user_id, track)
);

CREATE TABLE training_plan_sources (
    plan_id UUID NOT NULL REFERENCES training_plans(id),
    folder_id UUID NOT NULL REFERENCES folders(id),
    PRIMARY KEY (plan_id, folder_id)
);

CREATE TABLE user_material_progress (
    user_id UUID NOT NULL REFERENCES users(id),
    material_id UUID NOT NULL REFERENCES materials(id),
    track TEXT NOT NULL CHECK (track IN ('default', 'long_term', 'cram')),
    plan_id UUID,
    algorithm_key TEXT NOT NULL CHECK (algorithm_key <> ''),
    algorithm_version INTEGER NOT NULL CHECK (algorithm_version > 0),
    stage INTEGER NOT NULL CHECK (stage > 0),
    correct_count INTEGER NOT NULL DEFAULT 0 CHECK (correct_count >= 0),
    wrong_count INTEGER NOT NULL DEFAULT 0 CHECK (wrong_count >= 0),
    consecutive_correct INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_correct >= 0),
    consecutive_wrong INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_wrong >= 0),
    difficulty_override TEXT CHECK (difficulty_override IN ('easy', 'medium', 'hard')),
    learning_started_at TIMESTAMPTZ NOT NULL,
    target_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    stage_last_review_at TIMESTAMPTZ,
    stage_review_at TIMESTAMPTZ,
    rehab_active BOOLEAN NOT NULL DEFAULT FALSE,
    rehab_review_at TIMESTAMPTZ,
    rehab_step INTEGER NOT NULL DEFAULT 0,
    rehab_consecutive_correct INTEGER NOT NULL DEFAULT 0 CHECK (rehab_consecutive_correct >= 0),
    extra_review_at TIMESTAMPTZ,
    next_review_at TIMESTAMPTZ GENERATED ALWAYS AS (
        LEAST(stage_review_at, CASE WHEN rehab_active THEN rehab_review_at END, extra_review_at)
    ) STORED,
    version INTEGER NOT NULL CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CHECK ((track = 'cram') = (plan_id IS NOT NULL)),
    CHECK (target_at IS NULL OR target_at > learning_started_at),
    CHECK (
        (rehab_active AND rehab_step IN (1,2) AND rehab_review_at IS NOT NULL AND extra_review_at IS NULL)
        OR
        (NOT rehab_active AND rehab_step = 0 AND rehab_review_at IS NULL AND rehab_consecutive_correct = 0)
    ),
    FOREIGN KEY (plan_id, user_id, track) REFERENCES training_plans(id, user_id, track)
);

CREATE UNIQUE INDEX progress_persistent_identity ON user_material_progress(user_id, material_id, track)
    WHERE plan_id IS NULL;
CREATE UNIQUE INDEX progress_cram_identity ON user_material_progress(user_id, material_id, plan_id)
    WHERE plan_id IS NOT NULL;
CREATE INDEX progress_due ON user_material_progress(user_id, track, next_review_at, material_id)
    WHERE next_review_at IS NOT NULL;
