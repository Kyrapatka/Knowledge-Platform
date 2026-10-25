DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM mock_presets) THEN RAISE EXCEPTION 'Refusing to remove saved Mock presets'; END IF;
END $$;
DROP TABLE mock_presets;
