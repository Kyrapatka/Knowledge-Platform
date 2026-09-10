package model

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRecoveryTwoLearningDaysAndSeparateFollowUp(t *testing.T) {
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	stage := start.AddDate(0, 0, 40)
	p := UserMaterialProgress{Stage: 8, StageLastReviewAt: &start, StageReviewAt: &stage, ConsecutiveCorrect: 1, ConsecutiveWrong: 2}
	p.StartRehab(start)
	answer := func(at time.Time, kind ReviewKind, correct bool) {
		t.Helper()
		if err := p.ApplyRecovery(kind, correct, 2, at); err != nil {
			t.Fatal(err)
		}
	}
	answer(start, ReviewRehab, true)
	if p.RehabStep != 1 || p.RehabConsecutiveCorrect != 1 {
		t.Fatal("first day mastery advanced too early")
	}
	answer(start, ReviewRehab, false)
	answer(start, ReviewRehab, true)
	answer(start, ReviewRehab, true)
	day2 := start.AddDate(0, 0, 2).Add(-30 * time.Minute)
	assertReviewAt(t, p.RehabReviewAt, &day2)
	before := p
	if err := p.ApplyRecovery(ReviewRehab, true, 2, start.AddDate(0, 0, 1)); !errors.Is(err, ErrReviewNotDue) {
		t.Fatal("rest day accepted", err)
	}
	if !reflect.DeepEqual(before, p) {
		t.Fatal("rejected answer changed state")
	}
	answer(day2, ReviewRehab, true)
	answer(day2, ReviewRehab, true)
	day12 := start.AddDate(0, 0, 12).Add(-time.Hour)
	if p.RehabActive {
		t.Fatal("rehab not completed")
	}
	assertReviewAt(t, p.ExtraReviewAt, &day12)
	assertReviewAt(t, p.NextReviewAt(), &day12)
	answer(day12, ReviewExtra, true)
	assertReviewAt(t, p.NextReviewAt(), &stage)
	assertReviewAt(t, p.StageLastReviewAt, &start)
	if p.Stage != 8 || p.ConsecutiveCorrect != 1 || p.ConsecutiveWrong != 2 {
		t.Fatal("recovery changed Stage mastery")
	}
	if p.CorrectCount != 6 || p.WrongCount != 1 {
		t.Fatalf("wrong totals: %+v", p)
	}
}

func TestExtraReviewThreshold(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	for _, days := range []int{15, 30, 31, 40} {
		t.Run(time.Duration(days).String(), func(t *testing.T) {
			stage := now.AddDate(0, 0, days)
			p := UserMaterialProgress{StageLastReviewAt: &now, StageReviewAt: &stage}
			p.StartRehab(now)
			if err := p.ApplyRecovery(ReviewRehab, true, 1, now); err != nil {
				t.Fatal(err)
			}
			if err := p.ApplyRecovery(ReviewRehab, true, 1, now.AddDate(0, 0, 2)); err != nil {
				t.Fatal(err)
			}
			if (p.ExtraReviewAt != nil) != (days > 30) {
				t.Fatal("incorrect >30-day threshold")
			}
		})
	}
}

func TestStageSupersedesOverdueRecovery(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	earlier := now.AddDate(0, 0, -2)
	for _, kind := range []ReviewKind{ReviewRehab, ReviewExtra} {
		p := UserMaterialProgress{StageReviewAt: &now}
		if kind == ReviewRehab {
			p.StartRehab(earlier)
		} else {
			p.ExtraReviewAt = &earlier
		}
		assertReviewAt(t, p.NextReviewAt(), &earlier)
		if got, ok := p.DueReview(now); !ok || got != ReviewStage {
			t.Fatal("Stage priority lost")
		}
		if err := p.ApplyRecovery(kind, true, 1, now); !errors.Is(err, ErrReviewNotDue) {
			t.Fatal("stale recovery accepted")
		}
		p.CancelRecovery()
		assertReviewAt(t, p.NextReviewAt(), &now)
		if p.RehabActive || p.ExtraReviewAt != nil {
			t.Fatal("recovery not cancelled")
		}
	}
}
