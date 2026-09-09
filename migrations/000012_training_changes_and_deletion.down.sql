-- Refuse to resurrect deleted library content when rolling back this migration.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM materials WHERE deleted_at IS NOT NULL)
 OR EXISTS (SELECT 1 FROM folders WHERE deleted_at IS NOT NULL)
 OR EXISTS (SELECT 1 FROM training_plan_changes) THEN
  RAISE EXCEPTION 'Cannot roll back while deleted content or algorithm-change history exists';
 END IF;
END $$;
DROP TABLE training_plan_changes;
ALTER TABLE training_plans DROP COLUMN version;
DROP INDEX active_materials_by_folder;
DROP INDEX active_folders_by_owner;
ALTER TABLE materials DROP COLUMN deleted_at;
ALTER TABLE folders DROP COLUMN deleted_at;
