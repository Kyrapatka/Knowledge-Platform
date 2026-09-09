ALTER TABLE folders ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE materials ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX active_materials_by_folder ON materials(folder_id,created_at,id) WHERE deleted_at IS NULL;
CREATE INDEX active_folders_by_owner ON folders(owner_id,created_at,id) WHERE deleted_at IS NULL;
ALTER TABLE training_plans ADD COLUMN version INTEGER NOT NULL DEFAULT 1 CHECK (version>0);
CREATE TABLE training_plan_changes (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    plan_id UUID NOT NULL REFERENCES training_plans(id),
    command_id UUID NOT NULL,
    details JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(user_id,command_id)
);
CREATE INDEX changes_by_plan ON training_plan_changes(plan_id,created_at,id);
