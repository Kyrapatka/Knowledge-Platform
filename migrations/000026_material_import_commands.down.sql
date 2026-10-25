DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM material_import_commands) THEN RAISE EXCEPTION 'Refusing to remove import receipts: retries could duplicate materials'; END IF;
END $$;
DROP TABLE material_import_commands;
