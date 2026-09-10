package model

import "time"

// ReviewLead prevents daily review times from drifting later. Apply once when
// scheduling a future review, never when reading or rendering a saved date.
const ReviewLead = 30 * time.Minute

func EarlierReview(anchor, scheduled time.Time) time.Time {
	if !scheduled.After(anchor) {
		return scheduled
	}
	next := scheduled.Add(-ReviewLead)
	if next.Before(anchor) {
		return anchor
	}
	return next
}
