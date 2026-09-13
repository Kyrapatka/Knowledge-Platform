ALTER TABLE interview_graph_selection_events ADD COLUMN selection_order INTEGER NOT NULL DEFAULT 0;
WITH numbered AS (
    SELECT id, row_number() OVER (PARTITION BY session_id ORDER BY created_at,id) AS position
    FROM interview_graph_selection_events
)
UPDATE interview_graph_selection_events e SET selection_order=n.position FROM numbered n WHERE e.id=n.id;
CREATE INDEX interview_profiles_ready_folder ON interview_question_profiles(folder_id,material_id) WHERE status='ready';
CREATE INDEX interview_aliases_concept ON interview_concept_aliases(concept_id);
CREATE INDEX interview_graph_events_current ON interview_graph_selection_events(session_id,selection_order DESC,id) WHERE undone_at IS NULL;
CREATE INDEX interview_graph_training_events ON training_events(session_id,presentation_id) WHERE undone_at IS NULL;
