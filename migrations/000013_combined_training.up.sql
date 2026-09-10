ALTER TABLE training_sessions
    ADD COLUMN combined BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN selection JSONB NOT NULL DEFAULT '[]'::jsonb
        CHECK (jsonb_typeof(selection) = 'array');
