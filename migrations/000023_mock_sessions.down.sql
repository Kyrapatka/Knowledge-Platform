-- Refuse rollback while planless history exists; never delete user history.
ALTER TABLE training_sessions ALTER COLUMN plan_id SET NOT NULL;
ALTER TABLE training_events ALTER COLUMN plan_id SET NOT NULL;
ALTER TABLE training_undo ALTER COLUMN plan_id SET NOT NULL;
DROP INDEX one_active_mock_per_user;
ALTER TABLE training_events DROP COLUMN graph_selection_event_id;
ALTER TABLE training_sessions DROP CONSTRAINT mock_session_graph_only;
ALTER TABLE training_events DROP CONSTRAINT mock_event_practice_only;
