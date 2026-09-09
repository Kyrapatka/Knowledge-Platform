package algorithm

import (
	"fmt"
	"time"

	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
)

type Formula struct{}

func (Formula) Key() string  { return "formula_adaptive" }
func (Formula) Version() int { return 1 }
func (Formula) RequiredCorrect(d material.Difficulty) (int, error) {
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

var formulaDays = [...]int{0, 1, 3, 7, 14, 30, 60, 120, 240, 365}

// FormulaMode controls the support shown before the learner attempts a task.
// Recovery counters remain independent of normal stage mastery.
func (Formula) Mode(p model.UserMaterialProgress, kind model.ReviewKind) model.PracticeMode {
	if kind == model.ReviewExtra || (kind == model.ReviewRehab && p.RehabStep == 2) {
		return model.PracticeIndependent
	}
	if kind == model.ReviewRehab {
		return model.PracticeWorked
	}
	if p.Stage >= 4 && p.Stage <= 6 && p.ConsecutiveWrong > 0 {
		return model.PracticeFaded
	}
	switch p.Stage {
	case 1:
		return model.PracticeWorked
	case 2:
		return model.PracticeFaded
	case 3:
		return model.PracticeIndependent
	case 10:
		return model.PracticeMaintenance
	}
	return model.PracticeMixed
}

func (a Formula) Apply(in Input) (model.UserMaterialProgress, error) {
	p := in.Progress
	if p.AlgorithmKey != a.Key() || p.AlgorithmVersion != 1 || p.Track != model.ProgressTrackDefault || p.Stage < 1 || p.Stage > len(formulaDays) || p.CompletedAt != nil || in.Now.IsZero() {
		return in.Progress, fmt.Errorf("incompatible Formula progress")
	}
	required, err := a.RequiredCorrect(p.EffectiveDifficulty(in.Difficulty))
	if err != nil {
		return in.Progress, err
	}
	p.UpdatedAt = in.Now.UTC()
	if in.Action == SkipRehab {
		p.CancelRecovery()
		return p, nil
	}
	if in.Action != Correct && in.Action != Wrong && in.Action != Advance && in.Action != Rollback {
		return in.Progress, fmt.Errorf("unknown action")
	}
	if in.Action == Correct || in.Action == Wrong {
		kind, due := p.DueReview(in.Now)
		if !due || kind != in.Kind {
			return in.Progress, model.ErrReviewNotDue
		}
		if kind != model.ReviewStage {
			// Failing the independent recovery check lowers the stage once and starts
			// a new refresher. Wrong worked attempts only reset recovery mastery.
			if in.Action == Wrong && ((kind == model.ReviewRehab && p.RehabStep == 2) || kind == model.ReviewExtra) {
				p.WrongCount++
				if p.Stage > 1 {
					p.Stage--
				}
				p.ConsecutiveCorrect, p.ConsecutiveWrong = 0, 0
				scheduleFormula(&p, in.Now, false)
				p.StartRehab(in.Now)
				return p, nil
			}
			err = p.ApplyRecovery(kind, in.Action == Correct, required, in.Now)
			return p, err
		}
	}
	p.CancelRecovery()
	switch in.Action {
	case Correct:
		p.CorrectCount++
		p.ConsecutiveWrong = 0
		p.ConsecutiveCorrect++
		if p.ConsecutiveCorrect < required {
			return p, nil
		}
		maintenance := p.Stage == len(formulaDays)
		if !maintenance {
			p.Stage++
		}
		p.ConsecutiveCorrect = 0
		scheduleFormula(&p, in.Now, maintenance)
	case Wrong:
		p.WrongCount++
		p.ConsecutiveWrong++
		p.ConsecutiveCorrect = 0
		if p.Stage >= 7 {
			scheduleFormula(&p, in.Now, false)
			p.StartRehab(in.Now)
		}
	case Advance:
		if p.Stage == len(formulaDays) {
			return in.Progress, fmt.Errorf("already at last stage")
		}
		p.Stage++
		p.ConsecutiveCorrect, p.ConsecutiveWrong = 0, 0
		scheduleFormula(&p, in.Now, false)
	case Rollback:
		if p.Stage == 1 {
			return in.Progress, fmt.Errorf("already at first stage")
		}
		p.Stage--
		p.ConsecutiveCorrect, p.ConsecutiveWrong = 0, 0
		scheduleFormula(&p, in.Now, false)
		p.StartRehab(in.Now)
	}
	return p, nil
}

func scheduleFormula(p *model.UserMaterialProgress, now time.Time, maintenance bool) {
	gap := 1 // A manual rollback to Stage 1 leaves time for its refresher.
	if p.Stage > 1 {
		gap = formulaDays[p.Stage-1] - formulaDays[p.Stage-2]
	}
	if maintenance {
		gap = 365
	}
	anchor := now.UTC()
	next := anchor.AddDate(0, 0, gap)
	p.StageLastReviewAt, p.StageReviewAt = &anchor, &next
}
