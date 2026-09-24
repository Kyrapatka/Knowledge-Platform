package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/algorithm"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/Kyrapatka/knowledge-platform/internal/platform/analytics"
	"github.com/google/uuid"
)

type capturePublisher struct {
	events    []analytics.Event
	committed *bool
	t         *testing.T
}

func (p *capturePublisher) Publish(_ context.Context, e analytics.Event) {
	if p.committed != nil && !*p.committed {
		p.t.Error("published before commit")
	}
	p.events = append(p.events, e)
}

type transactionFake struct {
	tx        repository.Tx
	commitErr error
	committed bool
}

func (f *transactionFake) Transact(_ context.Context, _ uuid.UUID, fn func(repository.Tx) error) error {
	if err := fn(f.tx); err != nil {
		return err
	}
	if f.commitErr != nil {
		return f.commitErr
	}
	f.committed = true
	return nil
}
func TestAnalyticsCommitBoundary(t *testing.T) {
	for _, failure := range []string{"none", "callback", "commit"} {
		t.Run(failure, func(t *testing.T) {
			f := &transactionFake{}
			if failure == "commit" {
				f.commitErr = errors.New("commit failed")
			}
			s := NewService(f)
			p := &capturePublisher{committed: &f.committed, t: t}
			s.SetPublisher(p)
			err := s.transact(context.Background(), uuid.New(), func(tx repository.Tx) error {
				queueEvent(tx, analytics.New(analytics.TrainingAnswered, uuid.New()))
				if failure == "callback" {
					return errors.New("rollback")
				}
				return nil
			})
			if failure == "none" {
				if err != nil || len(p.events) != 1 {
					t.Fatal("missing committed event")
				}
			} else if err == nil || len(p.events) != 0 {
				t.Fatal("published rolled-back event")
			}
		})
	}
}
func TestAnsweredSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	m := material.Material{ID: uuid.New(), FolderID: uuid.New(), Difficulty: material.DifficultyMedium, Metadata: map[string]*string{"topic": analytics.Ptr("Go"), "subtopic": analytics.Ptr("goroutines"), "password": analytics.Ptr("never-copy")}}
	before := model.UserMaterialProgress{Stage: 2, ConsecutiveCorrect: 1, WrongCount: 3, StageReviewAt: &now}
	after := before
	after.Stage = 3
	after.ConsecutiveCorrect = 2
	after.CompletedAt = &now
	e := model.TrainingEvent{ID: uuid.New(), UserID: uuid.New(), SessionID: uuid.New(), MaterialID: m.ID, Action: "correct", AlgorithmKey: "english", AlgorithmVersion: 1, CreatedAt: now, ReviewCredit: true}
	out := answerEvent(e, model.TrainingPlan{Track: model.ProgressTrackDefault}, model.TrainingSession{PlanID: uuid.New()}, m, &model.Presentation{CreatedAt: now.Add(-2 * time.Second), Difficulty: m.Difficulty}, &before, &after)
	if out.EventID != e.ID.String() || *out.StageBefore != 2 || *out.StageAfter != 3 || *out.AnswerTimeMS != 2000 || *out.Topic != "Go" || !*out.LearnedAfter || out.RetrievabilityBefore != nil || out.StabilityAfter != nil {
		t.Fatalf("bad snapshot: %+v", out)
	}
	raw, _ := json.Marshal(out)
	if strings.Contains(string(raw), "never-copy") {
		t.Fatal("copied private metadata")
	}
	after.Stage = 9
	if *out.StageAfter != 3 {
		t.Fatal("snapshot aliases progress")
	}
	probe := answerEvent(e, model.TrainingPlan{}, model.TrainingSession{}, m, nil, nil, nil)
	if probe.StageBefore != nil || probe.WrongCountAfter != nil {
		t.Fatal("fabricated mock progress")
	}
}

type replayTx struct {
	repository.Tx
	session model.TrainingSession
	receipt model.CommandReceipt
}

func (r replayTx) Session(uuid.UUID) (model.TrainingSession, error) { return r.session, nil }
func (r replayTx) Receipt(uuid.UUID) (model.CommandReceipt, error)  { return r.receipt, nil }
func TestActReceiptReplayDoesNotRepublish(t *testing.T) {
	sessionID := uuid.New()
	req := ActionRequest{CommandID: uuid.New(), PresentationID: uuid.New(), ExpectedVersion: 1, Action: algorithm.Correct}
	raw, _ := json.Marshal(struct {
		SessionID uuid.UUID
		Request   ActionRequest
	}{sessionID, req})
	sum := sha256.Sum256(raw)
	want := model.ActionResult{Event: model.TrainingEvent{ID: uuid.New()}}
	response, _ := json.Marshal(want)
	f := &transactionFake{tx: replayTx{session: model.TrainingSession{ID: sessionID}, receipt: model.CommandReceipt{RequestHash: hex.EncodeToString(sum[:]), Response: response}}}
	s := NewService(f)
	p := &capturePublisher{}
	s.SetPublisher(p)
	out, err := s.Act(context.Background(), uuid.New(), sessionID, req)
	if err != nil || out.Event.ID != want.Event.ID || len(p.events) != 0 {
		t.Fatal("replay changed result or emitted event", err)
	}
}
func TestRehabCompletionRequiresRehabAnswer(t *testing.T) {
	for _, kind := range []string{"stage", "rehab"} {
		tx := &eventTx{}
		e := analytics.New(analytics.TrainingAnswered, uuid.New())
		e.Result = "correct"
		e.ReviewKind = kind
		e.RehabActiveBefore = analytics.Ptr(true)
		e.RehabActiveAfter = analytics.Ptr(false)
		queueAnswer(tx, e, false)
		want := 1
		if kind == "rehab" {
			want = 2
		}
		if len(tx.events) != want {
			t.Fatal("misclassified rehab exit")
		}
	}
}
func TestNoEventsOnReadOnlyOrReceiptReplayBoundary(t *testing.T) {
	f := &transactionFake{}
	s := NewService(f)
	p := &capturePublisher{}
	s.SetPublisher(p)
	// Replay handlers only deserialize receipts: no successful-operation event is queued.
	if err := s.transact(context.Background(), uuid.New(), func(repository.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(p.events) != 0 {
		t.Fatal("duplicate replay event")
	}
}
