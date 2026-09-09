package algorithm

import (
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
	"testing"
	"time"
)

func interviewProgress(t *testing.T, a Interview, horizon int) model.UserMaterialProgress {
	t.Helper()
	track := model.ProgressTrackLongTerm
	var id *uuid.UUID
	if a.Cram {
		track = model.ProgressTrackCram
		v := uuid.New()
		id = &v
	}
	p, err := model.NewProgress(model.NewProgressParams{UserID: uuid.New(), MaterialID: uuid.New(), PlanID: id, Track: track, AlgorithmKey: a.Key(), AlgorithmVersion: 1, HorizonDays: horizon, Now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func answerInterview(t *testing.T, a Interview, p model.UserMaterialProgress, action Action, now time.Time) model.UserMaterialProgress {
	t.Helper()
	kind, ok := p.DueReview(now)
	if !ok {
		t.Fatal("not due", p)
	}
	next, err := a.Apply(Input{Progress: p, Difficulty: material.DifficultyEasy, Kind: kind, Action: action, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err = next.Validate(); err != nil {
		t.Fatal(err)
	}
	return next
}
func TestInterviewAllSchedulesReachIndividualTarget(t *testing.T) {
	for _, a := range []Interview{{Cram: true}, {}} {
		low, high := 7, 365
		if a.Cram {
			low, high = 1, 7
		}
		for h := low; h <= high; h++ {
			p := interviewProgress(t, a, h)
			target := *p.TargetAt
			count := 0
			for p.CompletedAt == nil {
				if count > 15 {
					t.Fatal("nonterminating schedule", h)
				}
				count++
				now := *p.StageReviewAt
				p = answerInterview(t, a, p, Correct, now)
				if !p.TargetAt.Equal(target) {
					t.Fatal("target moved", h)
				}
			}
			if !p.CompletedAt.Equal(target) || p.NextReviewAt() != nil {
				t.Fatal("wrong completion", h, p)
			}
		}
	}
}
func TestCramSpacingWrongAndDifficulty(t *testing.T) {
	a := Interview{Cram: true}
	p := interviewProgress(t, a, 1)
	start := p.LearningStartedAt
	p = answerInterview(t, a, p, Correct, start)
	if p.Stage != 2 || p.StageReviewAt.Sub(start) != 8*time.Hour {
		t.Fatal(p)
	}
	now := *p.StageReviewAt
	for i := 0; i < 3; i++ {
		p = answerInterview(t, a, p, Wrong, now)
	}
	if p.Stage != 2 || p.RehabActive || p.DifficultyOverride == nil || *p.DifficultyOverride != material.DifficultyMedium {
		t.Fatal(p)
	}
	p = answerInterview(t, a, p, Correct, now)
	if p.Stage != 2 {
		t.Fatal("mastery must now require two answers")
	}
	p = answerInterview(t, a, p, Correct, now)
	if p.Stage != 3 || !p.StageReviewAt.Equal(*p.TargetAt) {
		t.Fatal(p)
	}
	_, err := a.Apply(Input{Progress: p, Action: Advance, Now: now, Difficulty: material.DifficultyEasy})
	if err == nil {
		t.Fatal("CRAM must not skip stages")
	}
}
func TestLongTermRollbackUsesGapAndPreservesTarget(t *testing.T) {
	for _, tc := range []struct{ horizon, stage, want int }{{120, 8, 7}, {365, 7, 6}, {365, 8, 6}, {365, 11, 9}} {
		a := Interview{}
		p := interviewProgress(t, a, tc.horizon)
		p.Stage = tc.stage
		now := p.LearningStartedAt
		target := *p.TargetAt
		p = answerInterview(t, a, p, Wrong, now)
		if p.Stage != tc.want || !p.RehabActive || !p.TargetAt.Equal(target) {
			t.Fatal(tc, p)
		}
	}
}
func TestInterviewRehabExtraAndStagePriority(t *testing.T) {
	a := Interview{}
	p := interviewProgress(t, a, 365)
	p.Stage = 11
	start := p.LearningStartedAt
	p = answerInterview(t, a, p, Wrong, start)
	main := *p.StageReviewAt
	if main.Sub(start) != 50*24*time.Hour {
		t.Fatal("rollback gap", p)
	}
	p = answerInterview(t, a, p, Correct, start)
	if p.RehabStep != 2 || p.RehabReviewAt.Sub(start) != 2*24*time.Hour {
		t.Fatal(p)
	}
	p = answerInterview(t, a, p, Correct, start.AddDate(0, 0, 2))
	if p.ExtraReviewAt == nil || p.ExtraReviewAt.Sub(start) != 12*24*time.Hour || !p.StageReviewAt.Equal(main) {
		t.Fatal(p)
	}
	p = answerInterview(t, a, p, Correct, main)
	if p.RehabActive || p.ExtraReviewAt != nil || p.Stage != 10 {
		t.Fatal("stage must supersede extra", p)
	}
}
func TestDeadlineOverridesRollbackAndFinalWrongStaysDue(t *testing.T) {
	a := Interview{}
	p := interviewProgress(t, a, 365)
	p.Stage = 11
	p = answerInterview(t, a, p, Wrong, p.LearningStartedAt)
	target := *p.TargetAt
	p = answerInterview(t, a, p, Wrong, target)
	if p.Stage != 12 || p.RehabActive || p.CompletedAt != nil {
		t.Fatal(p)
	}
	p = answerInterview(t, a, p, Correct, target)
	if p.CompletedAt == nil || p.NextReviewAt() != nil {
		t.Fatal(p)
	}
}
