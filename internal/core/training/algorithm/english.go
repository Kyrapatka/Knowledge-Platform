package algorithm

import (
	"fmt"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"time"
)

type Action string

const (
	Correct   Action = "correct"
	Wrong     Action = "wrong"
	Advance   Action = "advance"
	Rollback  Action = "rollback"
	SkipRehab Action = "skip_rehab"
)

type Input struct {
	Progress   model.UserMaterialProgress
	Difficulty material.Difficulty
	Kind       model.ReviewKind
	Action     Action
	Now        time.Time
}

type Algorithm interface {
	Key() string
	Version() int
	RequiredCorrect(material.Difficulty) (int, error)
	Apply(Input) (model.UserMaterialProgress, error)
}

// English implements the basic and difficulty-adaptive v1 rules. These values
// are stage cooldowns from the English table, unlike long-term day markers.
type English struct{ Adaptive bool }

func (a English) Key() string {
	if a.Adaptive {
		return "english_adaptive"
	}
	return "english_basic"
}
func (English) Version() int { return 1 }

var englishIntervals = [...]int{1, 1, 1, 6, 10, 16, 28, 46, 63, 100, 364}

func (a English) RequiredCorrect(difficulty material.Difficulty) (int, error) {
	if !difficulty.Valid() {
		return 0, fmt.Errorf("invalid difficulty")
	}
	if !a.Adaptive {
		return 3, nil
	}
	switch difficulty {
	case material.DifficultyEasy:
		return 3, nil
	case material.DifficultyMedium:
		return 4, nil
	default:
		return 5, nil
	}
}

func (a English) Apply(in Input) (model.UserMaterialProgress, error) {
	p := in.Progress
	if p.AlgorithmKey != a.Key() || p.AlgorithmVersion != a.Version() ||
		p.Track != model.ProgressTrackDefault || p.Stage < 1 || p.Stage > len(englishIntervals) || in.Now.IsZero() {
		return in.Progress, fmt.Errorf("progress is incompatible with algorithm")
	}
	difficulty := p.EffectiveDifficulty(in.Difficulty)
	required, err := a.RequiredCorrect(difficulty)
	if err != nil {
		return in.Progress, err
	}
	if in.Action == SkipRehab {
		p.CancelRecovery()
		p.UpdatedAt = in.Now
		return p, nil
	}
	if in.Action != Correct && in.Action != Wrong && in.Action != Advance && in.Action != Rollback {
		return in.Progress, fmt.Errorf("unknown action")
	}
	if in.Action == Correct || in.Action == Wrong {
		due, ok := p.DueReview(in.Now)
		if !ok || due != in.Kind {
			return in.Progress, model.ErrReviewNotDue
		}
		if in.Kind != model.ReviewStage {
			if err := p.ApplyRecovery(in.Kind, in.Action == Correct, required, in.Now); err != nil {
				return in.Progress, err
			}
			p.UpdatedAt = in.Now
			return p, nil
		}
	}
	p.CancelRecovery()
	p.UpdatedAt = in.Now
	switch in.Action {
	case Correct:
		p.CorrectCount++
		p.ConsecutiveWrong = 0
		p.ConsecutiveCorrect++
		if p.ConsecutiveCorrect < required {
			return p, nil
		}
		if p.Stage < len(englishIntervals) {
			p.Stage++
		}
		p.ConsecutiveCorrect = 0
		scheduleEnglish(&p, in.Now)
	case Wrong:
		p.WrongCount++
		p.ConsecutiveWrong++
		p.ConsecutiveCorrect = 0
		if p.Stage >= 7 {
			scheduleEnglish(&p, in.Now)
			p.StartRehab(in.Now)
		}
	case Advance:
		if p.Stage == len(englishIntervals) {
			return in.Progress, fmt.Errorf("already at final English stage")
		}
		p.Stage++
		p.ConsecutiveCorrect, p.ConsecutiveWrong = 0, 0
		scheduleEnglish(&p, in.Now)
	case Rollback:
		if p.Stage == 1 {
			return in.Progress, fmt.Errorf("already at first stage")
		}
		p.Stage--
		p.ConsecutiveCorrect, p.ConsecutiveWrong = 0, 0
		scheduleEnglish(&p, in.Now)
		p.StartRehab(in.Now)
	}
	return p, nil
}

func scheduleEnglish(p *model.UserMaterialProgress, now time.Time) {
	anchor := now.UTC()
	next := model.EarlierReview(anchor, anchor.AddDate(0, 0, englishIntervals[p.Stage-1]))
	p.StageLastReviewAt, p.StageReviewAt = &anchor, &next
}
