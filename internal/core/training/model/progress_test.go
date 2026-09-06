package model

import (
	"reflect"
	"testing"
	"time"
)

func TestRehabPreservesStageSchedule(t *testing.T) {
	day0 := time.Date(2026, time.September, 5, 12, 0, 0, 0, time.UTC)
	stageAt := day0.AddDate(0, 0, 15)
	rehabAt := day0.AddDate(0, 0, 10)

	for _, tc := range []struct {
		name  string
		endAt time.Time
	}{
		{name: "completed on day 10", endAt: rehabAt},
		{name: "skipped immediately", endAt: day0},
		{name: "completed after Stage became overdue", endAt: day0.AddDate(0, 0, 20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := UserMaterialProgress{
				Stage: 8, StageLastReviewAt: &day0, StageReviewAt: &stageAt,
				CorrectCount: 12, WrongCount: 3,
				ConsecutiveCorrect: 1, ConsecutiveWrong: 2,
			}
			before := p
			p.ScheduleRehab(1, rehabAt)
			assertReviewAt(t, p.NextReviewAt(), &rehabAt)

			// A later rehab step must not hide an earlier Stage review.
			laterRehab := stageAt.AddDate(0, 0, 2)
			p.ScheduleRehab(2, laterRehab)
			assertReviewAt(t, p.NextReviewAt(), &stageAt)

			p.EndRehab()
			if !reflect.DeepEqual(p, before) {
				t.Fatalf("ending rehab changed non-rehab state: got %+v, want %+v", p, before)
			}
			assertReviewAt(t, p.NextReviewAt(), &stageAt)
			if got, want := p.NextReviewAt().Sub(tc.endAt), stageAt.Sub(tc.endAt); got != want {
				t.Fatalf("remaining Stage wait = %v, want %v", got, want)
			}
			p.EndRehab()
			if !reflect.DeepEqual(p, before) {
				t.Fatal("ending rehab twice must be harmless")
			}
		})
	}
}

func TestNextReviewAt(t *testing.T) {
	early := time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC)
	late := early.AddDate(0, 0, 5)
	for _, tc := range []struct {
		name   string
		stage  *time.Time
		active bool
		rehab  *time.Time
		want   *time.Time
	}{
		{name: "unscheduled"},
		{name: "Stage only", stage: &late, want: &late},
		{name: "rehab earlier", stage: &late, active: true, rehab: &early, want: &early},
		{name: "Stage earlier", stage: &early, active: true, rehab: &late, want: &early},
		{name: "same instant", stage: &early, active: true, rehab: &early, want: &early},
		{name: "rehab only", active: true, rehab: &early, want: &early},
		{name: "inactive rehab ignored", stage: &late, rehab: &early, want: &late},
		{name: "inactive rehab without Stage", rehab: &early},
		{name: "missing rehab date", stage: &late, active: true, want: &late},
		{name: "no dates in active rehab", active: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := UserMaterialProgress{StageReviewAt: tc.stage, RehabActive: tc.active, RehabReviewAt: tc.rehab}
			got := p.NextReviewAt()
			assertReviewAt(t, got, tc.want)
			if got != nil {
				*got = got.Add(time.Hour)
				assertReviewAt(t, p.NextReviewAt(), tc.want)
			}
		})
	}
}

func TestAlgorithmRescheduleUsesStageAnchorDuringRehab(t *testing.T) {
	anchor := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	stageAt := anchor.AddDate(0, 0, 20)
	p := UserMaterialProgress{Stage: 8, StageLastReviewAt: &anchor, StageReviewAt: &stageAt}
	rehabAt := anchor.AddDate(0, 0, 10)
	p.ScheduleRehab(1, rehabAt)
	p.EndRehab()

	// The future algorithm-change command must use this anchor, not rehab time.
	newStageAt := p.StageLastReviewAt.AddDate(0, 0, 15)
	p.StageReviewAt = &newStageAt
	want := time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)
	assertReviewAt(t, p.NextReviewAt(), &want)
	assertReviewAt(t, p.StageLastReviewAt, &anchor)
}

func assertReviewAt(t *testing.T, got, want *time.Time) {
	t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Fatalf("review date = %v, want %v", got, want)
		}
		return
	}
	if !got.Equal(*want) {
		t.Fatalf("review date = %v, want %v", *got, *want)
	}
}
