package postgres

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
)

const topicExpression = "COALESCE(NULLIF(BTRIM(m.metadata->>'topic'),''), NULLIF(BTRIM(m.metadata->>'category'),''), '')"

func selectedMaterials(query *gorm.DB, sources []model.SessionSource) *gorm.DB {
	if len(sources) == 0 {
		return query
	}
	var alternatives []string
	var args []any
	for _, source := range sources {
		if len(source.Topics) == 0 {
			alternatives = append(alternatives, "m.folder_id=?")
			args = append(args, source.FolderID)
			continue
		}
		topics := make([]string, len(source.Topics))
		for i, topic := range source.Topics {
			if topic != "__none__" {
				topics[i] = topic
			}
		}
		alternatives = append(alternatives, "(m.folder_id=? AND "+topicExpression+" IN ?)")
		args = append(args, source.FolderID, topics)
	}
	return query.Where("("+strings.Join(alternatives, " OR ")+")", args...)
}

func (t *runtimeTx) LatestSession(plan uuid.UUID) (model.TrainingSession, error) {
	var session model.TrainingSession
	if plan == uuid.Nil {
		err := t.db.Table("training_sessions").Where("user_id=? AND plan_id IS NULL", t.user).Order("created_at DESC,id").Take(&session).Error
		return session, translate(err)
	}
	err := t.db.Table("training_sessions").Where("user_id=? AND plan_id=?", t.user, plan).
		Order("created_at DESC, id").Take(&session).Error
	return session, translate(err)
}

func (t *runtimeTx) Availability(plan model.TrainingPlan, selection []model.SessionSource) (model.Availability, error) {
	var result model.Availability
	if _, err := t.Plan(plan.ID); err != nil {
		return result, err
	}
	var alternatives []string
	var args []any
	for _, id := range plan.SourceFolderIDs {
		config, ok := plan.Config.Cards[id.String()]
		if !ok {
			continue
		}
		alternatives = append(alternatives, `(m.folder_id=? AND EXISTS (SELECT 1 FROM jsonb_each_text(m.values) v WHERE v.key IN ? AND NULLIF(btrim(v.value),'') IS NOT NULL) AND EXISTS (SELECT 1 FROM jsonb_each_text(m.values) v WHERE v.key IN ? AND NULLIF(btrim(v.value),'') IS NOT NULL))`)
		args = append(args, id, config.Card.QuestionFields, config.Card.AnswerFields)
	}
	if len(alternatives) == 0 {
		return result, nil
	}
	join := "LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=? AND p.plan_id IS NULL"
	joinArgs := []any{t.user, plan.Track}
	if plan.Track == model.ProgressTrackCram {
		join = "LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=? AND p.plan_id=?"
		joinArgs = append(joinArgs, plan.ID)
	}
	query := t.db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").Joins(join, joinArgs...).
		Where("f.owner_id=? AND m.deleted_at IS NULL AND f.deleted_at IS NULL", t.user).
		Where("("+strings.Join(alternatives, " OR ")+")", args...)
	if plan.AlgorithmKey == "formula_adaptive" {
		query = query.Where("EXISTS (SELECT 1 FROM formula_exercises e WHERE e.material_id=m.id)")
	}
	query = selectedMaterials(query, selection)
	err := query.Select("COUNT(*) AS total, COUNT(*) FILTER (WHERE p.completed_at IS NULL) AS pending, MIN(p.next_review_at) AS next_review_at").Scan(&result).Error
	return result, err
}

func (t *runtimeTx) CombinedSummary(sessions []uuid.UUID) (model.SessionSummary, error) {
	var result model.SessionSummary
	if len(sessions) == 0 {
		return result, nil
	}
	err := t.db.Table("training_events").Where("user_id=? AND session_id IN ? AND undone_at IS NULL", t.user, sessions).Select(`
		COUNT(*) FILTER (WHERE action='correct') AS correct,
		COUNT(*) FILTER (WHERE action='wrong') AS wrong,
		COUNT(*) FILTER (WHERE action='advance') AS advance,
		COUNT(*) FILTER (WHERE action='rollback') AS rollback,
		COUNT(*) FILTER (WHERE action='skip_rehab') AS skip_rehab,
		COUNT(DISTINCT material_id) FILTER (WHERE action IN ('correct','wrong')) AS materials_reviewed,
		COUNT(*) FILTER (WHERE action='correct' AND stage_after>stage_before) AS stage_promotions`).Scan(&result).Error
	return result, err
}
