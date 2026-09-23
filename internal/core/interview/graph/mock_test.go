package graph

import (
	"fmt"
	"github.com/google/uuid"
	"math"
	"reflect"
	"slices"
	"testing"
)

func mockBank(n int, groups ...string) (State, []Candidate) {
	s := State{PracticeOnly: true, Config: DefaultConfig(), RandomSeed: 19}
	pool := []Candidate{}
	for _, g := range groups {
		folder := uuid.New()
		s.Sources = append(s.Sources, Source{FolderID: folder})
		for i := 0; i < n; i++ {
			slug := fmt.Sprintf("%s_%d", g, i)
			pool = append(pool, Candidate{MaterialID: uuid.New(), FolderID: folder, Question: slug, Domain: g, Topic: g, Status: "ready", HasAnswer: true, RootWeight: 8, FollowupWeight: 8, Frequency: 7, InterviewDifficulty: 3, Specificity: 2, Concepts: []Link{{Slug: slug, Role: "primary", Weight: 1}}})
		}
	}
	return s, pool
}
func TestMockDistributionAndScaling(t *testing.T) {
	for _, tc := range []struct {
		mode    string
		groups  []string
		weights map[string]float64
		want    []float64
	}{
		{"real", []string{"go", "http"}, nil, []float64{45. / 57, 12. / 57}},
		{"real", []string{"go", "sql", "http"}, nil, []float64{45. / 77, 20. / 77, 12. / 77}},
		{"balanced", []string{"go", "sql", "http"}, nil, []float64{1. / 3, 1. / 3, 1. / 3}},
		{"custom", []string{"go", "sql", "http"}, map[string]float64{"go": 5, "sql": 3, "http": 2}, []float64{.5, .3, .2}},
		{"custom", []string{"go", "sql", "http"}, map[string]float64{"go": 10, "sql": 6, "http": 4}, []float64{.5, .3, .2}},
	} {
		s, pool := mockBank(10, tc.groups...)
		s.Config.InterviewMode = tc.mode
		s.Config.CustomWeights = tc.weights
		p, err := BuildPlan(s, pool)
		if err != nil {
			t.Fatal(err)
		}
		sum := 0
		for i, topic := range p.Topics {
			if math.Abs(topic.Weight-tc.want[i]) > 1e-6 {
				t.Fatalf("%s %+v", tc.mode, p)
			}
			sum += topic.Roots
		}
		if sum != 6 || len(p.Slots) != 6 {
			t.Fatal(p)
		}
	}
	for questions, want := range map[int]int{12: 4, 18: 5, 24: 6, 30: 7, 40: 9} {
		c := DefaultConfig()
		c.QuestionLimit = questions
		if s := StrategyFor(c); s.TargetRoots != want {
			t.Fatalf("%d: %+v", questions, s)
		}
	}
}
func TestMockCapacityFairTiesAndValidation(t *testing.T) {
	s, pool := mockBank(12, "sql", "http")
	other, goPool := mockBank(1, "go")
	s.Sources = append(s.Sources, other.Sources...)
	pool = append(pool, goPool...)
	p, err := BuildPlan(s, pool)
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, topic := range p.Topics {
		sum += topic.Roots
		if topic.Roots > topic.Available {
			t.Fatal(p)
		}
		if topic.Key == "go" && topic.Roots != 1 {
			t.Fatal(p)
		}
	}
	if sum != 6 {
		t.Fatal(p)
	}
	s.Config.InterviewMode = "custom"
	if _, err := BuildPlan(s, pool); err == nil {
		t.Fatal("zero custom accepted")
	}
	s.Config.CustomWeights = map[string]float64{"go": -1}
	if s.Config.Validate() == nil {
		t.Fatal("negative accepted")
	}
	s.Config = DefaultConfig()
	s.Config.InterviewMode = "typo"
	if s.Config.Validate() == nil {
		t.Fatal("invalid mode")
	}
	s.Config.InterviewMode = "deep"
	s.Config.DepthLevel = 4
	if s.Config.Validate() == nil {
		t.Fatal("invalid depth")
	}
	s.Config = DefaultConfig()
	s.Config.InterviewMode = "balanced"
	s.Config.QuestionLimit = 1
	winners := map[string]bool{}
	for seed := int64(1); seed < 40; seed++ {
		s.RandomSeed = seed
		p, err = BuildPlan(s, pool)
		if err != nil {
			t.Fatal(err)
		}
		winners[p.Slots[0]] = true
	}
	if len(winners) != 3 {
		t.Fatal("biased ties", winners)
	}
}
func TestMockHundredReplacementsDoNotComplete(t *testing.T) {
	s, pool := mockBank(8, "go")
	first := SelectRoot(s, pool)
	s = first.State
	last := first.Candidate.MaterialID
	seen := map[uuid.UUID]bool{last: true}
	for i := 0; i < 100; i++ {
		next := SelectMetadata(s, *first.Candidate, "next_route", pool, Catalog{})
		if next.Candidate == nil || next.State.StopReason != "" {
			t.Fatalf("skip %d stopped %+v", i, next)
		}
		if i < 7 && seen[next.Candidate.MaterialID] {
			t.Fatal("unnecessary exact repeat")
		}
		if next.Candidate.MaterialID == last {
			t.Fatal("consecutive repeat")
		}
		seen[next.Candidate.MaterialID] = true
		last = next.Candidate.MaterialID
		s = next.State
		first = next
		if s.CurrentRoot != 1 || len(s.CompletedRootIDs) != 0 || s.AnsweredQuestions != 0 || next.ReviewCredit {
			t.Fatal("skip consumed slot", s)
		}
	}
	if len(s.SkippedRootIDs) != 100 || len(s.ShownRootIDs) != 101 {
		t.Fatal("history lost")
	}
	s, pool = mockBank(1, "go")
	one := SelectRoot(s, pool)
	for i := 0; i < 20; i++ {
		one = SelectMetadata(one.State, *one.Candidate, "next_route", pool, Catalog{})
		if one.Candidate == nil {
			t.Fatal("single root exhausted")
		}
	}
}
func TestMockAreaNoveltyAndDeepPreference(t *testing.T) {
	s, pool := mockBank(3, "go")
	pool[1].Concepts = append([]Link(nil), pool[0].Concepts...)
	s.RootAreas = []string{RootArea(pool[0])}
	s.AskedMaterialIDs = []uuid.UUID{pool[0].MaterialID}
	s.ShownRootIDs = []uuid.UUID{pool[0].MaterialID}
	n := SelectRoot(s, pool)
	if n.Candidate.MaterialID != pool[2].MaterialID {
		t.Fatal("repeated area ahead of unseen area")
	}
	c := DefaultConfig()
	normal := StrategyFor(c)
	previous := normal.TargetRoots
	for level := 1; level <= 3; level++ {
		c.InterviewMode = "deep"
		c.DepthLevel = level
		strategy := StrategyFor(c)
		if strategy.TargetRoots >= previous || strategy.TargetBranch <= normal.TargetBranch || strategy.FollowProbability <= normal.FollowProbability {
			t.Fatal(strategy)
		}
		previous = strategy.TargetRoots
		s.Plan = &InterviewPlan{Strategy: strategy}
		easy, hard := pool[0], pool[0]
		easy.InterviewDifficulty = 1
		easy.Frequency = 10
		hard.InterviewDifficulty = 5
		hard.Frequency = 1
		if deepScore(hard, s, false) <= deepScore(easy, s, false) || deepScore(hard, s, true) != 0 {
			t.Fatal("deep preference or remediation")
		}
	}
}
func TestMockBranchesHistoryAndCompletion(t *testing.T) {
	s, pool := mockBank(8, "go")
	for i := range pool {
		pool[i].Concepts = append(pool[i].Concepts, Link{Slug: "shared", Role: "tested", Weight: 1}, Link{Slug: "shared", Role: "answer", Weight: 1})
	}
	first := SelectRoot(s, pool)
	original := clone(first.State)
	seen := []uuid.UUID{first.Candidate.MaterialID}
	v := first
	for i := 0; i < 24 && v.Candidate != nil; i++ {
		old := v
		v = SelectMetadata(v.State, *v.Candidate, "correct", pool, Catalog{})
		if v.Candidate != nil && v.State.CurrentRoot == old.State.CurrentRoot && slices.Contains(seen, v.Candidate.MaterialID) {
			t.Fatal("repeated follow-up")
		}
		if v.Candidate != nil {
			seen = append(seen, v.Candidate.MaterialID)
		}
		if v.ReviewCredit {
			t.Fatal("SRS credit")
		}
	}
	if !reflect.DeepEqual(first.State, original) {
		t.Fatal("input mutated")
	}
	if v.Candidate != nil || len(v.State.CompletedRootIDs) != 6 || v.State.AnsweredQuestions == 0 {
		t.Fatalf("did not complete branches: %+v", v.State)
	}
}

