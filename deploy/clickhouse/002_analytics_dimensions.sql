-- Additive migration: old rows remain unknown, not fabricated historical values.
ALTER TABLE knowledge_analytics.analytics_events ADD COLUMN IF NOT EXISTS plan_id String DEFAULT '';
ALTER TABLE knowledge_analytics.analytics_events ADD COLUMN IF NOT EXISTS interview_mode LowCardinality(String) DEFAULT '';
ALTER TABLE knowledge_analytics.analytics_events ADD COLUMN IF NOT EXISTS interview_depth Nullable(Int32);
