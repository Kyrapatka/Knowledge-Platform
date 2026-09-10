package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Windows installations may not provide an IANA zone database.

	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("library item not found")
var ErrInvalid = errors.New("invalid library query")

type Store struct {
	db  *gorm.DB
	now func() time.Time
}

func NewStore(db *gorm.DB) *Store { return &Store{db: db, now: time.Now} }

// A nonempty explicit topic takes precedence over legacy category. Matching is
// exact after trimming; no vocabulary is silently merged by case folding.
const topicSQL = `COALESCE(NULLIF(BTRIM(m.metadata->>'topic'),''),NULLIF(BTRIM(m.metadata->>'category'),''),'')`

const planFieldsSQL = `p.id,p.algorithm_key,p.algorithm_version,p.track,p.status,
 COALESCE((p.config->>'horizon_days')::integer,0) AS horizon_days,
 COALESCE((p.config->>'pool_size')::integer,0) AS pool_size,p.created_at`

const chosenPlanJSON = `CASE WHEN cp.id IS NULL THEN NULL ELSE jsonb_build_object(
 'id',cp.id,'algorithm_key',cp.algorithm_key,'algorithm_version',cp.algorithm_version,
 'track',cp.track,'status',cp.status,'horizon_days',COALESCE((cp.config->>'horizon_days')::integer,0),
 'pool_size',COALESCE((cp.config->>'pool_size')::integer,0),'created_at',cp.created_at) END`

func (s *Store) Library(ctx context.Context, user uuid.UUID) (LibraryView, error) {
	out := LibraryView{Folders: []FolderSummary{}}
	if user == uuid.Nil {
		return out, ErrNotFound
	}
	// Folder summaries and topic counts use one query, independent of the number
	// of folders. The same chosen plan policy is used by FolderMaterials below.
	query := `WITH owned AS (
 SELECT * FROM folders WHERE owner_id=? AND deleted_at IS NULL
), chosen AS (
 SELECT DISTINCT ON (ps.folder_id) ps.folder_id,p.*
 FROM training_plan_sources ps JOIN training_plans p ON p.id=ps.plan_id
 JOIN owned f ON f.id=ps.folder_id
 WHERE p.user_id=? AND p.status <> 'cancelled'
 ORDER BY ps.folder_id,(p.status='active') DESC,(p.algorithm_key=f.training_config->>'default_algorithm_key') DESC,p.created_at DESC,p.id
), live AS (
 SELECT m.id,m.folder_id,` + topicSQL + ` AS topic,p.version,p.completed_at,p.next_review_at,
 cp.status AS plan_status,
 (f.template_key <> 'formulas' OR EXISTS (SELECT 1 FROM formula_exercises e WHERE e.material_id=m.id)) AS trainable
 FROM materials m JOIN owned f ON f.id=m.folder_id
 LEFT JOIN chosen cp ON cp.folder_id=m.folder_id
 LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=cp.track
 AND ((cp.track='cram' AND p.plan_id=cp.id) OR (cp.track<>'cram' AND p.plan_id IS NULL))
 WHERE m.deleted_at IS NULL
), counts AS (
 SELECT folder_id,COUNT(*) AS material_count,
 COUNT(*) FILTER (WHERE version IS NOT NULL AND completed_at IS NULL) AS learning_count,
 COUNT(*) FILTER (WHERE completed_at IS NOT NULL) AS completed_count,
 COUNT(*) FILTER (WHERE completed_at IS NULL AND next_review_at<=? AND plan_status='active' AND trainable) AS due_count
 FROM live GROUP BY folder_id
), topic_counts AS (
 SELECT folder_id,topic,COUNT(*) AS count FROM live GROUP BY folder_id,topic
), topics AS (
 SELECT folder_id,jsonb_agg(jsonb_build_object('name',topic,'count',count) ORDER BY topic) AS items
 FROM topic_counts GROUP BY folder_id
)
SELECT to_jsonb(f) || jsonb_build_object(
 'material_count',COALESCE(c.material_count,0),'learning_count',COALESCE(c.learning_count,0),
 'completed_count',COALESCE(c.completed_count,0),'due_count',COALESCE(c.due_count,0),
 'topics',COALESCE(t.items,'[]'::jsonb),'selected_plan',` + chosenPlanJSON + `) AS payload
FROM owned f LEFT JOIN counts c ON c.folder_id=f.id LEFT JOIN topics t ON t.folder_id=f.id
LEFT JOIN chosen cp ON cp.folder_id=f.id ORDER BY f.created_at DESC,f.id`
	var rows []struct{ Payload []byte }
	if err := s.db.WithContext(ctx).Raw(query, user, user, user, s.now().UTC()).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		var folder FolderSummary
		if err := json.Unmarshal(row.Payload, &folder); err != nil {
			return out, err
		}
		out.Folders = append(out.Folders, folder)
		out.Totals.FolderCount++
		out.Totals.MaterialCount += folder.MaterialCount
		out.Totals.DueCount += folder.DueCount
		out.Totals.LearningCount += folder.LearningCount
		out.Totals.CompletedCount += folder.CompletedCount
	}
	return out, nil
}

