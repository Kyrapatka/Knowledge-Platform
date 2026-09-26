package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	folderpg "github.com/Kyrapatka/knowledge-platform/internal/core/folder/repository/postgres"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialpg "github.com/Kyrapatka/knowledge-platform/internal/core/material/repository/postgres"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RuntimeStore struct{ db *gorm.DB }

func NewRuntimeStore(db *gorm.DB) *RuntimeStore { return &RuntimeStore{db} }

type runtimeTx struct {
	db   *gorm.DB
	ctx  context.Context
	user uuid.UUID
}

func (s *RuntimeStore) Transact(ctx context.Context, user uuid.UUID, fn func(repository.Tx) error) error {
	if user == uuid.Nil {
		return repository.ErrNotFound
	}
	return translate(s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		// A local row lock also protects empty-folder plan overlap and creation
		// of absent progress rows. All training commands acquire it first.
		var identity struct{ ID uuid.UUID }
		if err := db.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", user).Take(&identity).Error; err != nil {
			return translate(err)
		}
		return fn(&runtimeTx{db, ctx, user})
	}))
}

func (t *runtimeTx) Folder(id uuid.UUID) (folder.Folder, error) {
	var count int64
	if err := t.db.Table("folders").Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, t.user).Count(&count).Error; err != nil {
		return folder.Folder{}, err
	}
	if count == 0 {
		return folder.Folder{}, repository.ErrNotFound
	}
	return folderpg.NewRepository(t.db).GetByID(t.ctx, id)
}

func (t *runtimeTx) SaveDefaults(id uuid.UUID, c folderconfig.TrainingConfig, version int64, now time.Time) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	r := t.db.Table("folders").Where("id = ? AND owner_id = ? AND training_config_version = ?", id, t.user, version).
		Updates(map[string]any{"training_config": b, "training_config_version": version + 1, "updated_at": now})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return repository.ErrConflict
	}
	return nil
}

func (t *runtimeTx) HasPlanOverlap(ids []uuid.UUID, track model.ProgressTrack) (bool, error) {
	var n int64
	err := t.db.Table("training_plans p").Joins("JOIN training_plan_sources s ON s.plan_id=p.id").
		Where("p.user_id=? AND p.track=? AND p.status='active' AND s.folder_id IN ?", t.user, track, ids).Count(&n).Error
	return n > 0, err
}

func (t *runtimeTx) HasIncompatibleProgress(ids []uuid.UUID, track model.ProgressTrack, key string, version int) (bool, error) {
	var n int64
	err := t.db.Table("user_material_progress p").Joins("JOIN materials m ON m.id=p.material_id").
		Where("p.user_id=? AND p.track=? AND p.plan_id IS NULL AND m.deleted_at IS NULL AND m.folder_id IN ? AND (p.algorithm_key<>? OR p.algorithm_version<>?)", t.user, track, ids, key, version).Count(&n).Error
	return n > 0, err
}

