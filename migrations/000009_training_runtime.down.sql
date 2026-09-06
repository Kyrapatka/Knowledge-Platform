DROP TABLE training_commands;
DROP TABLE training_events;
DROP TABLE training_session_items;
DROP TABLE training_sessions;
ALTER TABLE folders DROP COLUMN training_config, DROP COLUMN training_config_version;
