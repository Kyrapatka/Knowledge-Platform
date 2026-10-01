-- Refuse a downgrade that would erase bank imports, memberships or new graph roles.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM interview_question_concepts WHERE role IN ('answer','wrong_fallback') OR ordinal <> 0)
 OR EXISTS (SELECT 1 FROM interview_question_memberships)
 OR EXISTS (SELECT 1 FROM interview_bank_import_runs)
 OR EXISTS (SELECT 1 FROM interview_bank_alias_policies)
 OR EXISTS (SELECT 1 FROM interview_profiles)
 OR EXISTS (SELECT 1 FROM interview_concept_aliases WHERE seed_managed)
 OR EXISTS (SELECT 1 FROM interview_question_profiles WHERE followup_weight <> 5 OR level_min <> 1 OR level_max <> 5 OR seed_revision <> '' OR topic <> '' OR subtopic <> '') THEN
  RAISE EXCEPTION 'Cannot roll back question bank while bank metadata or import history exists';
 END IF;
END $$;
DROP TABLE interview_bank_import_runs;
DROP TABLE interview_bank_alias_policies;
ALTER TABLE interview_concept_aliases DROP COLUMN seed_managed;
DROP TABLE interview_question_memberships;
DROP TABLE interview_profiles;
DROP INDEX interview_question_bank_filter;
DROP INDEX interview_question_concepts_ordered;
ALTER TABLE interview_question_concepts DROP CONSTRAINT interview_question_concepts_role_check;
ALTER TABLE interview_question_concepts ADD CONSTRAINT interview_question_concepts_role_check CHECK(role IN ('primary','tested','hook','prerequisite'));
ALTER TABLE interview_question_concepts DROP COLUMN ordinal;
ALTER TABLE interview_question_profiles DROP COLUMN seed_revision, DROP COLUMN subtopic, DROP COLUMN topic, DROP CONSTRAINT interview_question_levels, DROP COLUMN level_max, DROP COLUMN level_min, DROP COLUMN followup_weight;
DROP INDEX interview_question_bank_seed_key;
UPDATE interview_question_profiles SET seed_key=legacy_seed_key WHERE legacy_seed_key IS NOT NULL;
ALTER TABLE interview_question_profiles DROP COLUMN legacy_seed_key, DROP COLUMN owner_id;
