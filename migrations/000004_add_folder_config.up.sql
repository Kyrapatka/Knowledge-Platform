ALTER TABLE folders
    ADD COLUMN config JSONB NOT NULL DEFAULT '{}'::jsonb;