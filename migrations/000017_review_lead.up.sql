-- One-time adjustment for dates scheduled before the 30-minute lead policy.
-- Due reviews stay due; material targets and actual review anchors are unchanged.
UPDATE user_material_progress
SET stage_review_at = CASE WHEN stage_review_at > NOW() THEN GREATEST(NOW(), stage_review_at - INTERVAL '30 minutes') ELSE stage_review_at END,
    rehab_review_at = CASE WHEN rehab_review_at > NOW() THEN GREATEST(NOW(), rehab_review_at - INTERVAL '30 minutes') ELSE rehab_review_at END,
    extra_review_at = CASE WHEN extra_review_at > NOW() THEN GREATEST(NOW(), extra_review_at - INTERVAL '30 minutes') ELSE extra_review_at END,
    version = version + 1,
    updated_at = NOW()
WHERE completed_at IS NULL AND (stage_review_at > NOW() OR rehab_review_at > NOW() OR extra_review_at > NOW());
