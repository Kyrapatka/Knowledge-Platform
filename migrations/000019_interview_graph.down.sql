DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM training_sessions WHERE selection_strategy='interview_graph_v1') THEN
   RAISE EXCEPTION 'Cannot remove graph schema while interview sessions exist';
 END IF;
END $$;
ALTER TABLE training_events DROP COLUMN event_mode, DROP COLUMN review_credit;
DROP TABLE interview_graph_selection_events, interview_graph_session_state;
ALTER TABLE training_sessions DROP COLUMN selection_strategy;
DROP TABLE interview_seed_folders, interview_concept_edges, interview_question_concepts,
  interview_concept_aliases, interview_concepts, interview_question_profiles;
-- Keep answer optional: reintroducing required answers would invalidate drafts.
