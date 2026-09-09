package postgres

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"strings"
	"time"
)

// Empty sources stay open. A finite plan finishes only when every currently
// trainable source material has completed its own final review.
func (t *runtimeTx) CompletePlanIfReady(p model.TrainingPlan, now time.Time) (bool, error) {
	if p.Track == model.ProgressTrackDefault {
		return false, nil
	}
	if _, err := t.Plan(p.ID); err != nil {
		return false, err
	}
	var alternatives []string
	var args []any
	for _, id := range p.SourceFolderIDs {
		c, ok := p.Config.Cards[id.String()]
		if !ok {
			continue
		}
		alternatives = append(alternatives, `(m.folder_id=? AND EXISTS (SELECT 1 FROM jsonb_each_text(m.values) v WHERE v.key IN ? AND NULLIF(btrim(v.value),'') IS NOT NULL) AND EXISTS (SELECT 1 FROM jsonb_each_text(m.values) v WHERE v.key IN ? AND NULLIF(btrim(v.value),'') IS NOT NULL))`)
		args = append(args, id, c.Card.QuestionFields, c.Card.AnswerFields)
	}
	if len(alternatives) == 0 {
		return false, nil
	}
	join := "LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=? AND p.plan_id IS NULL"
	joinArgs := []any{t.user, p.Track}
	if p.Track == model.ProgressTrackCram {
		join = "LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=? AND p.plan_id=?"
		joinArgs = append(joinArgs, p.ID)
	}
	var counts struct{ Total, Pending int64 }
	err := t.db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").Joins(join, joinArgs...).Where("f.owner_id=? AND m.deleted_at IS NULL AND f.deleted_at IS NULL", t.user).
		Where("("+strings.Join(alternatives, " OR ")+")", args...).Select("COUNT(*) AS total, COUNT(*) FILTER (WHERE p.completed_at IS NULL) AS pending").Scan(&counts).Error
	if err != nil || counts.Total == 0 || counts.Pending != 0 {
		return false, err
	}
	if err = t.db.Table("training_sessions").Where("plan_id=? AND user_id=? AND status='active'", p.ID, t.user).Updates(map[string]any{"status": model.StatusCompleted, "finished_at": now}).Error; err != nil {
		return false, err
	}
	err = t.db.Table("training_plans").Where("id=? AND user_id=? AND status='active'", p.ID, t.user).Updates(map[string]any{"status": model.StatusCompleted, "updated_at": now}).Error
	return err == nil, err
}
