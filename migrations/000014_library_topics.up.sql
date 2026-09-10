-- Topic is one optional material metadata value. Keep legacy category and all
-- other workshop definitions intact; existing presentation snapshots are not
-- rewritten when the live folder schema gains this field.
UPDATE folders
SET config = jsonb_set(config, '{metadata_schema}',
    (CASE WHEN jsonb_typeof(config->'metadata_schema') = 'object'
        THEN config->'metadata_schema' ELSE '{}'::jsonb END) || jsonb_build_object(
        'fields', (CASE WHEN jsonb_typeof(config #> '{metadata_schema,fields}') = 'array'
        THEN config #> '{metadata_schema,fields}' ELSE '[]'::jsonb END) ||
        '[{"key":"topic","label":"Topic","required":false,"active":true}]'::jsonb)),
    config_version = config_version + 1,
    updated_at = NOW()
WHERE NOT EXISTS (
    SELECT 1 FROM jsonb_array_elements(CASE WHEN jsonb_typeof(config #> '{metadata_schema,fields}') = 'array'
        THEN config #> '{metadata_schema,fields}' ELSE '[]'::jsonb END) field
    WHERE field->>'key' = 'topic'
);

CREATE INDEX training_answer_activity ON training_events(user_id, created_at)
    WHERE action IN ('correct', 'wrong');
