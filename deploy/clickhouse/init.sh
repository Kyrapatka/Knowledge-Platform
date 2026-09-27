#!/bin/bash
set -euo pipefail
client=(clickhouse-client --host clickhouse --user analytics_admin --password "$CLICKHOUSE_PASSWORD")
for migration in /schema/[0-9]*.sql; do
  "${client[@]}" --multiquery < "$migration"
done
# Hashes are fixed-length hex: passwords never become executable SQL.
writer_hash=$(printf %s "$WRITER_PASSWORD" | sha256sum | cut -d' ' -f1)
reader_hash=$(printf %s "$READER_PASSWORD" | sha256sum | cut -d' ' -f1)
"${client[@]}" --multiquery <<SQL
CREATE USER IF NOT EXISTS analytics_writer IDENTIFIED WITH sha256_hash BY '$writer_hash';
ALTER USER analytics_writer IDENTIFIED WITH sha256_hash BY '$writer_hash';
GRANT INSERT ON knowledge_analytics.analytics_events TO analytics_writer;
CREATE USER IF NOT EXISTS grafana_reader IDENTIFIED WITH sha256_hash BY '$reader_hash';
ALTER USER grafana_reader IDENTIFIED WITH sha256_hash BY '$reader_hash' SETTINGS readonly = 1, join_use_nulls = 1, max_execution_time = 30 CHANGEABLE_IN_READONLY;
GRANT SELECT ON knowledge_analytics.* TO grafana_reader;
SQL
