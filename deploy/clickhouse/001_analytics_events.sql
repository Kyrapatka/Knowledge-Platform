-- Apply explicitly with an administrative account. The API never creates schema.
-- Runtime user needs INSERT only; Grafana uses a separate SELECT-only account.
CREATE DATABASE IF NOT EXISTS knowledge_analytics;
CREATE TABLE IF NOT EXISTS knowledge_analytics.analytics_events
(
    event_id UUID,
    event_name LowCardinality(String),
    occurred_at DateTime64(3, 'UTC'),
    user_id String,
    session_id String,
    app_version LowCardinality(String),
    algorithm_version LowCardinality(String),
    experiment_group LowCardinality(String),
    folder_id String,
    material_id String,
    template LowCardinality(String),
    topic Nullable(String),
    subtopic Nullable(String),
    difficulty LowCardinality(Nullable(String)),
    mode LowCardinality(String),
    result LowCardinality(String),
    review_kind LowCardinality(String),
    answer_time_ms Nullable(Int64),
    stage_before Nullable(Int32),
    stage_after Nullable(Int32),
    consecutive_correct_before Nullable(Int32),
    consecutive_correct_after Nullable(Int32),
    wrong_count_before Nullable(Int32),
    wrong_count_after Nullable(Int32),
    rehab_active_before Nullable(Bool),
    rehab_active_after Nullable(Bool),
    retrievability_before Nullable(Float64),
    retrievability_after Nullable(Float64),
    stability_before Nullable(Float64),
    stability_after Nullable(Float64),
    next_review_before Nullable(DateTime64(3, 'UTC')),
    next_review_after Nullable(DateTime64(3, 'UTC')),
    learned_before Nullable(Bool),
    learned_after Nullable(Bool),
    review_credit Nullable(Bool),
    related_event_id String
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(occurred_at)
ORDER BY (event_name, toDate(occurred_at), template, user_id, occurred_at, event_id)
TTL toDateTime(occurred_at) + INTERVAL 365 DAY DELETE;
