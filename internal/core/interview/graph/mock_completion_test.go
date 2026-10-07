package graph

import (
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func TestMockExhaustedRootsContinueUntilEligibleExhausted(t *testing.T) {
	for _, mode := range []string{"real", "balanced", "custom", "deep"} {
		for _, depth := range []int{1, 2, 3} {
			t.Run(mode+string(rune('0'+depth)), func(t *testing.T) {
				s, pool := mockBank(8, "go")
				s.Config.InterviewMode = mode
				s.Config.DepthLevel = depth
				s.Config.CustomWeights = map[string]float64{"go": 1}
				for i := range pool {
					pool[i].RootWeight = 0
				}
				pool[0].RootWeight = 8
				original := append([]Candidate(nil), pool...)
				first := SelectRoot(s, pool)
				if first.Candidate == nil || first.Candidate.MaterialID != pool[0].MaterialID || first.Reason != "root" {
					t.Fatal("normal root was not preferred")
				}
				// The plan is a distribution hint, not a completion budget.
				first.State.Plan.Slots = []string{"go"}
				first.State.Plan.Strategy.TargetRoots = 1
				seen := map[uuid.UUID]bool{first.Candidate.MaterialID: true}
				v := first
				for i := 0; i < 20 && v.Candidate != nil; i++ {
					v = SelectMetadata(v.State, *v.Candidate, "correct", pool, Catalog{})
					if v.Candidate != nil {
						if seen[v.Candidate.MaterialID] {
							t.Fatal("automatic repeat")
						}
						seen[v.Candidate.MaterialID] = true
						if v.Reason != "fallback_root" {
							t.Fatalf("wanted fallback, got %s", v.Reason)
						}
					}
				}
				if len(seen) != len(pool) || v.State.StopReason != "no_remaining_questions" || v.State.AnsweredQuestions != len(pool) {
					t.Fatalf("premature/nonterminating completion: %+v", v.State)
				}
				if !reflect.DeepEqual(original, pool) {
					t.Fatal("fallback changed persistent metadata")
				}
			})
		}
	}
}
func TestMockFallbackRunsOrdinaryFollowupsAndRespectsLimit(t *testing.T) {
	s, pool := mockBank(8, "go")
	for i := range pool {
		pool[i].RootWeight = 0
		pool[i].Concepts = []Link{{Slug: "shared", Role: "primary", Weight: 1}, {Slug: "shared", Role: "answer", Weight: 1}}
	}
	s.Config.InterviewMode = "deep"
	s.Config.DepthLevel = 3
	s.Config.QuestionLimit = 3
	v := SelectRoot(s, pool)
	if v.Candidate == nil || v.Reason != "fallback_root" {
		t.Fatal("fallback-only bank cannot start")
	}
	before := clone(v.State)
	remaining := 0
	for _, c := range pool {
		if Eligible(c, v.State) {
			remaining++
		}
	}
	if remaining != 7 {
		t.Fatal("candidates consumed before presentation")
	}
	_ = SelectRoot(s, pool)
	if !reflect.DeepEqual(before, v.State) {
		t.Fatal("selection mutated input")
	}
	next := SelectMetadata(v.State, *v.Candidate, "correct", pool, Catalog{})
	if next.Candidate == nil || next.State.CurrentDepth != 1 || next.State.CurrentRoot != v.State.CurrentRoot {
		t.Fatal("fallback follow-up lost", next)
	}
	if Eligible(*next.Candidate, next.State) {
		t.Fatal("presented question still eligible")
	}
	next = SelectMetadata(next.State, *next.Candidate, "wrong", pool, Catalog{})
	if next.Candidate == nil {
		t.Fatal("stopped below limit")
	}
	next = SelectMetadata(next.State, *next.Candidate, "correct", pool, Catalog{})
	if next.Candidate != nil || next.Reason != "question_limit" || next.State.AnsweredQuestions != 3 {
		t.Fatal("question limit exceeded", next)
	}
}
func TestMockFallbackHonorsExplicitFilters(t *testing.T) {
	s, pool := mockBank(3, "go", "sql")
	s.Config.InterviewMode = "custom"
	s.Config.CustomWeights = map[string]float64{"go": 1, "sql": 0}
	for i := range pool {
		pool[i].RootWeight = 0
	}
	pool[1].Status = "archived"
	pool[2].LevelMin = 5
	v := SelectRoot(s, pool)
	if v.Candidate == nil || v.Candidate.MaterialID != pool[0].MaterialID {
		t.Fatal("fallback ignored filters")
	}
	next := SelectMetadata(v.State, *v.Candidate, "correct", pool, Catalog{})
	if next.Candidate != nil || next.Reason != "no_remaining_questions" {
		t.Fatal("excluded candidates admitted")
	}
}
func TestMockPlannedSlotsExhaustedWithNormalRootsStillContinues(t *testing.T) {
	s, pool := mockBank(8, "go")
	v := SelectRoot(s, pool)
	v.State.Plan.Slots = []string{"go"}
	v.State.Plan.Strategy.TargetRoots = 1
	v = SelectMetadata(v.State, *v.Candidate, "correct", pool, Catalog{})
	if v.Candidate == nil || v.State.CurrentRoot != 2 || v.Reason != "root" {
		t.Fatal("planned slot exhaustion stopped roots")
	}
}
