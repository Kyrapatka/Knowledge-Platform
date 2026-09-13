package postgres

import (
	"encoding/json"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"sort"
	"time"
)

type graphTx struct{ t *runtimeTx }

func (t *runtimeTx) InterviewGraph() repository.InterviewGraphRepository { return &graphTx{t} }
func (g *graphTx) RootCandidates(p model.TrainingPlan, s model.TrainingSession, now time.Time) ([]graph.Candidate, error) {
	return g.candidates(p, s, now, true)
}
func (g *graphTx) FollowUpCandidates(p model.TrainingPlan, s model.TrainingSession, now time.Time) ([]graph.Candidate, error) {
	return g.candidates(p, s, now, false)
}
func (g *graphTx) candidates(p model.TrainingPlan, s model.TrainingSession, now time.Time, root bool) ([]graph.Candidate, error) {
	out := []graph.Candidate{}
	if p.UserID != g.t.user || s.UserID != g.t.user {
		return out, repository.ErrNotFound
	}
	query := g.t.db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").Joins("JOIN interview_question_profiles q ON q.material_id=m.id AND q.folder_id=m.folder_id").
		Joins("LEFT JOIN user_material_progress p ON p.material_id=m.id AND p.user_id=? AND p.track=? AND ((p.track='cram' AND p.plan_id=?) OR (p.track<>'cram' AND p.plan_id IS NULL))", g.t.user, p.Track, p.ID).
		Where("f.owner_id=? AND f.deleted_at IS NULL AND m.deleted_at IS NULL AND f.template_key='interview_questions' AND m.folder_id IN ? AND q.status='ready'", g.t.user, p.SourceFolderIDs).
		Where("NULLIF(btrim(m.values->>'question'),'') IS NOT NULL AND (NULLIF(btrim(m.values->>'answer'),'') IS NOT NULL OR NULLIF(btrim(m.values->>'short_answer'),'') IS NOT NULL)").
		Where("(p.material_id IS NULL OR (p.algorithm_key=? AND p.algorithm_version=?))", p.AlgorithmKey, p.AlgorithmVersion)
	query = selectedMaterials(query, s.Selection)
	if root {
		query = query.Where("q.root_weight>0")
	}
	err := query.Select(`q.*,m.id AS material_id,m.folder_id,m.values->>'question' AS question,COALESCE(m.metadata->>'topic',m.metadata->>'category','') AS topic,
 TRUE AS has_answer, (p.material_id IS NULL) AS new, COALESCE(p.next_review_at<=?,FALSE) AS due,
 CASE WHEN p.material_id IS NULL THEN 1.0 ELSE LEAST(1.0,GREATEST(0.0,(p.wrong_count+1.0)/(p.correct_count+p.wrong_count+2.0))) END AS learning_need`, now).
		Order("m.id").Limit(5000).Scan(&out).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(out))
	byID := map[uuid.UUID]int{}
	for i := range out {
		ids[i] = out[i].MaterialID
		byID[ids[i]] = i
		out[i].Concepts = []graph.Link{}
	}
	if len(ids) == 0 {
		return out, nil
	}
	var links []struct {
		MaterialID uuid.UUID
		Slug, Role string
		Weight     float64
	}
	if err = g.t.db.Table("interview_question_concepts q").Select("q.material_id,c.slug,q.role,q.weight").Joins("JOIN interview_concepts c ON c.id=q.concept_id").Where("q.material_id IN ? AND c.owner_id=?", ids, g.t.user).Order("q.material_id,q.role,c.slug").Scan(&links).Error; err != nil {
		return nil, err
	}
	for _, l := range links {
		i := byID[l.MaterialID]
		out[i].Concepts = append(out[i].Concepts, graph.Link{Slug: l.Slug, Role: l.Role, Weight: l.Weight})
	}
	return out, nil
}
func (g *graphTx) Catalog() (graph.Catalog, error) {
	raw, err := interview.LoadCatalog(g.t.db, g.t.user)
	out := graph.Catalog{DocumentFrequency: map[string]int{}}
	if err != nil {
		return out, err
	}
	for _, c := range raw.Concepts {
		for _, a := range c.Aliases {
			out.Aliases = append(out.Aliases, graph.Alias{ConceptID: c.ID, Slug: c.Slug, Text: a.Alias, Language: a.Language, Weight: a.Weight, WholeWord: a.WholeWord, Constraints: graph.Constraints{RequiresAny: a.Constraints.RequiresAny, RequiresDomain: a.Constraints.RequiresDomain}})
		}
	}
	for _, e := range raw.Edges {
		out.Edges = append(out.Edges, graph.Edge{From: e.From, To: e.To, Relation: e.Relation, Weight: e.Weight})
	}
	var counts []struct {
		Slug  string
		Count int
	}
	err = g.t.db.Table("interview_question_concepts q").Select("c.slug,COUNT(DISTINCT q.material_id) AS count").Joins("JOIN interview_concepts c ON c.id=q.concept_id").Where("c.owner_id=? AND q.role IN ('primary','tested')", g.t.user).Group("c.slug").Scan(&counts).Error
	if err != nil {
		return out, err
	}
	for _, c := range counts {
		out.DocumentFrequency[c.Slug] = c.Count
	}
	var n int64
	err = g.t.db.Table("interview_question_profiles q").Joins("JOIN folders f ON f.id=q.folder_id").Where("f.owner_id=?", g.t.user).Count(&n).Error
	out.QuestionCount = int(n)
	return out, err
}
func (g *graphTx) State(id uuid.UUID) (graph.State, error) {
	var out graph.State
	if _, err := g.t.Session(id); err != nil {
		return out, err
	}
	var row struct {
		State   []byte
		Version int
	}
	if err := g.t.db.Table("interview_graph_session_state").Where("session_id=?", id).Take(&row).Error; err != nil {
		return out, translate(err)
	}
	err := json.Unmarshal(row.State, &out)
	out.Version = row.Version
	return out, err
}
func (g *graphTx) SaveState(id uuid.UUID, state graph.State, expected int, now time.Time) (graph.State, error) {
	if _, err := g.t.Session(id); err != nil {
		return state, err
	}
	state.Version = expected + 1
	raw, err := json.Marshal(state)
	if err != nil {
		return state, err
	}
	if expected == 0 {
		err = g.t.db.Table("interview_graph_session_state").Create(map[string]any{"session_id": id, "strategy_version": 1, "state": raw, "version": 1, "created_at": now, "updated_at": now}).Error
		return state, err
	}
	r := g.t.db.Table("interview_graph_session_state").Where("session_id=? AND version=?", id, expected).Updates(map[string]any{"state": raw, "version": state.Version, "updated_at": now})
	if r.Error != nil {
		return state, r.Error
	}
	if r.RowsAffected != 1 {
		return state, repository.ErrConflict
	}
	return state, nil
}
func (g *graphTx) SaveSelection(e model.GraphSelectionEvent) error {
	if _, err := g.t.Session(e.SessionID); err != nil {
		return err
	}
	return g.t.db.Table("interview_graph_selection_events").Create(&e).Error
}
func (g *graphTx) Selection(session, id uuid.UUID) (model.GraphSelectionEvent, error) {
	var e model.GraphSelectionEvent
	if _, err := g.t.Session(session); err != nil {
		return e, err
	}
	err := g.t.db.Table("interview_graph_selection_events").Where("session_id=? AND id=? AND undone_at IS NULL", session, id).Take(&e).Error
	return e, translate(err)
}
func (g *graphTx) LatestSelection(session uuid.UUID) (*model.GraphSelectionEvent, error) {
	if _, err := g.t.Session(session); err != nil {
		return nil, err
	}
	var events []model.GraphSelectionEvent
	err := g.t.db.Table("interview_graph_selection_events").Where("session_id=? AND undone_at IS NULL", session).Order("selection_order DESC,id DESC").Limit(1).Find(&events).Error
	if err != nil || len(events) == 0 {
		return nil, err
	}
	return &events[0], nil
}
func (g *graphTx) UndoSelection(session, id uuid.UUID, now time.Time) error {
	if _, err := g.t.Session(session); err != nil {
		return err
	}
	r := g.t.db.Table("interview_graph_selection_events").Where("session_id=? AND id=? AND undone_at IS NULL", session, id).Update("undone_at", now)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return repository.ErrConflict
	}
	return nil
}
func (g *graphTx) Statistics(session uuid.UUID) (model.GraphStatistics, error) {
	out := model.GraphStatistics{ConceptCoverage: []model.ConceptCoverage{}}
	if _, err := g.t.Session(session); err != nil {
		return out, err
	}
	var rows []struct {
		Action         string
		ReviewCredit   bool
		DepthAfter     int
		Snapshot       []byte
		FromMaterialID *uuid.UUID
	}
	// Join via the exact displayed snapshot, so edits/deletion cannot rewrite coverage.
	err := g.t.db.Raw(`SELECT e.action,e.review_credit,s.depth_after,s.snapshot,s.from_material_id FROM training_events e
 JOIN interview_graph_selection_events s ON s.session_id=e.session_id AND s.to_material_id=e.material_id
 WHERE e.user_id=? AND e.session_id=? AND e.undone_at IS NULL AND s.undone_at IS NULL AND e.action IN ('correct','wrong') ORDER BY s.selection_order,e.id`, g.t.user, session).Scan(&rows).Error
	if err != nil {
		return out, err
	}
	coverage := map[string]*model.ConceptCoverage{}
	domains := map[uuid.UUID]string{}
	totalDepth := 0
	for _, r := range rows {
		var c graph.Candidate
		if err := json.Unmarshal(r.Snapshot, &c); err != nil {
			return out, err
		}
		domains[c.MaterialID] = c.Domain
		if r.ReviewCredit {
			out.ScheduledReviews++
		} else {
			out.InterviewProbes++
		}
		if r.Action == "correct" {
			out.Correct++
		} else {
			out.Wrong++
		}
		out.MaxDepth = max(out.MaxDepth, r.DepthAfter)
		totalDepth += r.DepthAfter
		if r.FromMaterialID != nil && domains[*r.FromMaterialID] != "" && domains[*r.FromMaterialID] != c.Domain {
			out.CrossDomainTransitions++
		}
		seen := map[string]bool{}
		for _, l := range c.Concepts {
			if (l.Role != "primary" && l.Role != "tested") || seen[l.Slug] {
				continue
			}
			seen[l.Slug] = true
			if coverage[l.Slug] == nil {
				coverage[l.Slug] = &model.ConceptCoverage{Slug: l.Slug}
			}
			x := coverage[l.Slug]
			x.Asked++
			if r.Action == "correct" {
				x.Correct++
			} else {
				x.Wrong++
			}
		}
	}
	if len(rows) > 0 {
		out.AverageDepth = float64(totalDepth) / float64(len(rows))
	}
	for _, v := range coverage {
		out.ConceptCoverage = append(out.ConceptCoverage, *v)
	}
	sort.Slice(out.ConceptCoverage, func(i, j int) bool { return out.ConceptCoverage[i].Slug < out.ConceptCoverage[j].Slug })
	return out, nil
}
