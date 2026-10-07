CREATE TABLE material_bulk_commands (
 user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 command_id UUID NOT NULL,
 request_hash TEXT NOT NULL,
 response JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(user_id,command_id)
);