type MaterialsQuery struct {
	PlanID    *uuid.UUID
	Topic     string
	Search    string
	Sort      string
	Direction string
	Limit     int
	Offset    int
}

func (q MaterialsQuery) validate() error {
	if q.Limit < 1 || q.Limit > 200 || q.Offset < 0 || q.Offset > 1000000 || len(q.Search) > 500 || len(q.Topic) > 500 {
		return ErrInvalid
	}
	if q.PlanID != nil && *q.PlanID == uuid.Nil {
		return ErrInvalid
	}
	if q.Direction != "asc" && q.Direction != "desc" {
		return ErrInvalid
	}
	switch q.Sort {
	case "created_at", "question", "topic", "stage", "next_review_at":
		return nil
	default:
		return ErrInvalid
	}
}

func (s *Store) FolderMaterials(ctx context.Context, user, folder uuid.UUID, q MaterialsQuery) (MaterialsView, error) {
	out := MaterialsView{Items: []MaterialView{}, Topics: []Topic{}, Plans: []PlanOption{}, Limit: q.Limit, Offset: q.Offset}
	if err := q.validate(); err != nil {
		return out, err
	}
	if user == uuid.Nil || folder == uuid.Nil {
		return out, ErrNotFound
	}
	// Pagination, totals and progress context form one coherent read even if a
	// material is edited or deleted while the request is running.
	err := s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var n int64
		if err := db.Table("folders").Where("id=? AND owner_id=? AND deleted_at IS NULL", folder, user).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		if err := db.Raw(`SELECT `+planFieldsSQL+` FROM training_plans p
 JOIN training_plan_sources ps ON ps.plan_id=p.id
 JOIN folders selected_folder ON selected_folder.id=ps.folder_id
 WHERE ps.folder_id=? AND p.user_id=?
 ORDER BY (p.status='active') DESC,(p.status='completed') DESC,(p.algorithm_key=selected_folder.training_config->>'default_algorithm_key') DESC,p.created_at DESC,p.id`, folder, user).Scan(&out.Plans).Error; err != nil {
			return err
		}
		for i := range out.Plans {
			p := &out.Plans[i]
			if (q.PlanID != nil && p.ID == *q.PlanID) || (q.PlanID == nil && p.Status != "cancelled") {
				out.SelectedPlan = p
				break
			}
		}
		if q.PlanID != nil && out.SelectedPlan == nil {
			return ErrNotFound
		}
		if err := db.Raw(`SELECT `+topicSQL+` AS name,COUNT(*) AS count
 FROM materials m WHERE m.folder_id=? AND m.deleted_at IS NULL
 GROUP BY `+topicSQL+` ORDER BY name`, folder).Scan(&out.Topics).Error; err != nil {
			return err
		}

		var planID uuid.UUID
		track := ""
		if out.SelectedPlan != nil {
			planID, track = out.SelectedPlan.ID, out.SelectedPlan.Track
		}
		from := ` FROM materials m JOIN folders f ON f.id=m.folder_id
 LEFT JOIN user_material_progress p ON p.user_id=? AND p.material_id=m.id AND p.track=?
 AND ((p.track='cram' AND p.plan_id=?) OR (p.track<>'cram' AND p.plan_id IS NULL))
 WHERE m.folder_id=? AND f.owner_id=? AND f.deleted_at IS NULL AND m.deleted_at IS NULL`
		args := []any{user, track, planID, folder, user}
		if q.Topic != "" {
			topic := strings.TrimSpace(q.Topic)
			if topic == "__none__" {
				topic = ""
			}
			from += " AND " + topicSQL + " = ?"
			args = append(args, topic)
		}
		if search := strings.TrimSpace(q.Search); search != "" {
			// Match values, not JSON keys or serialization escapes. POSITION keeps
			// user-entered % and _ literal instead of treating them as wildcards.
			from += ` AND (EXISTS (SELECT 1 FROM jsonb_each_text(m.values) v WHERE POSITION(LOWER(?) IN LOWER(v.value))>0)
 OR POSITION(LOWER(?) IN LOWER(` + topicSQL + `))>0)`
			args = append(args, search, search)
		}
		if err := db.Raw("SELECT COUNT(*)"+from, args...).Scan(&out.Total).Error; err != nil {
			return err
		}
		sortColumn := map[string]string{
			"created_at": "m.created_at", "topic": topicSQL, "stage": "p.stage", "next_review_at": "p.next_review_at",
			"question": `LOWER(COALESCE(m.values->>(f.config #>> '{card,question_fields,0}'),m.values->>'question',m.values->>'foreign',m.values->>'name',''))`,
		}[q.Sort]
		query := `SELECT to_jsonb(m) || jsonb_build_object('topic',` + topicSQL + `,'progress',to_jsonb(p)) AS payload` + from +
			" ORDER BY " + sortColumn + " " + q.Direction + " NULLS LAST,m.id LIMIT ? OFFSET ?"
		var rows []struct{ Payload []byte }
		pageArgs := append(append([]any{}, args...), q.Limit, q.Offset)
		if err := db.Raw(query, pageArgs...).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var m MaterialView
			if err := json.Unmarshal(row.Payload, &m); err != nil {
				return err
			}
			if m.Progress != nil && out.SelectedPlan != nil {
				m.Progress.CanStartFinal = canStartFinal(*m.Progress, *out.SelectedPlan)
			}
			out.Items = append(out.Items, m)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return out, err
}

