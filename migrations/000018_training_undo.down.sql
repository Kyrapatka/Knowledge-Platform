DROP TABLE training_undo;
-- Do not silently count previously undone answers again on rollback.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM training_events WHERE undone_at IS NOT NULL) THEN
   RAISE EXCEPTION 'Cannot remove undo history after answers have been undone';
 END IF;
END $$;
ALTER TABLE training_events DROP COLUMN undone_at;
