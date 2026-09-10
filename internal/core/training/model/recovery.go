package model

import (
	"errors"
	"time"
)

type ReviewKind string

const (
	ReviewStage ReviewKind = "stage"
	ReviewRehab ReviewKind = "rehab"
	ReviewExtra ReviewKind = "extra"
)

var ErrReviewNotDue = errors.New("review is not due or has been superseded")
var ErrInvalidMastery = errors.New("mastery requirement must be positive")

// DueReview chooses an action independently of the cached earliest timestamp.
// Merely reading the next action never cancels recovery: cancellation happens
// when the ordinary review is actually processed.
func (p UserMaterialProgress) DueReview(now time.Time) (ReviewKind, bool) {
	if p.StageReviewAt != nil && !p.StageReviewAt.After(now) {
		return ReviewStage, true
	}
	if p.RehabActive && p.RehabReviewAt != nil && !p.RehabReviewAt.After(now) {
		return ReviewRehab, true
	}
	if p.ExtraReviewAt != nil && !p.ExtraReviewAt.After(now) {
		return ReviewExtra, true
	}
	return "", false
}

func (p *UserMaterialProgress) StartRehab(now time.Time) {
	p.CancelRecovery()
	p.ScheduleRehab(1, now)
}

// ApplyRecovery handles only recovery answers. The caller chooses the mastery
// requirement using the algorithm. Wrong extra reviews restart reinforcement.
// Stage transitions and optimistic-lock version increments belong to the
// algorithm and repository respectively.
func (p *UserMaterialProgress) ApplyRecovery(kind ReviewKind, correct bool, required int, now time.Time) error {
	if required <= 0 {
		return ErrInvalidMastery
	}
	due, ok := p.DueReview(now)
	if !ok || due != kind || kind == ReviewStage {
		return ErrReviewNotDue
	}
	if correct {
		p.CorrectCount++
	} else {
		p.WrongCount++
	}
	if kind == ReviewExtra {
		p.ExtraReviewAt = nil
		if !correct {
			p.StartRehab(now)
		}
		return nil
	}
	if !correct {
		p.RehabConsecutiveCorrect = 0
		return nil
	}
	p.RehabConsecutiveCorrect++
	if p.RehabConsecutiveCorrect < required {
		return nil
	}
	if p.RehabStep == 1 {
		// Today is the first learning day, tomorrow is skipped, and the second
		// learning day is the day after tomorrow. Anchor to actual mastery.
		p.ScheduleRehab(2, EarlierReview(now.UTC(), now.UTC().AddDate(0, 0, 2)))
		return nil
	}
	p.EndRehab()
	if p.StageLastReviewAt != nil && p.StageReviewAt != nil &&
		p.StageReviewAt.Sub(*p.StageLastReviewAt) > 30*24*time.Hour {
		extra := EarlierReview(now.UTC(), now.UTC().AddDate(0, 0, 10))
		p.ExtraReviewAt = &extra
	}
	return nil
}