func canStartFinal(p Progress, plan PlanOption) bool {
	if plan.Status != "active" || p.CompletedAt != nil || p.AlgorithmVersion != 1 ||
		(p.AlgorithmKey != "interview_cram" && p.AlgorithmKey != "interview_long_term") {
		return false
	}
	a := algorithm.Interview{Cram: p.AlgorithmKey == "interview_cram"}
	schedule, err := a.Schedule(model.UserMaterialProgress{LearningStartedAt: p.LearningStartedAt, TargetAt: p.TargetAt})
	return err == nil && p.Stage == len(schedule)
}

func (s *Store) Statistics(ctx context.Context, user uuid.UUID, days int, timezone string) (StatisticsView, error) {
	out := StatisticsView{Days: days, Timezone: timezone, Daily: []DailyActivity{}}
	if user == uuid.Nil {
		return out, ErrNotFound
	}
	if days < 1 || days > 366 || timezone == "" || len(timezone) > 100 || timezone == "Local" {
		return out, ErrInvalid
	}
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		return out, fmt.Errorf("%w: unsupported timezone", ErrInvalid)
	}
	now := s.now()
	local := now.In(zone)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone).AddDate(0, 0, 1-days)
	out.From, out.To = start.UTC(), now.UTC()
	// Only actual answers contribute to activity. Administrative events such as
	// Skip Rehab, Start Final, or manual stage changes are not learning attempts.
	const fields = `COUNT(*) AS answers,
 COUNT(*) FILTER (WHERE action='correct') AS correct,
 COUNT(*) FILTER (WHERE action='wrong') AS wrong,
 COUNT(DISTINCT material_id) AS materials_reviewed,
 COUNT(DISTINCT session_id) AS sessions,
 COUNT(*) FILTER (WHERE action='correct' AND stage_after>stage_before) AS stage_promotions`
	const where = ` FROM training_events WHERE user_id=? AND action IN ('correct','wrong') AND created_at>=? AND created_at<=?`
	err = s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		if err := db.Raw(`SELECT `+fields+`,COUNT(DISTINCT timezone(?,created_at)::date) AS active_days`+where,
			timezone, user, out.From, out.To).Scan(&out.Totals).Error; err != nil {
			return err
		}
		var rows []DailyActivity
		if err := db.Raw(`SELECT to_char(timezone(?,created_at),'YYYY-MM-DD') AS date,`+fields+where+` GROUP BY date ORDER BY date`,
			timezone, user, out.From, out.To).Scan(&rows).Error; err != nil {
			return err
		}
		byDay := make(map[string]DailyActivity, len(rows))
		for _, day := range rows {
			byDay[day.Date] = day
		}
		for d := 0; d < days; d++ {
			date := start.AddDate(0, 0, d).Format("2006-01-02")
			day, found := byDay[date]
			if !found {
				day.Date = date
			}
			out.Daily = append(out.Daily, day)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return out, err
}
