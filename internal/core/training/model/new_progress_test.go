package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMaterialsStartTheirOwnHorizon(t *testing.T) {
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	params := NewProgressParams{UserID: uuid.New(), MaterialID: uuid.New(), Track: ProgressTrackLongTerm,
		AlgorithmKey: "interview_long_term", AlgorithmVersion: 1, HorizonDays: 150, Now: start}
	first, err := NewProgress(params)
	if err != nil {
		t.Fatal(err)
	}
	params.MaterialID = uuid.New()
	params.Now = start.AddDate(0, 0, 100)
	second, err := NewProgress(params)
	if err != nil {
		t.Fatal(err)
	}
	if second.TargetAt.Sub(first.TargetAt.UTC()) != 100*24*time.Hour {
		t.Fatal("late material inherited an earlier target")
	}
	if second.TargetAt.Sub(second.LearningStartedAt) != 150*24*time.Hour {
		t.Fatal("late material lost part of its horizon")
	}
	if second.Stage != 1 || second.StageLastReviewAt != nil {
		t.Fatal("new material inherited learning state")
	}
	assertReviewAt(t, second.NextReviewAt(), &params.Now)
}

func TestProgressScopeValidation(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	plan := uuid.New()
	params := NewProgressParams{UserID: uuid.New(), MaterialID: uuid.New(), Track: ProgressTrackCram,
		AlgorithmKey: "interview_cram", AlgorithmVersion: 1, HorizonDays: 5, Now: now}
	if _, err := NewProgress(params); err == nil {
		t.Fatal("CRAM accepted without PlanID")
	}
	params.PlanID = &plan
	p, err := NewProgress(params)
	if err != nil {
		t.Fatal(err)
	}
	p.Track = ProgressTrackDefault
	if p.Validate() == nil {
		t.Fatal("persistent progress accepted PlanID")
	}
	p.Track = ProgressTrackCram
	p.StartRehab(now)
	p.RehabStep = 3
	if p.Validate() == nil {
		t.Fatal("third rehab day accepted")
	}
}
