package algorithm

import (
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"time"
)

type ChangeMode string

const (
	KeepStage  ChangeMode = "keep"
	ResetStage ChangeMode = "reset"
	SetStage   ChangeMode = "set"
)

func TrackFor(a Algorithm) model.ProgressTrack {
	if interview, ok := a.(Interview); ok {
		if interview.Cram {
			return model.ProgressTrackCram
		}
		return model.ProgressTrackLongTerm
	}
	return model.ProgressTrackDefault
}

// ChangeProgress preserves historical answer counts and difficulty overrides.
// KEEP/SET anchor the new interval to the last Stage review; RESET starts now.
// Clearing mastery/recovery prevents carrying requirements from another algorithm.
func ChangeProgress(p model.UserMaterialProgress, a Algorithm, mode ChangeMode, stage int, horizon *int, now time.Time) (model.UserMaterialProgress, error) {
	original := p
	if now.IsZero() || TrackFor(a) != p.Track {
		return original, fmt.Errorf("changing tracks requires a new plan")
	}
	if mode != KeepStage && mode != ResetStage && mode != SetStage {
		return original, fmt.Errorf("invalid change mode")
	}
	if mode != SetStage && stage != 0 {
		return original, fmt.Errorf("stage is only valid for SET")
	}
	if mode == SetStage && stage < 1 {
		return original, fmt.Errorf("SET requires a positive stage")
	}
	p.AlgorithmKey, p.AlgorithmVersion = a.Key(), a.Version()
	now = now.UTC()
	oldHorizon := 0
	if p.TargetAt != nil {
		oldHorizon = int(p.TargetAt.Sub(p.LearningStartedAt) / (24 * time.Hour))
	}
	if mode == ResetStage {
		p.Stage = 1
		p.LearningStartedAt = now
		p.StageLastReviewAt = nil
		p.CompletedAt = nil
	}
	if mode == SetStage {
		p.Stage = stage
		p.CompletedAt = nil
	}
	if p.Track != model.ProgressTrackDefault {
		if horizon != nil {
			oldHorizon = *horizon
		}
		target := p.LearningStartedAt.AddDate(0, 0, oldHorizon)
		p.TargetAt = &target
	} else if horizon != nil && *horizon != 0 {
		return original, fmt.Errorf("continuous algorithms have no horizon")
	}
	interval, err := ChangeInterval(a, p)
	if err != nil {
		return original, err
	}
	p.CancelRecovery()
	p.ConsecutiveCorrect, p.ConsecutiveWrong = 0, 0
	p.UpdatedAt = now
	if p.CompletedAt != nil {
		p.StageReviewAt = nil
		return p, nil
	}
	anchor := p.LearningStartedAt
	if p.StageLastReviewAt != nil {
		anchor = *p.StageLastReviewAt
	}
	next := anchor.Add(interval)
	if mode == ResetStage || (p.Stage == 1 && p.StageLastReviewAt == nil) {
		next = now
	}
	if interview, ok := a.(Interview); ok {
		schedule, _ := interview.Schedule(p)
		if p.Stage == len(schedule) || next.After(*p.TargetAt) {
			next = *p.TargetAt
		}
	}
	p.StageReviewAt = &next
	return p, nil
}

func ChangeInterval(a Algorithm, p model.UserMaterialProgress) (time.Duration, error) {
	switch a := a.(type) {
	case English:
		if p.Stage < 1 || p.Stage > len(englishIntervals) {
			return 0, fmt.Errorf("stage outside English schedule; choose RESET or SET")
		}
		return time.Duration(englishIntervals[p.Stage-1]) * 24 * time.Hour, nil
	case Formula:
		if p.Stage < 1 || p.Stage > len(formulaDays) {
			return 0, fmt.Errorf("stage outside Formula schedule; choose RESET or SET")
		}
		days := 0
		if p.Stage > 1 {
			days = formulaDays[p.Stage-1] - formulaDays[p.Stage-2]
		}
		return time.Duration(days) * 24 * time.Hour, nil
	case Interview:
		schedule, err := a.Schedule(p)
		if err != nil {
			return 0, err
		}
		if p.Stage < 1 || p.Stage > len(schedule) {
			return 0, fmt.Errorf("stage outside Interview schedule; choose RESET or SET")
		}
		interval := schedule[p.Stage-1]
		if p.Stage > 1 {
			interval -= schedule[p.Stage-2]
		}
		return interval, nil
	default:
		return 0, fmt.Errorf("algorithm does not support change")
	}
}
