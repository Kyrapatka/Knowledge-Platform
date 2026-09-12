-- Concepts are shared by a user's interview folders; another account cannot
-- change their vocabulary, aliases, or the resulting interview route.
CREATE TABLE interview_question_profiles (
  material_id UUID PRIMARY KEY REFERENCES materials(id) ON DELETE CASCADE,
  folder_id UUID NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  seed_key TEXT,
  domain TEXT NOT NULL DEFAULT '',
  frequency SMALLINT NOT NULL CHECK(frequency BETWEEN 1 AND 10),
  frequency_confidence DOUBLE PRECISION NOT NULL DEFAULT 0.5 CHECK(frequency_confidence BETWEEN 0 AND 1),
  interview_difficulty SMALLINT NOT NULL CHECK(interview_difficulty BETWEEN 1 AND 5),
  specificity SMALLINT NOT NULL CHECK(specificity BETWEEN 1 AND 5),
  root_weight SMALLINT NOT NULL CHECK(root_weight BETWEEN 0 AND 10),
  status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','ready','archived')),
  profile_version INTEGER NOT NULL DEFAULT 1 CHECK(profile_version > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(folder_id, seed_key)
);
CREATE TABLE interview_concepts (
  id UUID PRIMARY KEY, owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  slug TEXT NOT NULL, display_name TEXT NOT NULL, domain TEXT NOT NULL, topic TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(owner_id,slug)
);
CREATE TABLE interview_concept_aliases (
  id UUID PRIMARY KEY, concept_id UUID NOT NULL REFERENCES interview_concepts(id) ON DELETE CASCADE,
  alias TEXT NOT NULL, normalized_alias TEXT NOT NULL, language TEXT NOT NULL DEFAULT 'any',
  weight DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK(weight > 0 AND weight <= 2),
  whole_word BOOLEAN NOT NULL DEFAULT TRUE, constraints JSONB NOT NULL DEFAULT '{}',
  UNIQUE(concept_id,normalized_alias,language)
);
CREATE TABLE interview_question_concepts (
  material_id UUID NOT NULL REFERENCES materials(id) ON DELETE CASCADE,
  concept_id UUID NOT NULL REFERENCES interview_concepts(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK(role IN ('primary','tested','hook','prerequisite')),
  weight DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK(weight > 0 AND weight <= 2),
  PRIMARY KEY(material_id,concept_id,role)
);
CREATE INDEX interview_question_concepts_lookup ON interview_question_concepts(concept_id,role);
CREATE TABLE interview_concept_edges (
  from_concept_id UUID NOT NULL REFERENCES interview_concepts(id) ON DELETE CASCADE,
  to_concept_id UUID NOT NULL REFERENCES interview_concepts(id) ON DELETE CASCADE,
  relation TEXT NOT NULL, weight DOUBLE PRECISION NOT NULL DEFAULT 1 CHECK(weight > 0 AND weight <= 2),
  PRIMARY KEY(from_concept_id,to_concept_id,relation)
);
CREATE TABLE interview_seed_folders (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  domain TEXT NOT NULL, folder_id UUID NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
  PRIMARY KEY(user_id,domain)
);
ALTER TABLE training_sessions ADD selection_strategy TEXT NOT NULL DEFAULT 'random'
  CHECK(selection_strategy IN ('random','interview_graph_v1'));
CREATE TABLE interview_graph_session_state (
  session_id UUID PRIMARY KEY REFERENCES training_sessions(id) ON DELETE CASCADE,
  strategy_version INTEGER NOT NULL DEFAULT 1,
  state JSONB NOT NULL, version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE interview_graph_selection_events (
  id UUID PRIMARY KEY, session_id UUID NOT NULL REFERENCES training_sessions(id) ON DELETE CASCADE,
  from_material_id UUID, to_material_id UUID NOT NULL,
  root_index INTEGER NOT NULL, depth_before INTEGER NOT NULL, depth_after INTEGER NOT NULL,
  detected_concepts JSONB NOT NULL, candidates JSONB NOT NULL,
  selection_reason TEXT NOT NULL, review_credit BOOLEAN NOT NULL,
  random_seed BIGINT NOT NULL, raw_answer TEXT,
  snapshot JSONB NOT NULL,п
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), undone_at TIMESTAMPTZ
);
CREATE INDEX interview_graph_selections_session ON interview_graph_selection_events(session_id,created_at);
ALTER TABLE training_events ADD review_credit BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE training_events ADD event_mode TEXT NOT NULL DEFAULT 'scheduled' CHECK(event_mode IN ('scheduled','graph_probe'));

-- Permit genuine draft questions. Scheduling still requires a usable answer.
UPDATE folders SET config=jsonb_set(config,'{schema,fields}',
  (SELECT jsonb_agg(CASE WHEN f->>'key'='answer' THEN jsonb_set(f,'{required}','false') ELSE f END ORDER BY n)
   FROM jsonb_array_elements(config->'schema'->'fields') WITH ORDINALITY AS field(f,n))),
  config_version=config_version+1, updated_at=NOW()
WHERE template_key='interview_questions' AND deleted_at IS NULL AND EXISTS
  (SELECT 1 FROM jsonb_array_elements(config->'schema'->'fields') AS f WHERE f->>'key'='answer' AND f->>'required'='true');
