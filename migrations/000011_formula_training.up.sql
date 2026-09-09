CREATE TABLE formula_exercises (
    id UUID PRIMARY KEY,
    material_id UUID NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
    problem TEXT NOT NULL CHECK (btrim(problem) <> ''),
    answer TEXT NOT NULL CHECK (btrim(answer) <> ''),
    solution TEXT NOT NULL CHECK (btrim(solution) <> ''),
    hint TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX formula_exercises_material ON formula_exercises(material_id, created_at, id);
-- Historical identifiers survive exercise deletion. Displayed content is
-- already snapshotted in session items and command receipts.
ALTER TABLE training_events ADD COLUMN exercise_id UUID,
    ADD COLUMN practice_mode TEXT CHECK (practice_mode IN ('worked','faded','independent','mixed','maintenance'));