func TestMockSparseMetadataDraftsZeroWeightsAndSkippedAnsweredBranch(t *testing.T) {
	s, pool := mockBank(4, "go", "sql")
	s.Config.InterviewMode = "custom"
	s.Config.CustomWeights = map[string]float64{"go": 1, "sql": 0}
	for i := range pool {
		pool[i].Concepts = nil
		pool[i].Subtopic = fmt.Sprintf("area%d", i)
		pool[i].Due = true
		pool[i].New = true
	}
	v := SelectRoot(s, pool)
	if v.Candidate == nil || Classify(*v.Candidate) != "go" || v.ReviewCredit {
		t.Fatal("sparse metadata")
	}
	for i := 0; i < 12; i++ {
		v = SelectMetadata(v.State, *v.Candidate, "next_route", pool, Catalog{})
		if Classify(*v.Candidate) != "go" {
			t.Fatal("zero-weight topic used")
		}
	}
	for i := range pool {
		pool[i].Status = "draft"
		pool[i].HasAnswer = false
	}
	s.Config.IncludeDraft = false
	if _, err := BuildPlan(s, pool); err == nil {
		t.Fatal("drafts admitted")
	}
	s.Config.IncludeDraft = true
	if _, err := BuildPlan(s, pool); err != nil {
		t.Fatal(err)
	}
	s, pool = mockBank(6, "go")
	for i := range pool {
		pool[i].Concepts = append(pool[i].Concepts, Link{Slug: "shared", Role: "answer", Weight: 1}, Link{Slug: "shared", Role: "tested", Weight: 1})
	}
	v = SelectRoot(s, pool)
	v = SelectMetadata(v.State, *v.Candidate, "correct", pool, Catalog{})
	if v.State.BranchAnswered != 1 {
		t.Fatal("missing answer")
	}
	v = SelectMetadata(v.State, *v.Candidate, "next_route", pool, Catalog{})
	if v.State.AnsweredQuestions != 1 || len(v.State.CompletedRootIDs) != 0 || v.State.CurrentRoot != 1 || v.State.BranchAnswered != 0 {
		t.Fatal("skip completed partial branch")
	}
}

