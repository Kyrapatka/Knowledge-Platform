-- Mock interviews share graph/session storage, but do not own an SRS plan.
ALTER TABLE training_sessions ALTER COLUMN plan_id DROP NOT NULL;
ALTER TABLE training_events ALTER COLUMN plan_id DROP NOT NULL;
ALTER TABLE training_undo ALTER COLUMN plan_id DROP NOT NULL;
ALTER TABLE training_events ADD graph_selection_event_id UUID REFERENCES interview_graph_selection_events(id);
ALTER TABLE training_sessions ADD CONSTRAINT mock_session_graph_only
 CHECK (plan_id IS NOT NULL OR (selection_strategy='interview_graph_v1' AND NOT combined));
ALTER TABLE training_events ADD CONSTRAINT mock_event_practice_only
 CHECK (plan_id IS NOT NULL OR (NOT review_credit AND event_mode IN ('graph_probe','graph_navigation') AND progress_version_before=0 AND progress_version_after=0));
CREATE UNIQUE INDEX one_active_mock_per_user ON training_sessions(user_id) WHERE plan_id IS NULL AND status='active';
