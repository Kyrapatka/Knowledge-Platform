package algorithm

import (
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"reflect"
	"testing"
	"time"
)

func TestChangeKeepsElapsedTimeAndHistory(t *testing.T) {
	p := formulaProgress(t)
	p.AlgorithmKey = "english_basic"
	p.Stage = 5
	anchor := p.LearningStartedAt
	p.StageLastReviewAt = &anchor
	p.CorrectCount = 17
	p.WrongCount = 4
	p.ConsecutiveCorrect = 2
	p.StartRehab(anchor)
	before := p
	now := anchor.AddDate(0, 0, 9)
	next, err := ChangeProgress(p, Formula{}, KeepStage, 0, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Stage != 5 || !next.StageReviewAt.Equal(anchor.AddDate(0, 0, 7).Add(-30*time.Minute)) || next.CorrectCount != 17 || next.WrongCount != 4 || next.ConsecutiveCorrect != 0 || next.RehabActive {
		t.Fatal(next)
	}
	if _, due := next.DueReview(now); !due {
		t.Fatal("shorter interval must already be due")
	}
	if !reflect.DeepEqual(p, before) {
		t.Fatal("mutated input")
	}
	next, err = ChangeProgress(next, English{}, SetStage, 8, nil, now)
	if err != nil || next.Stage != 8 || !next.StageReviewAt.Equal(anchor.AddDate(0, 0, 46).Add(-30*time.Minute)) {
		t.Fatal(next, err)
	}
	next, err = ChangeProgress(next, English{Adaptive: true}, ResetStage, 0, nil, now)
	if err != nil || next.Stage != 1 || next.StageLastReviewAt != nil || !next.StageReviewAt.Equal(now) || next.CorrectCount != 17 {
		t.Fatal(next, err)
	}
}
func TestChangeFiniteHorizonAndTrackBoundaries(t *testing.T) {
	a := Interview{}
	p := interviewProgress(t, a, 150)
	p.Stage = 8
	anchor := p.LearningStartedAt.AddDate(0, 0, 60)
	p.StageLastReviewAt = &anchor
	horizon := 180
	now := anchor.AddDate(0, 0, 2)
	next, err := ChangeProgress(p, a, KeepStage, 0, &horizon, now)
	if err != nil || next.TargetAt.Sub(p.LearningStartedAt) != 180*24*time.Hour || !next.StageReviewAt.Equal(anchor.AddDate(0, 0, 35).Add(-30*time.Minute)) {
		t.Fatal(next, err)
	}
	next, err = ChangeProgress(p, a, ResetStage, 0, &horizon, now)
	if err != nil || next.TargetAt.Sub(now) != 180*24*time.Hour || !next.StageReviewAt.Equal(now) {
		t.Fatal(next, err)
	}
	if _, err = ChangeProgress(p, Interview{Cram: true}, KeepStage, 0, nil, now); err == nil {
		t.Fatal("tracks cannot be converted")
	}
	p.AlgorithmKey = "english_basic"
	p.Track = model.ProgressTrackDefault
	p.TargetAt = nil
	p.Stage = 11
	if _, err = ChangeProgress(p, Formula{}, KeepStage, 0, nil, now); err == nil {
		t.Fatal("invalid stage must not be silently clamped")
	}
}
