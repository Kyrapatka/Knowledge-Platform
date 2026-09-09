-- Refuses rollback if start_final history exists; never silently delete events.
ALTER TABLE training_events DROP CONSTRAINT training_events_action_check;
ALTER TABLE training_events DROP CONSTRAINT training_events_check;
ALTER TABLE training_events ADD CONSTRAINT training_events_action_check
    CHECK (action IN ('correct','wrong','advance','rollback','skip_rehab'));
ALTER TABLE training_events ADD CONSTRAINT training_events_check
    CHECK (action = 'skip_rehab' OR presentation_id IS NOT NULL);