func TestMockFrontierAndClassification(t *testing.T) {
	for _, tc := range []struct {
		c    Candidate
		want string
	}{{Candidate{Domain: "go", Topic: "SQL"}, "go"}, {Candidate{Topic: "PostgreSQL"}, "sql"}, {Candidate{Keywords: "dns,tls"}, "http"}, {Candidate{Domain: "infrastructure"}, "ops"}, {Candidate{FolderTitle: "Odd subjects"}, "other"}, {Candidate{Domain: "messaging"}, "messaging"}} {
		if got := Classify(tc.c); got != tc.want {
			t.Fatalf("%+v: %s", tc, got)
		}
	}
	s, pool := mockBank(3, "go")
	v := SelectRoot(s, pool)
	s = v.State
	s.Config.QuestionLimit = 24
	s.Plan.Slots = []string{"go"}
	s.Plan.Strategy.TargetRoots = 1
	var target Candidate
	for _, c := range pool {
		if c.MaterialID != v.Candidate.MaterialID {
			target = c
			break
		}
	}
	s.Frontier = []FrontierEntry{{MaterialID: target.MaterialID, RootIndex: 1, SourceDepth: 0, Status: "available", Score: 5}}
	n := SelectMetadata(s, *v.Candidate, "correct", pool, Catalog{})
	if n.Reason != "frontier" || n.Candidate == nil || n.Candidate.MaterialID != target.MaterialID {
		t.Fatal("lost graph backtracking")
	}
}
