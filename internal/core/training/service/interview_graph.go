package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"reflect"
	"strings"
)

type StartGraphRequest struct {
	CommandID uuid.UUID             `json:"command_id"`
	Sources   []model.SessionSource `json:"sources"`
	Config    *graph.Config         `json:"config"`
}

func graphHash(prefix string, value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(append([]byte(prefix), raw...))
	return hex.EncodeToString(sum[:])
}
func (s *Service) StartGraph(ctx context.Context, user, planID uuid.UUID, req StartGraphRequest) (model.SessionView, error) {
	var out model.SessionView
	if req.CommandID == uuid.Nil {
		return out, ErrInvalid
	}
	config := graph.DefaultConfig()
	if req.Config != nil {
		config = *req.Config
	}
	if err := config.Validate(); err != nil {
		return out, errors.Join(ErrInvalid, err)
	}
	hash := graphHash("graph-start:", struct {
		PlanID  uuid.UUID
		Request StartGraphRequest
	}{planID, req})
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		receipt, err := tx.Receipt(req.CommandID)
		if err == nil {
			if receipt.RequestHash != hash {
				return repository.ErrConflict
			}
			return json.Unmarshal(receipt.Response, &out)
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		plan, err := tx.Plan(planID)
		if err != nil {
			return err
		}
		if plan.Status != model.StatusActive {
			return repository.ErrConflict
		}
		if plan.AlgorithmKey != "interview_long_term" && plan.AlgorithmKey != "interview_cram" {
			return fmt.Errorf("%w: graph selection needs an interview plan", ErrInvalid)
		}
		sources := req.Sources
		if len(sources) == 0 {
			for _, id := range plan.SourceFolderIDs {
				sources = append(sources, model.SessionSource{FolderID: id})
			}
		}
		if len(sources) > 100 {
			return ErrInvalid
		}
		seen := map[uuid.UUID]bool{}
		for _, source := range sources {
			if source.FolderID == uuid.Nil || seen[source.FolderID] || !containsFolder(plan, source.FolderID) || len(source.Topics) > 100 {
				return fmt.Errorf("%w: graph sources must be a unique subset of the plan", ErrInvalid)
			}
			seen[source.FolderID] = true
			folder, err := tx.Folder(source.FolderID)
			if err != nil {
				return err
			}
			if folder.TemplateKey != "interview_questions" {
				return ErrInvalid
			}
			topics := map[string]bool{}
			for _, topic := range source.Topics {
				if strings.TrimSpace(topic) == "" || len(topic) > 200 || topics[topic] {
					return ErrInvalid
				}
				topics[topic] = true
			}
		}
		active, err := tx.ActiveSession(planID)
		if err == nil {
			if active.SelectionStrategy != model.SelectionInterviewGraphV1 || active.Combined {
				return fmt.Errorf("%w: finish the active card session before starting an interview", repository.ErrConflict)
			}
			state, err := tx.InterviewGraph().State(active.ID)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(active.Selection, sources) || state.Config != config {
				return fmt.Errorf("%w: resume or end the existing interview before changing its settings", repository.ErrConflict)
			}
			out, err = s.graphView(ctx, tx, plan, active)
			if err != nil {
				return err
			}
		} else {
			if !errors.Is(err, repository.ErrNotFound) {
				return err
			}
			now := s.now().UTC()
			session := model.TrainingSession{ID: uuid.New(), PlanID: plan.ID, UserID: user, SelectionStrategy: model.SelectionInterviewGraphV1, Selection: sources, Status: model.StatusActive, StartedAt: now, CreatedAt: now}
			if err = tx.CreateSession(session); err != nil {
				return err
			}
			var seed [8]byte
			if _, err = rand.Read(seed[:]); err != nil {
				return err
			}
			state := graph.State{StrategyVersion: 1, RandomSeed: int64(binary.LittleEndian.Uint64(seed[:]) & 0x7fffffffffffffff), Config: config, AskedMaterialIDs: []uuid.UUID{}, RecentConcepts: []string{}, Frontier: []graph.FrontierEntry{}, ForksUsed: map[int]int{}}
			for _, source := range sources {
				state.Sources = append(state.Sources, graph.Source{FolderID: source.FolderID, Topics: source.Topics})
			}
			candidates, err := tx.InterviewGraph().RootCandidates(plan, session, now)
			if err != nil {
				return err
			}
			selected := graph.SelectRoot(state, candidates)
			if _, err = s.persistGraphSelection(ctx, tx, plan, &session, state, selected, nil, ""); err != nil {
				return err
			}
			out, err = s.graphView(ctx, tx, plan, session)
			if err != nil {
				return err
			}
		}
		response, err := json.Marshal(out)
		if err != nil {
			return err
		}
		return tx.SaveReceipt(model.CommandReceipt{UserID: user, CommandID: req.CommandID, RequestHash: hash, Response: response})
	})
	return out, err
}
func (s *Service) persistGraphSelection(ctx context.Context, tx repository.Tx, p model.TrainingPlan, session *model.TrainingSession, before graph.State, selected graph.Selection, from *uuid.UUID, answer string) (*uuid.UUID, error) {
	now := s.now().UTC()
	gr := tx.InterviewGraph()
	if selected.Candidate == nil {
		if _, err := gr.SaveState(session.ID, selected.State, before.Version, now); err != nil {
			return nil, err
		}
		if session.Status == model.StatusActive {
			if err := tx.FinishSession(session.ID, model.StatusCompleted, now); err != nil {
				return nil, err
			}
			session.Status = model.StatusCompleted
			session.FinishedAt = &now
		}
		return nil, nil
	}
	c := selected.Candidate
	m, err := tx.Material(c.MaterialID)
	if err != nil {
		return nil, err
	}
	progress, err := tx.Progress().Get(ctx, keyFor(p, m.ID))
	if selected.ReviewCredit && !selected.State.Config.IncludeDraft {
		progress, err = s.ensureProgress(ctx, tx, p, m, now)
	} else if errors.Is(err, repository.ErrNotFound) {
		progress = model.UserMaterialProgress{MaterialID: m.ID, Stage: 1}
		err = nil
	}
	if err != nil {
		return nil, err
	}
	// A newly encountered probe uses initial progress; answering it cannot change
	// that initial schedule. Root admission receives normal new-material credit.
	kind, due := progress.DueReview(now)
	if !due {
		kind = model.ReviewStage
	}
	credit := selected.ReviewCredit && due
	a, err := s.registry.Get(p.AlgorithmKey, p.AlgorithmVersion)
	if err != nil {
		return nil, err
	}
	difficulty := progress.EffectiveDifficulty(m.Difficulty)
	required, err := a.RequiredCorrect(difficulty)
	if err != nil {
		return nil, err
	}
	if kind == model.ReviewExtra {
		required = 1
	}
	p, err = currentCardConfig(tx, p)
	if err != nil {
		return nil, err
	}
	cfg, ok := p.Config.Cards[m.FolderID.String()]
	if !ok {
		return nil, repository.ErrConflict
	}
	question := []model.CardField{{Key: "question", Label: "Question", Value: c.Question}}
	answerFields := []model.CardField{}
	for _, f := range []struct{ key, label string }{{"short_answer", "Short Answer"}, {"answer", "Detailed Answer"}, {"sources", "Source"}} {
		value := ""
		if m.Values[f.key] != nil {
			value = *m.Values[f.key]
		}
		answerFields = append(answerFields, model.CardField{Key: f.key, Label: f.label, Value: value})
	}
	_ = cfg // Folder ownership/config are validated above; graph has fixed reference fields.
	if len(question) == 0 {
		return nil, repository.ErrConflict
	}
	// Candidates are queried and persisted under the user row lock also acquired
	// by material/profile edits and deletion, so this snapshot cannot race those writes.
	event := model.GraphSelectionEvent{ID: uuid.New(), SessionID: session.ID, FromMaterialID: from, ToMaterialID: m.ID, RootIndex: selected.State.CurrentRoot, DepthBefore: before.CurrentDepth, DepthAfter: selected.State.CurrentDepth, DetectedConcepts: selected.Matches, Candidates: selected.Scores, SelectionReason: selected.Reason, ReviewCredit: credit, RandomSeed: selected.State.RandomSeed, Snapshot: *c, CreatedAt: now}
	event.SelectionOrder = selected.State.QuestionsAsked
	if event.DetectedConcepts == nil {
		event.DetectedConcepts = []graph.Match{}
	}
	if event.Candidates == nil {
		event.Candidates = []graph.Score{}
	}
	if err = gr.SaveSelection(event); err != nil {
		return nil, err
	}
	final := false
	stage, correct := progress.Stage, progress.ConsecutiveCorrect
	if interview, ok := a.(algorithm.Interview); ok && credit {
		final = kind == model.ReviewStage && interview.IsFinal(progress, now)
		if final {
			schedule, err := interview.Schedule(progress)
			if err != nil {
				return nil, err
			}
			stage = len(schedule)
			if stage != progress.Stage {
				correct = 0
			}
		}
	}
	shown := &model.Presentation{ID: uuid.New(), MaterialID: m.ID, FolderID: m.FolderID, Kind: kind, ProgressVersion: progress.Version, Stage: stage, ConsecutiveCorrect: correct, RehabConsecutiveCorrect: progress.RehabConsecutiveCorrect, RequiredCorrect: required, Difficulty: difficulty, Question: question, Answer: answerFields, FinalReview: final, CreatedAt: now, InterviewGraph: &model.InterviewGraphPresentation{SelectionEventID: event.ID, RootIndex: event.RootIndex, Depth: event.DepthAfter, Probe: !credit, ReviewCredit: credit}}
	shown.InterviewGraph.BankVerification = selected.State.Config.IncludeDraft
	shown.InterviewGraph.AnswerIncomplete = !c.HasAnswer
	if err = tx.SaveItem(model.SessionItem{SessionID: session.ID, MaterialID: m.ID, State: "active", Position: int64(selected.State.QuestionsAsked), Presentation: shown}); err != nil {
		return nil, err
	}
	if _, err = gr.SaveState(session.ID, selected.State, before.Version, now); err != nil {
		return nil, err
	}
	return &event.ID, nil
}
func (s *Service) graphView(ctx context.Context, tx repository.Tx, p model.TrainingPlan, session model.TrainingSession) (model.SessionView, error) {
	out := model.SessionView{Session: session}
	gr := tx.InterviewGraph()
	state, err := gr.State(session.ID)
	if err != nil {
		return out, err
	}
	items, err := tx.Items(session.ID)
	if err != nil {
		return out, err
	}
	if session.Status == model.StatusActive && len(items) > 0 {
		item := items[0]
		if item.Presentation == nil || item.Presentation.InterviewGraph == nil {
			return out, repository.ErrConflict
		}
		_, err := tx.Material(item.MaterialID)
		if errors.Is(err, repository.ErrNotFound) {
			current, err := gr.Selection(session.ID, item.Presentation.InterviewGraph.SelectionEventID)
			if err != nil {
				return out, err
			}
			item.State = "completed"
			item.Presentation = nil
			if err = tx.SaveItem(item); err != nil {
				return out, err
			}
			candidates, err := gr.FollowUpCandidates(p, session, s.now().UTC())
			if err != nil {
				return out, err
			}
			catalog, err := gr.Catalog()
			if err != nil {
				return out, err
			}
			next := graph.SelectMetadata(state, current.Snapshot, "next_route", candidates, catalog)
			if _, err = s.persistGraphSelection(ctx, tx, p, &session, state, next, &item.MaterialID, ""); err != nil {
				return out, err
			}
			return s.graphView(ctx, tx, p, session)
		} else if err != nil {
			return out, err
		}
		out.Current = item.Presentation
		out.PoolSize = 1
	}
	var event *model.GraphSelectionEvent
	if out.Current != nil {
		selected, err := gr.Selection(session.ID, out.Current.InterviewGraph.SelectionEventID)
		if err != nil {
			return out, err
		}
		event = &selected
	} else {
		event, err = gr.LatestSelection(session.ID)
		if err != nil {
			return out, err
		}
	}
	stats, err := gr.Statistics(session.ID)
	if err != nil {
		return out, err
	}
	out.Graph = &model.InterviewGraphView{State: state, Selection: event, Statistics: stats}
	out.Summary, err = tx.Summary(session.ID)
	if err != nil {
		return out, err
	}
	out.UndoActions, err = availableUndos(ctx, tx, []model.TrainingSession{session})
	return out, err
}
