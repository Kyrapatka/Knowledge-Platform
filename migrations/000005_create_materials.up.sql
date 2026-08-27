CREATE TABLE materials (
                           id UUID PRIMARY KEY,

                           folder_id UUID NOT NULL,

                           values JSONB NOT NULL DEFAULT '{}'::jsonb,
                           metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

                           created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                           updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

                           CONSTRAINT fk_materials_folder
                               FOREIGN KEY (folder_id)
                                   REFERENCES folders(id)
                                   ON DELETE CASCADE
);

CREATE INDEX idx_materials_folder_id
    ON materials(folder_id);