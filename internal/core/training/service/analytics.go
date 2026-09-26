package service

import (
	"context"
	"strconv"
	"time"

	folder "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/google/uuid"
)

// The service boundary owns pending analytics. The repository remains unchanged;
// no publisher runs inside a transaction or before its commit succeeds.
type eventTx struct {
	repository.Tx
	events    []analytics.Event
	templates map[string]string
}

func (t *eventTx) Folder(id uuid.UUID) (folder.Folder, error) {
	f, err := t.Tx.Folder(id)
	if err == nil {
		t.templates[id.String()] = f.TemplateKey
	}
	return f, err
}
func (s *Service) transact(ctx context.Context, user uuid.UUID, fn func(repository.Tx) error) error {
	var pending *eventTx
	err := s.store.Transact(ctx, user, func(tx repository.Tx) error {
		pending = &eventTx{Tx: tx, templates: map[string]string{}}
		return fn(pending)
	})
	if err == nil && pending != nil {
		for _, e := range pending.events {
			if e.Template == "" {
				e.Template = pending.templates[e.FolderID]
			}
			s.Publish(ctx, e)
		}
	}
	return err
}
func queueEvent(tx repository.Tx, e analytics.Event) {
	if pending, ok := tx.(*eventTx); ok {
		pending.events = append(pending.events, e)
	}
}
func mode(p model.TrainingPlan, session model.TrainingSession) string {
	if session.PlanID == uuid.Nil && session.SelectionStrategy == model.SelectionInterviewGraphV1 {
		return "mock"
	}
	return string(p.Track)
}
func sessionEvent(tx repository.Tx, name analytics.Name, p model.TrainingPlan, session model.TrainingSession, at time.Time) {
	e := analytics.New(name, session.UserID)
	e.EventID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(name)+":"+session.ID.String())).String()
	e.OccurredAt, e.SessionID, e.Mode = at, session.ID.String(), mode(p, session)
	e.AlgorithmVersion = p.AlgorithmKey + ":" + strconv.Itoa(p.AlgorithmVersion)
	if len(p.SourceFolderIDs) == 1 {
		e.FolderID = p.SourceFolderIDs[0].String()
	}
	queueEvent(tx, e)
	derived := func(start, complete analytics.Name) {
		d := e
		if name == analytics.TrainingStarted {
			d.EventName = start
		} else if name == analytics.TrainingCompleted {
			d.EventName = complete
		} else {
			return
		}
		d.EventID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(d.EventName)+":"+session.ID.String())).String()
		queueEvent(tx, d)
	}
	if p.Track == model.ProgressTrackCram {
		derived(analytics.CramStarted, analytics.CramCompleted)
	}
	if session.SelectionStrategy == model.SelectionInterviewGraphV1 {
		derived(analytics.InterviewStarted, analytics.InterviewCompleted)
	}
}

func answerEvent(e model.TrainingEvent, p model.TrainingPlan, session model.TrainingSession, m material.Material, shown *model.Presentation, before, after *model.UserMaterialProgress) analytics.Event {
	name := analytics.TrainingAnswered
	switch e.Action {
	case "skip_rehab", "next_route":
		name = analytics.TrainingSkipped
	case "rollback":
		name = analytics.TrainingRollback
	}
	out := analytics.New(name, e.UserID)
	out.EventID, out.OccurredAt = e.ID.String(), e.CreatedAt
	out.SessionID, out.FolderID, out.MaterialID = e.SessionID.String(), m.FolderID.String(), e.MaterialID.String()
	out.Mode, out.Result = mode(p, session), e.Action
	out.ReviewKind = string(e.Kind)
	out.AlgorithmVersion = e.AlgorithmKey + ":" + strconv.Itoa(e.AlgorithmVersion)
	out.ReviewCredit = analytics.Ptr(e.ReviewCredit)
	for key, dst := range map[string]**string{"topic": &out.Topic, "subtopic": &out.Subtopic} {
		if v := m.Metadata[key]; v != nil && len(*v) <= 200 {
			*dst = analytics.Ptr(*v)
		}
	}
	out.Difficulty = analytics.Ptr(string(m.Difficulty))
	if shown != nil {
		out.Difficulty = analytics.Ptr(string(shown.Difficulty))
		if !shown.CreatedAt.IsZero() && !e.CreatedAt.Before(shown.CreatedAt) {
			out.AnswerTimeMS = analytics.Ptr(e.CreatedAt.Sub(shown.CreatedAt).Milliseconds())
		}
	}
	if before != nil {
		out.StageBefore = analytics.Ptr(before.Stage)
		out.ConsecutiveCorrectBefore = analytics.Ptr(before.ConsecutiveCorrect)
		out.WrongCountBefore = analytics.Ptr(before.WrongCount)
		out.RehabActiveBefore = analytics.Ptr(before.RehabActive)
		out.NextReviewBefore = before.NextReviewAt()
		out.LearnedBefore = analytics.Ptr(before.CompletedAt != nil)
	}
	if after != nil {
		out.StageAfter = analytics.Ptr(after.Stage)
		out.ConsecutiveCorrectAfter = analytics.Ptr(after.ConsecutiveCorrect)
		out.WrongCountAfter = analytics.Ptr(after.WrongCount)
		out.RehabActiveAfter = analytics.Ptr(after.RehabActive)
		out.NextReviewAfter = after.NextReviewAt()
		out.LearnedAfter = analytics.Ptr(after.CompletedAt != nil)
	}
	return out
}
func queueAnswer(tx repository.Tx, e analytics.Event, interview bool) {
	if e.Result == "advance" {
		return
	} // Manual promotion is not an answer.
	if interview && e.Mode == "mock" && e.EventName == analytics.TrainingAnswered {
		// Practice-only answers must not inflate normal SRS answer analytics.
		e.EventName = analytics.InterviewQuestionAnswered
		queueEvent(tx, e)
		return
	}
	queueEvent(tx, e)
	derived := func(name analytics.Name) {
		d := e
		d.EventName = name
		d.RelatedEventID = e.EventID
		d.EventID = uuid.NewString()
		queueEvent(tx, d)
	}
	if interview && e.EventName == analytics.TrainingAnswered {
		derived(analytics.InterviewQuestionAnswered)
	}
	if e.RehabActiveBefore != nil && e.RehabActiveAfter != nil {
		if !*e.RehabActiveBefore && *e.RehabActiveAfter {
			derived(analytics.RehabStarted)
		}
		if *e.RehabActiveBefore && !*e.RehabActiveAfter && e.Result == "correct" && e.ReviewKind == string(model.ReviewRehab) {
			derived(analytics.RehabCompleted)
		}
	}
}
