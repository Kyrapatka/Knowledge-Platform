package algorithm

import (
	"fmt"
	"time"
)

var longTermDays = [][]int{
	{1, 2, 3, 5, 7}, {1, 2, 4, 7, 10}, {1, 2, 4, 8, 14},
	{1, 2, 4, 8, 14, 21}, {1, 2, 4, 8, 15, 22, 30},
	{1, 2, 4, 8, 18, 30, 45}, {1, 2, 5, 10, 20, 35, 50, 60},
	{1, 2, 5, 10, 20, 35, 55, 75, 90}, {1, 2, 5, 10, 20, 40, 65, 90, 120},
	{1, 2, 5, 10, 20, 40, 70, 105, 140, 180},
	{1, 2, 5, 10, 20, 40, 70, 110, 160, 215, 270},
	{1, 2, 5, 10, 20, 40, 70, 110, 160, 220, 290, 365},
}

// LongTermDays returns absolute day markers, NOT intervals. Ties prefer the
// shorter template. Only the final marker is replaced with the user's horizon.
func LongTermDays(horizon int) ([]int, error) {
	if horizon < 7 || horizon > 365 {
		return nil, fmt.Errorf("long-term horizon must be between 7 and 365 days")
	}
	best := longTermDays[0]
	for _, days := range longTermDays[1:] {
		if abs(days[len(days)-1]-horizon) < abs(best[len(best)-1]-horizon) {
			best = days
		}
	}
	result := append([]int(nil), best...)
	result[len(result)-1] = horizon
	return result, nil
}

// StageInterval computes the gap before a one-based stage. For example the
// Day 90 stage after Day 65 has a 25-day interval and is not a late (>30) gap.
func StageInterval(days []int, stage int) (time.Duration, error) {
	if stage < 1 || stage > len(days) {
		return 0, fmt.Errorf("stage outside schedule")
	}
	previous := 0
	if stage > 1 {
		previous = days[stage-2]
	}
	if days[stage-1] < previous {
		return 0, fmt.Errorf("schedule must be nondecreasing")
	}
	return time.Duration(days[stage-1]-previous) * 24 * time.Hour, nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
