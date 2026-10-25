CREATE TABLE mock_presets (
 id UUID PRIMARY KEY,
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 name TEXT NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 120),
 folder_ids JSONB NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(folder_ids) = 'array'),
 settings JSONB,
 version INTEGER NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 last_used_at TIMESTAMPTZ
);
CREATE INDEX mock_presets_owner ON mock_presets(user_id, created_at DESC);
