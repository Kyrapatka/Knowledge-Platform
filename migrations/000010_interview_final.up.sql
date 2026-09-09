ALTER TABLE training_events DROP CONSTRAINT training_events_action_check;
ALTER TABLE training_events DROP CONSTRAINT training_events_check;
ALTER TABLE training_events ADD CONSTRAINT training_events_action_check
    CHECK (action IN ('correct','wrong','advance','rollback','skip_rehab','start_final'));
ALTER TABLE training_events ADD CONSTRAINT training_events_check
    CHECK (action IN ('skip_rehab','start_final') OR presentation_id IS NOT NULL);
