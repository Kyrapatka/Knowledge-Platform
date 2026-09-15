ALTER TABLE training_events DROP CONSTRAINT training_events_action_check;
ALTER TABLE training_events ADD CONSTRAINT training_events_action_check CHECK(action IN ('correct','wrong','advance','rollback','skip_rehab','start_final','review_early','next_route'));
ALTER TABLE training_events DROP CONSTRAINT training_events_event_mode_check;
ALTER TABLE training_events ADD CONSTRAINT training_events_event_mode_check CHECK(event_mode IN ('scheduled','graph_probe','graph_navigation'));
