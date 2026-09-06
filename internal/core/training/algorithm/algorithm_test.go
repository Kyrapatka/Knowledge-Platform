package algorithm

import (
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"reflect"
	"testing"
	"time"
)

func TestLongTermSchedules(t *testing.T) {
	for _, tc := range []struct {
		target int
		want   []int
	}{
		{48, []int{1, 2, 4, 8, 18, 30, 48}},
		{53, []int{1, 2, 5, 10, 20, 35, 50, 53}},
		{12, []int{1, 2, 4, 7, 12}},
	} {
		got, err := LongTermDays(tc.target)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("target %d: %v %v", tc.target, got, err)
		}
	}
	for target := 7; target <= 365; target++ {
		days, err := LongTermDays(target)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(days); i++ {
			if days[i] <= days[i-1] {
				t.Fatalf("non-increasing schedule for %d: %v", target, days)
			}
		}
	}
	days, _ := LongTermDays(120)
	gap, err := StageInterval(days, 8)
	if err != nil || gap != 25*24*time.Hour {
		t.Fatalf("90 minus 65 must be 25 days, got %v %v", gap, err)
	}
}

func TestEnglishMasteryAndRecovery(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	for _, adaptive := range []bool{false, true} {
		a := English{Adaptive: adaptive}
		p := model.UserMaterialProgress{AlgorithmKey: a.Key(), AlgorithmVersion: 1, Track: model.ProgressTrackDefault, Stage: 8, StageReviewAt: &now}
		in := Input{Progress: p, Difficulty: material.DifficultyHard, Kind: model.ReviewStage, Action: Wrong, Now: now}
		p, err := a.Apply(in)
		if err != nil {
			t.Fatal(err)
		}
		stageAt := *p.StageReviewAt
		required := 3
		if adaptive {
			required = 5
		}
		for _, day := range []int{0, 2} {
			for i := 0; i < required; i++ {
				in.Progress = p
				in.Kind = model.ReviewRehab
				in.Action = Correct
				in.Now = now.AddDate(0, 0, day)
				p, err = a.Apply(in)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		if p.Stage != 8 || p.ConsecutiveCorrect != 0 || p.ExtraReviewAt == nil || !p.StageReviewAt.Equal(stageAt) {
			t.Fatalf("invalid recovery result %+v", p)
		}
		// Ordinary review cancels the overdue extra check on its first answer.
		in.Progress = p
		in.Kind = model.ReviewStage
		in.Now = stageAt
		p, err = a.Apply(in)
		if err != nil {
			t.Fatal(err)
		}
		if p.ExtraReviewAt != nil || p.ConsecutiveCorrect != 1 {
			t.Fatal("Stage did not supersede extra check")
		}
		for i := 1; i < required; i++ {
			in.Progress = p
			p, err = a.Apply(in)
			if err != nil {
				t.Fatal(err)
			}
		}
		if p.Stage != 9 || p.ConsecutiveCorrect != 0 {
			t.Fatal("Stage mastery failed")
		}
	}
}

func TestEnglishManualActionsDoNotCountAsAnswers(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	p := model.UserMaterialProgress{AlgorithmKey: "english_basic", AlgorithmVersion: 1, Track: model.ProgressTrackDefault, Stage: 4, CorrectCount: 8, WrongCount: 2}
	for _, action := range []Action{Advance, Rollback} {
		got, err := (English{}).Apply(Input{Progress: p, Difficulty: material.DifficultyMedium, Action: action, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		if got.CorrectCount != 8 || got.WrongCount != 2 {
			t.Fatal("manual action counted as answer")
		}
	}
}