type planRow struct {
	Version          int
	ID               uuid.UUID
	UserID           uuid.UUID
	Track            model.ProgressTrack
	AlgorithmKey     string
	AlgorithmVersion int
	Status           model.Status
	Config           []byte
	StartedAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (t *runtimeTx) CreatePlan(p model.TrainingPlan) error {
	if p.UserID != t.user {
		return repository.ErrNotFound
	}
	b, err := json.Marshal(p.Config)
	if err != nil {
		return err
	}
	r := planRow{p.Version, p.ID, p.UserID, p.Track, p.AlgorithmKey, p.AlgorithmVersion, p.Status, b, p.StartedAt, p.CreatedAt, p.UpdatedAt}
	if err = t.db.Table("training_plans").Create(&r).Error; err != nil {
		return err
	}
	for _, id := range p.SourceFolderIDs {
		if _, err = t.Folder(id); err != nil {
			return err
		}
		if err = t.db.Table("training_plan_sources").Create(map[string]any{"plan_id": p.ID, "folder_id": id}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (t *runtimeTx) decodePlan(r planRow) (model.TrainingPlan, error) {
	p := model.TrainingPlan{Version: r.Version, ID: r.ID, UserID: r.UserID, Track: r.Track, AlgorithmKey: r.AlgorithmKey, AlgorithmVersion: r.AlgorithmVersion, Status: r.Status, StartedAt: r.StartedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if err := json.Unmarshal(r.Config, &p.Config); err != nil {
		return p, err
	}
	err := t.db.Table("training_plan_sources").Where("plan_id=?", p.ID).Order("folder_id").Pluck("folder_id", &p.SourceFolderIDs).Error
	return p, err
}

func (t *runtimeTx) Plan(id uuid.UUID) (model.TrainingPlan, error) {
	var r planRow
	if err := t.db.Table("training_plans").Where("id=? AND user_id=?", id, t.user).Take(&r).Error; err != nil {
		return model.TrainingPlan{}, translate(err)
	}
	return t.decodePlan(r)
}

func (t *runtimeTx) Plans(limit, offset int) ([]model.TrainingPlan, error) {
	var rows []planRow
	if err := t.db.Table("training_plans").Where("user_id=?", t.user).Order("created_at DESC, id").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]model.TrainingPlan, 0, len(rows))
	for _, r := range rows {
		p, err := t.decodePlan(r)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}

func (t *runtimeTx) CancelPlan(id uuid.UUID, now time.Time) error {
	if _, err := t.Plan(id); err != nil {
		return err
	}
	if err := t.db.Table("training_sessions").Where("plan_id=? AND user_id=? AND status='active'", id, t.user).Updates(map[string]any{"status": model.StatusCancelled, "finished_at": now}).Error; err != nil {
		return err
	}
	return t.db.Table("training_plans").Where("id=? AND user_id=?", id, t.user).Updates(map[string]any{"status": model.StatusCancelled, "updated_at": now}).Error
}

func (t *runtimeTx) Session(id uuid.UUID) (model.TrainingSession, error) {
	var s model.TrainingSession
	err := t.db.Table("training_sessions").Where("id=? AND user_id=?", id, t.user).Take(&s).Error
	return s, translate(err)
}
func (t *runtimeTx) ActiveSession(plan uuid.UUID) (model.TrainingSession, error) {
	var s model.TrainingSession
	if plan == uuid.Nil {
		err := t.db.Table("training_sessions").Where("plan_id IS NULL AND user_id=? AND status='active'", t.user).Take(&s).Error
		return s, translate(err)
	}
	err := t.db.Table("training_sessions").Where("plan_id=? AND user_id=? AND status='active'", plan, t.user).Take(&s).Error
	return s, translate(err)
}
func (t *runtimeTx) CreateSession(s model.TrainingSession) error {
	if s.SelectionStrategy == "" {
		s.SelectionStrategy = model.SelectionRandom
	}
	if s.Selection == nil {
		s.Selection = []model.SessionSource{}
	}
	if s.UserID != t.user {
		return repository.ErrNotFound
	}
	if s.PlanID == uuid.Nil {
		if s.SelectionStrategy != model.SelectionInterviewGraphV1 || s.Combined {
			return repository.ErrConflict
		}
		return t.db.Table("training_sessions").Omit("PlanID").Create(&s).Error
	}
	if _, err := t.Plan(s.PlanID); err != nil {
		return err
	}
	return t.db.Table("training_sessions").Create(&s).Error
}
func (t *runtimeTx) FinishSession(id uuid.UUID, status model.Status, now time.Time) error {
	r := t.db.Table("training_sessions").Where("id=? AND user_id=? AND status='active'", id, t.user).Updates(map[string]any{"status": status, "finished_at": now})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return repository.ErrConflict
	}
	return nil
}

type itemRow struct {
	SessionID    uuid.UUID
	MaterialID   uuid.UUID
	Position     int64
	State        string
	Presentation []byte
}

func (t *runtimeTx) Items(session uuid.UUID) ([]model.SessionItem, error) {
	if _, err := t.Session(session); err != nil {
		return nil, err
	}
	var rows []itemRow
	if err := t.db.Table("training_session_items").Where("session_id=? AND state='active'", session).Order("position, material_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]model.SessionItem, 0, len(rows))
	for _, r := range rows {
		i := model.SessionItem{SessionID: r.SessionID, MaterialID: r.MaterialID, Position: r.Position, State: r.State}
		if len(r.Presentation) > 0 {
			if err := json.Unmarshal(r.Presentation, &i.Presentation); err != nil {
				return nil, err
			}
		}
		items = append(items, i)
	}
	return items, nil
}

func (t *runtimeTx) SaveItem(i model.SessionItem) error {
	if _, err := t.Session(i.SessionID); err != nil {
		return err
	}
	var b []byte
	if i.Presentation != nil {
		var err error
		b, err = json.Marshal(i.Presentation)
		if err != nil {
			return err
		}
	}
	r := itemRow{i.SessionID, i.MaterialID, i.Position, i.State, b}
	return t.db.Table("training_session_items").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}, {Name: "material_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"position", "state", "presentation"}),
	}).Create(&r).Error
}

func (t *runtimeTx) Material(id uuid.UUID) (material.Material, error) {
	var n int64
	err := t.db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").Where("m.id=? AND f.owner_id=? AND m.deleted_at IS NULL AND f.deleted_at IS NULL", id, t.user).Count(&n).Error
	if err != nil {
		return material.Material{}, err
	}
	if n == 0 {
		return material.Material{}, repository.ErrNotFound
	}
	return materialpg.NewRepository(t.db).GetByID(t.ctx, id)
}

func (t *runtimeTx) Candidates(p model.TrainingPlan, session uuid.UUID, now time.Time, limit int) ([]material.Material, error) {
	if p.UserID != t.user || limit <= 0 {
		return nil, fmt.Errorf("invalid candidate query")
	}
	// Filter empty cards in SQL, so an untrainable prefix cannot starve the pool.
	var alternatives []string
	var args []any
	for _, id := range p.SourceFolderIDs {
		cfg, ok := p.Config.Cards[id.String()]
		if !ok {
			continue
		}
		alternatives = append(alternatives, `(m.folder_id=? AND EXISTS (SELECT 1 FROM jsonb_each_text(m.values) v WHERE v.key IN ? AND NULLIF(btrim(v.value),'') IS NOT NULL) AND EXISTS (SELECT 1 FROM jsonb_each_text(m.values) v WHERE v.key IN ? AND NULLIF(btrim(v.value),'') IS NOT NULL))`)
		args = append(args, id, cfg.Card.QuestionFields, cfg.Card.AnswerFields)
	}
	if len(alternatives) == 0 {
		return []material.Material{}, nil
	}
	var ids []uuid.UUID
	join := "LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=? AND p.plan_id IS NULL"
	joinArgs := []any{t.user, p.Track}
	if p.Track == model.ProgressTrackCram {
		join = "LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=? AND p.plan_id=?"
		joinArgs = append(joinArgs, p.ID)
	}
	order := "p.next_review_at ASC NULLS LAST, m.created_at, m.id"
	if p.Track != model.ProgressTrackDefault || p.AlgorithmKey == "formula_adaptive" {
		order = "random()"
	}
	// Normal recall eligibility is content + SRS state, not interview editorial
	// status. Keep this independent of the graph selector (including draft policy).
	query := t.db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").
		Joins(join, joinArgs...).
		Where("f.owner_id=? AND m.deleted_at IS NULL AND f.deleted_at IS NULL", t.user).Where("("+strings.Join(alternatives, " OR ")+")", args...).
		Where("(p.material_id IS NULL OR (p.algorithm_key=? AND p.algorithm_version=? AND p.next_review_at<=?))", p.AlgorithmKey, p.AlgorithmVersion, now).
		Where("NOT EXISTS (SELECT 1 FROM training_session_items i WHERE i.session_id=? AND i.material_id=m.id AND i.state='active')", session)
	if p.AlgorithmKey == "formula_adaptive" {
		query = query.Where("EXISTS (SELECT 1 FROM formula_exercises e WHERE e.material_id=m.id)")
	}
	scope, err := t.Session(session)
	if err != nil {
		return nil, err
	}
	query = selectedMaterials(query, scope.Selection)
	err = query.Order(order).Limit(limit).Pluck("m.id", &ids).Error
	if err != nil {
		return nil, err
	}
	materials := make([]material.Material, 0, len(ids))
	for _, id := range ids {
		m, err := t.Material(id)
		if err != nil {
			return nil, err
		}
		materials = append(materials, m)
	}
	return materials, nil
}

func (t *runtimeTx) Progress() repository.ProgressRepository { return NewProgressRepository(t.db) }
func (t *runtimeTx) Receipt(id uuid.UUID) (model.CommandReceipt, error) {
	var r model.CommandReceipt
	err := t.db.Table("training_commands").Where("user_id=? AND command_id=?", t.user, id).Take(&r).Error
	return r, translate(err)
}
func (t *runtimeTx) SaveReceipt(r model.CommandReceipt) error {
	if r.UserID != t.user {
		return repository.ErrNotFound
	}
	return t.db.Table("training_commands").Create(&r).Error
}
func (t *runtimeTx) SaveEvent(e model.TrainingEvent) error {
	if e.EventMode == "" {
		e.EventMode = "scheduled"
		e.ReviewCredit = true
	}
	if e.UserID != t.user {
		return repository.ErrNotFound
	}
	if e.Action != "correct" && e.Action != "wrong" && e.Action != "next_route" {
		if err := t.db.Exec("DELETE FROM training_undo WHERE user_id=?", t.user).Error; err != nil {
			return err
		}
	}
	if e.PlanID == uuid.Nil {
		return t.db.Table("training_events").Omit("PlanID").Create(&e).Error
	}
	return t.db.Table("training_events").Create(&e).Error
}
func (t *runtimeTx) Summary(session uuid.UUID) (model.SessionSummary, error) {
	var s model.SessionSummary
	if _, err := t.Session(session); err != nil {
		return s, err
	}
	err := t.db.Table("training_events").Where("user_id=? AND session_id=? AND undone_at IS NULL", t.user, session).Select(`
		COUNT(*) FILTER (WHERE action IN ('correct','wrong') AND review_credit) AS scheduled_reviews,
		COUNT(*) FILTER (WHERE action IN ('correct','wrong') AND NOT review_credit) AS interview_probes,
		COUNT(*) FILTER (WHERE action='correct') AS correct,
		COUNT(*) FILTER (WHERE action='wrong') AS wrong,
		COUNT(*) FILTER (WHERE action='advance') AS advance,
		COUNT(*) FILTER (WHERE action='rollback') AS rollback,
		COUNT(*) FILTER (WHERE action='skip_rehab') AS skip_rehab,
		COUNT(DISTINCT material_id) FILTER (WHERE action IN ('correct','wrong')) AS materials_reviewed,
		COUNT(*) FILTER (WHERE action='correct' AND stage_after>stage_before) AS stage_promotions`).Scan(&s).Error
	return s, err
}
