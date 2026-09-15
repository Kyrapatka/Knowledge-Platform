-- Navigation history cannot be represented by the old constraints. Refuse a
-- lossy downgrade until the operator explicitly handles those events.
ALTER TABLE training_events DROP CONSTRAINT training_events_action_check;
ALTER TABLE training_events ADD CONSTRAINT training_events_action_check CHECK(action IN ('correct','wrong','advance','rollback','skip_rehab','start_final','review_early'));
ALTER TABLE training_events DROP CONSTRAINT training_events_event_mode_check;
ALTER TABLE training_events ADD CONSTRAINT training_events_event_mode_check CHECK(event_mode IN ('scheduled','graph_probe'));
