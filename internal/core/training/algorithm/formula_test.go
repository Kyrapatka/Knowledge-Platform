package algorithm

import (
	"reflect"
	"testing"
	"time"

	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/google/uuid"
)

func formulaProgress(t *testing.T) model.UserMaterialProgress {
	t.Helper()
	p, err := model.NewProgress(model.NewProgressParams{UserID: uuid.New(), MaterialID: uuid.New(), Track: model.ProgressTrackDefault, AlgorithmKey: "formula_adaptive", AlgorithmVersion: 1, Now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func formulaAnswer(t *testing.T, p model.UserMaterialProgress, action Action, now time.Time, d material.Difficulty) model.UserMaterialProgress {
	t.Helper()
	kind, _ := p.DueReview(now)
	next, err := (Formula{}).Apply(Input{Progress: p, Action: action, Kind: kind, Difficulty: d, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err = next.Validate(); err != nil {
		t.Fatal(err)
	}
	return next
}
func TestFormulaScheduleMasteryAndMaintenance(t *testing.T) {
	for _, tc := range []struct {
		difficulty material.Difficulty
		required   int
	}{{material.DifficultyEasy, 1}, {material.DifficultyMedium, 2}, {material.DifficultyHard, 3}} {
		p := formulaProgress(t)
		start := p.LearningStartedAt
		for stage, day := range []int{0, 1, 3, 7, 14, 30, 60, 120, 240, 365} {
			if p.Stage != stage+1 || !p.StageReviewAt.Equal(start.AddDate(0, 0, day)) {
				t.Fatal(stage, p)
			}
			now := *p.StageReviewAt
			for i := 0; i < tc.required; i++ {
				p = formulaAnswer(t, p, Correct, now, tc.difficulty)
				if i < tc.required-1 && (p.Stage != stage+1 || p.ConsecutiveCorrect != i+1) {
					t.Fatal("premature promotion", p)
				}
			}
		}
		if p.CompletedAt != nil || p.TargetAt != nil || p.Stage != 10 || !p.StageReviewAt.Equal(start.AddDate(0, 0, 730)) {
			t.Fatal("maintenance must repeat yearly", p)
		}
	}
}
func TestFormulaEarlyAndMiddleWrong(t *testing.T) {
	a := Formula{}
	for stage := 1; stage <= 6; stage++ {
		p := formulaProgress(t)
		p.Stage = stage
		p.ConsecutiveCorrect = 1
		p = formulaAnswer(t, p, Wrong, p.LearningStartedAt, material.DifficultyMedium)
		if p.Stage != stage || p.ConsecutiveCorrect != 0 || p.RehabActive {
			t.Fatal(p)
		}
		if stage >= 4 && a.Mode(p, model.ReviewStage) != model.PracticeFaded {
			t.Fatal("missing faded refresher")
		}
		p = formulaAnswer(t, p, Correct, p.LearningStartedAt, material.DifficultyMedium)
		if p.Stage != stage || p.ConsecutiveCorrect != 1 {
			t.Fatal(p)
		}
		if stage >= 4 && a.Mode(p, model.ReviewStage) != model.PracticeMixed {
			t.Fatal("support must fade after success")
		}
	}
}
func TestFormulaRecoveryAndIndependentFailure(t *testing.T) {
	p := formulaProgress(t)
	p.Stage = 8
	start := p.LearningStartedAt
	p = formulaAnswer(t, p, Wrong, start, material.DifficultyEasy)
	main := *p.StageReviewAt
	if main.Sub(start) != 60*24*time.Hour || (Formula{}).Mode(p, model.ReviewRehab) != model.PracticeWorked {
		t.Fatal(p)
	}
	p = formulaAnswer(t, p, Correct, start, material.DifficultyEasy)
	if !p.RehabReviewAt.Equal(start.AddDate(0, 0, 2)) || (Formula{}).Mode(p, model.ReviewRehab) != model.PracticeIndependent {
		t.Fatal(p)
	}
	failed := formulaAnswer(t, p, Wrong, *p.RehabReviewAt, material.DifficultyEasy)
	if failed.Stage != 7 || failed.RehabStep != 1 || failed.WrongCount != 2 || failed.StageReviewAt.Sub(*failed.StageLastReviewAt) != 30*24*time.Hour {
		t.Fatal(failed)
	}
	failed = formulaAnswer(t, failed, Correct, *failed.RehabReviewAt, material.DifficultyEasy)
	failed = formulaAnswer(t, failed, Correct, *failed.RehabReviewAt, material.DifficultyEasy)
	if failed.ExtraReviewAt != nil || failed.RehabActive {
		t.Fatal("a 30-day interval must not schedule an extra check", failed)
	}
	p = formulaAnswer(t, p, Correct, *p.RehabReviewAt, material.DifficultyEasy)
	if p.ExtraReviewAt == nil || !p.ExtraReviewAt.Equal(start.AddDate(0, 0, 12)) || !p.StageReviewAt.Equal(main) || p.ConsecutiveCorrect != 0 {
		t.Fatal(p)
	}
	p = formulaAnswer(t, p, Correct, main, material.DifficultyEasy)
	if p.Stage != 9 || p.RehabActive || p.ExtraReviewAt != nil {
		t.Fatal("Stage must supersede extra", p)
	}
}
func TestFormulaManualActionsAndDueValidation(t *testing.T) {
	p := formulaProgress(t)
	start := p.LearningStartedAt
	next := formulaAnswer(t, p, Advance, start, material.DifficultyEasy)
	if next.Stage != 2 || next.CorrectCount != 0 || next.WrongCount != 0 {
		t.Fatal(next)
	}
	original := next
	_, err := (Formula{}).Apply(Input{Progress: next, Kind: model.ReviewStage, Action: Correct, Difficulty: material.DifficultyEasy, Now: start})
	if err == nil || !reflect.DeepEqual(original, next) {
		t.Fatal("early answer or mutation")
	}
	next = formulaAnswer(t, next, Rollback, start, material.DifficultyEasy)
	main := *next.StageReviewAt
	next = formulaAnswer(t, next, SkipRehab, start, material.DifficultyEasy)
	if next.Stage != 1 || next.RehabActive || !next.StageReviewAt.Equal(main) || next.WrongCount != 0 {
		t.Fatal(next)
	}
}
