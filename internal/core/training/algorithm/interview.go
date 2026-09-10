package algorithm

import (
	"fmt"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"time"
)

// Interview keeps the deadline on each material, including persistent progress
// resumed by a later plan. Plans do not override an existing material's horizon.
type Interview struct{ Cram bool }

func (a Interview) Key() string {
	if a.Cram {
		return "interview_cram"
	}
	return "interview_long_term"
}
func (Interview) Version() int { return 1 }
func (Interview) RequiredCorrect(d material.Difficulty) (int, error) {
	switch d {
	case material.DifficultyEasy:
		return 1, nil
	case material.DifficultyMedium:
		return 2, nil
	case material.DifficultyHard:
		return 3, nil
	}
	return 0, fmt.Errorf("invalid difficulty")
}
func (a Interview) Schedule(p model.UserMaterialProgress) ([]time.Duration, error) {
	if p.TargetAt == nil {
		return nil, fmt.Errorf("missing target")
	}
	duration := p.TargetAt.Sub(p.LearningStartedAt)
	horizon := int(duration / (24 * time.Hour))
	if duration != time.Duration(horizon)*24*time.Hour {
		return nil, fmt.Errorf("horizon must be whole days")
	}
	if a.Cram {
		return CramSchedule(horizon)
	}
	days, err := LongTermDays(horizon)
	if err != nil {
		return nil, err
	}
	offsets := make([]time.Duration, len(days))
	for i, d := range days {
		offsets[i] = time.Duration(d) * 24 * time.Hour
	}
	return offsets, nil
}

// Day 1 is the initial session. Subsequent gaps follow the marker table;
// the final review is always anchored to the individual target.
func CramSchedule(horizon int) ([]time.Duration, error) {
	if horizon < 1 || horizon > 7 {
		return nil, fmt.Errorf("CRAM horizon must be 1..7 days")
	}
	if horizon <= 2 {
		return []time.Duration{0, 8 * time.Hour, time.Duration(horizon) * 24 * time.Hour}, nil
	}
	tables := map[int][]int{3: {1, 2, 3}, 4: {1, 2, 4}, 5: {1, 2, 3, 5}, 6: {1, 2, 4, 6}, 7: {1, 2, 3, 5, 7}}
	days := tables[horizon]
	result := make([]time.Duration, len(days))
	for i, d := range days {
		result[i] = time.Duration(d) * 24 * time.Hour
	}
	return result, nil
}
func (a Interview) IsFinal(p model.UserMaterialProgress, now time.Time) bool {
	schedule, err := a.Schedule(p)
	return err == nil && p.CompletedAt == nil && (p.Stage == len(schedule) || !p.TargetAt.After(now))
}
func (a Interview) Apply(in Input) (model.UserMaterialProgress, error) {
	p := in.Progress
	track := model.ProgressTrackLongTerm
	if a.Cram {
		track = model.ProgressTrackCram
	}
	schedule, err := a.Schedule(p)
	if err != nil {
		return in.Progress, err
	}
	if p.AlgorithmKey != a.Key() || p.AlgorithmVersion != 1 || p.Track != track || p.Stage < 1 || p.Stage > len(schedule) || p.CompletedAt != nil || in.Now.IsZero() {
		return in.Progress, fmt.Errorf("incompatible Interview progress")
	}
	if in.Action == SkipRehab {
		p.CancelRecovery()
		p.UpdatedAt = in.Now.UTC()
		return p, nil
	}
	if !a.Cram && (in.Action == Advance || in.Action == Rollback) {
		if (in.Action == Advance && p.Stage == len(schedule)) || (in.Action == Rollback && p.Stage == 1) {
			return in.Progress, fmt.Errorf("stage boundary")
		}
		p.CancelRecovery()
		if in.Action == Advance {
			p.Stage++
		} else {
			p.Stage--
		}
		p.ConsecutiveCorrect, p.ConsecutiveWrong = 0, 0
		p.UpdatedAt = in.Now.UTC()
		scheduleInterview(&p, schedule, in.Now)
		if in.Action == Rollback {
			p.StartRehab(in.Now)
		}
		return p, nil
	}
	if in.Action != Correct && in.Action != Wrong {
		return in.Progress, fmt.Errorf("Interview stages require answers; use start-final for early final review")
	}
	kind, due := p.DueReview(in.Now)
	if !due || kind != in.Kind {
		return in.Progress, model.ErrReviewNotDue
	}
	difficulty := p.EffectiveDifficulty(in.Difficulty)
	required, err := a.RequiredCorrect(difficulty)
	if err != nil {
		return in.Progress, err
	}
	p.UpdatedAt = in.Now.UTC()
	if kind != model.ReviewStage {
		err = p.ApplyRecovery(kind, in.Action == Correct, required, in.Now)
		return p, err
	}
	p.CancelRecovery()
	final := a.IsFinal(p, in.Now)
	if final && p.Stage != len(schedule) {
		p.Stage = len(schedule)
		p.ConsecutiveCorrect = 0
		p.ConsecutiveWrong = 0
	}
	if in.Action == Correct {
		p.CorrectCount++
		p.ConsecutiveWrong = 0
		p.ConsecutiveCorrect++
		if p.ConsecutiveCorrect < required {
			return p, nil
		}
		p.ConsecutiveCorrect = 0
		if final {
			now := in.Now.UTC()
			p.CompletedAt = &now
			p.StageLastReviewAt = &now
			p.StageReviewAt = nil
			return p, nil
		}
		p.Stage++
		scheduleInterview(&p, schedule, in.Now)
	} else {
		p.WrongCount++
		p.ConsecutiveWrong++
		p.ConsecutiveCorrect = 0
		if difficulty == material.DifficultyEasy && p.ConsecutiveWrong >= 3 {
			medium := material.DifficultyMedium
			p.DifficultyOverride = &medium
		}
		if !a.Cram && !final {
			gap := schedule[p.Stage-1]
			if p.Stage > 1 {
				gap -= schedule[p.Stage-2]
			}
			rollback := 1
			if gap > 30*24*time.Hour {
				rollback = 2
			}
			p.Stage -= rollback
			if p.Stage < 1 {
				p.Stage = 1
			}
			scheduleInterview(&p, schedule, in.Now)
			p.StartRehab(in.Now)
		}
	}
	return p, nil
}
func scheduleInterview(p *model.UserMaterialProgress, schedule []time.Duration, now time.Time) {
	anchor := now.UTC()
	gap := schedule[p.Stage-1]
	if p.Stage > 1 {
		gap -= schedule[p.Stage-2]
	}
	next := anchor.Add(gap)
	if p.Stage == len(schedule) || next.After(*p.TargetAt) {
		next = *p.TargetAt
	}
	next = model.EarlierReview(anchor, next)
	p.StageLastReviewAt = &anchor
	p.StageReviewAt = &next
}
