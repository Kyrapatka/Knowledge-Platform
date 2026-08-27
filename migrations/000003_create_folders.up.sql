CREATE TABLE folders (
                         id UUID PRIMARY KEY,
                         owner_id UUID NOT NULL,
                         title TEXT NOT NULL,
                         description TEXT NOT NULL DEFAULT '',
                         template_key TEXT NOT NULL,
                         created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                         updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

                         CONSTRAINT fk_folders_owner
                             FOREIGN KEY (owner_id)
                                 REFERENCES users(id)
                                 ON DELETE CASCADE
);

CREATE INDEX idx_folders_owner_id
    ON folders(owner_id);