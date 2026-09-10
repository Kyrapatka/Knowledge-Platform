ALTER TABLE training_events ADD COLUMN direction TEXT NOT NULL DEFAULT '' CHECK (direction IN ('','foreign','native'));
CREATE INDEX training_events_direction_idx ON training_events(user_id,material_id,progress_version_after DESC) WHERE direction <> '';
