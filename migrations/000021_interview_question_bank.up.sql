-- One seed question per account, independent of folders and interview profiles.
-- Existing material IDs and SRS progress are never replaced during this upgrade.
ALTER TABLE interview_question_profiles ADD owner_id UUID REFERENCES users(id) ON DELETE CASCADE;
UPDATE interview_question_profiles q SET owner_id=f.owner_id FROM folders f WHERE f.id=q.folder_id;
ALTER TABLE interview_question_profiles ALTER owner_id SET NOT NULL;
ALTER TABLE interview_question_profiles ADD legacy_seed_key TEXT;
WITH duplicates AS (
  SELECT material_id, seed_key, row_number() OVER (PARTITION BY owner_id,seed_key ORDER BY created_at,material_id) AS n
  FROM interview_question_profiles WHERE seed_key IS NOT NULL
)
UPDATE interview_question_profiles q SET legacy_seed_key=d.seed_key,seed_key=NULL
FROM duplicates d WHERE q.material_id=d.material_id AND d.n>1;
CREATE UNIQUE INDEX interview_question_bank_seed_key ON interview_question_profiles(owner_id,seed_key) WHERE seed_key IS NOT NULL;
ALTER TABLE interview_question_profiles ADD followup_weight SMALLINT NOT NULL DEFAULT 5 CHECK(followup_weight BETWEEN 0 AND 10);
ALTER TABLE interview_question_profiles ADD level_min SMALLINT NOT NULL DEFAULT 1 CHECK(level_min BETWEEN 1 AND 5);
ALTER TABLE interview_question_profiles ADD level_max SMALLINT NOT NULL DEFAULT 5 CHECK(level_max BETWEEN 1 AND 5);
ALTER TABLE interview_question_profiles ADD CONSTRAINT interview_question_levels CHECK(level_min<=level_max);
ALTER TABLE interview_question_profiles ADD topic TEXT NOT NULL DEFAULT '';
ALTER TABLE interview_question_profiles ADD subtopic TEXT NOT NULL DEFAULT '';
ALTER TABLE interview_question_profiles ADD seed_revision TEXT NOT NULL DEFAULT '';
UPDATE interview_question_profiles q SET topic=COALESCE(m.metadata->>'topic',''),subtopic=COALESCE(m.metadata->>'category','') FROM materials m WHERE m.id=q.material_id;
ALTER TABLE interview_question_concepts DROP CONSTRAINT interview_question_concepts_role_check;
ALTER TABLE interview_question_concepts ADD CONSTRAINT interview_question_concepts_role_check CHECK(role IN('primary','tested','answer','hook','prerequisite','wrong_fallback'));
ALTER TABLE interview_question_concepts ADD ordinal INTEGER NOT NULL DEFAULT 0 CHECK(ordinal>=0);
CREATE INDEX interview_question_concepts_ordered ON interview_question_concepts(material_id,role,ordinal);
CREATE TABLE interview_profiles (
  slug TEXT PRIMARY KEY, label TEXT NOT NULL
);
CREATE TABLE interview_question_memberships (
  material_id UUID NOT NULL REFERENCES interview_question_profiles(material_id) ON DELETE CASCADE,
  profile_slug TEXT NOT NULL REFERENCES interview_profiles(slug) ON DELETE RESTRICT,
  PRIMARY KEY(material_id,profile_slug)
);
CREATE INDEX interview_memberships_lookup ON interview_question_memberships(profile_slug,material_id);
CREATE INDEX interview_question_bank_filter ON interview_question_profiles(owner_id,status,domain,topic,level_min,level_max);
ALTER TABLE interview_concept_aliases ADD seed_managed BOOLEAN NOT NULL DEFAULT FALSE;
CREATE TABLE interview_bank_alias_policies (
  owner_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  seed_revision TEXT NOT NULL, scoped_aliases JSONB NOT NULL, ambiguous_aliases JSONB NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE interview_bank_import_runs (
  id UUID PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  seed_revision TEXT NOT NULL, questions_sha256 TEXT NOT NULL, concepts_sha256 TEXT NOT NULL,
  domains JSONB NOT NULL, forced_content BOOLEAN NOT NULL DEFAULT FALSE,
  started_at TIMESTAMPTZ NOT NULL, finished_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL CHECK(status='completed'), created_count INTEGER NOT NULL,
  updated_count INTEGER NOT NULL, skipped_count INTEGER NOT NULL
);
