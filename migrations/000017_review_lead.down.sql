-- Existing dates cannot safely be reconstructed after subsequent answers.
-- Intentionally preserve review dates when rolling back this policy migration.
SELECT 1;
