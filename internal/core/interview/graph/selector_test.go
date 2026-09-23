package graph

import (
	"encoding/json"
	"github.com/google/uuid"
	"reflect"
	"testing"
)

func graphFixture() (State, []Candidate, Catalog) {
	folder := uuid.New()
	s := State{StrategyVersion: 1, RandomSeed: 123, Config: DefaultConfig(), Sources: []Source{{FolderID: folder}}, ForksUsed: map[int]int{}}
	cs := []Candidate{}
	for i, slug := range []string{"goroutine", "context_switch", "stack", "work_stealing", "heap"} {
		cs = append(cs, Candidate{MaterialID: uuid.New(), FolderID: folder, Question: slug + "?", Domain: "go", Topic: "runtime", Status: "ready", Frequency: 8, Specificity: 2, RootWeight: 1, HasAnswer: true, New: true, LearningNeed: .6, Concepts: []Link{{Slug: slug, Role: "primary", Weight: 1}, {Slug: slug, Role: "tested", Weight: 1}}})
		if i == 0 {
			cs[i].RootWeight = 10
		}
	}
	c := testCatalog()
	for _, slug := range []string{"stack", "heap"} {
		c.Aliases = append(c.Aliases, Alias{Slug: slug, Text: slug, Language: "any", Weight: 1, WholeWord: true})
	}
	return s, cs, c
}
func started(s State, c Candidate) State {
	s.CurrentRoot = 1
	s.RootsUsed = 1
	s.CurrentBranch = "1.0"
	s.QuestionsAsked = 1
	s.AskedMaterialIDs = []uuid.UUID{c.MaterialID}
	s.LastMaterialID = &c.MaterialID
	s.RecentConcepts = []string{primary(c)}
	return s
}
func TestSelectRootHighRootWeight(t *testing.T) {
	s, cs, _ := graphFixture()
	high := 0
	for i := 0; i < 500; i++ {
		s.RandomSeed = int64(i)
		got := SelectRoot(s, cs[:2])
		if got.Candidate.MaterialID == cs[0].MaterialID {
			high++
		}
	}
	if high < 300 || high == 500 {
		t.Fatal(high)
	}
}
func TestRootRespectsSourceSelection(t *testing.T) {
	s, cs, _ := graphFixture()
	cs[0].FolderID = uuid.New()
	cs[1].Status = "draft"
	got := SelectRoot(s, cs[:2])
	if got.Candidate != nil || got.State.StopReason != "no_ready_roots" {
		t.Fatal(got)
	}
}
func TestGraphFollowUpAcceptance(t *testing.T) {
	for _, name := range []string{"TestFollowUpDirectConceptMatch", "TestFollowUpUsesConceptEdge", "TestFollowUpUsesFrontier", "TestFollowUpSameTopicFallback", "TestFollowUpStartsNewRootOnDeadEnd", "TestMaxDepthBacktracks", "TestUnknownAnswerDoesNotDeepen", "TestEmptyAnswerFallback"} {
		t.Run(name, func(t *testing.T) {
			s, cs, c := graphFixture()
			s = started(s, cs[0])
			answer := "context switch"
			want := cs[1].MaterialID
			reason := "direct_concept_match"
			switch name {
			case "TestFollowUpUsesConceptEdge":
				answer = "goroutine"
				c.Edges = []Edge{{From: "goroutine", To: "context_switch", Weight: 1}}
				reason = "concept_edge"
			case "TestFollowUpUsesFrontier", "TestMaxDepthBacktracks":
				s.CurrentDepth = s.Config.MaxDepthPerBranch
				s.Frontier = []FrontierEntry{{MaterialID: cs[1].MaterialID, RootIndex: 1, SourceDepth: 0, Status: "available"}}
				reason = "frontier"
			case "TestFollowUpSameTopicFallback":
				answer = "unrelated"
				cs = cs[:2]
				reason = "same_topic"
			case "TestFollowUpStartsNewRootOnDeadEnd":
				answer = "unrelated"
				cs = cs[:2]
				cs[1].Topic = "sql"
				reason = "root"
			case "TestUnknownAnswerDoesNotDeepen":
				answer = "не знаю"
				s.CurrentDepth = 4
				cs[0].Specificity = 4
				cs[1].Concepts = []Link{{Slug: "goroutine", Role: "tested"}}
				reason = "remediation"
			case "TestEmptyAnswerFallback":
				answer = ""
				cs[0].Concepts = append(cs[0].Concepts, Link{Slug: "context_switch", Role: "hook"})
			}
			before, _ := json.Marshal(s)
			got := SelectFollowUp(s, cs[0], answer, "", false, cs, c)
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("mutated caller state, breaks undo")
			}
			if got.Candidate == nil || got.Candidate.MaterialID != want || got.Reason != reason {
				t.Fatalf("got %+v", got)
			}
			if name == "TestUnknownAnswerDoesNotDeepen" && got.State.CurrentDepth > s.CurrentDepth {
				t.Fatal("unknown deepened")
			}
		})
	}
}
func TestDepthDoesNotBelongToQuestion(t *testing.T) {
	s, cs, c := graphFixture()
	s = started(s, cs[0])
	for _, depth := range []int{0, 5, 8} {
		s.CurrentDepth = depth
		got := SelectFollowUp(s, cs[0], "context switch", "", false, cs, c)
		if got.Candidate == nil || got.State.CurrentDepth != depth+1 {
			t.Fatal(got)
		}
		if got.Candidate.Specificity != 2 {
			t.Fatal("depth changed specificity")
		}
	}
}
func TestLimitsAndAlreadyAsked(t *testing.T) {
	s, cs, c := graphFixture()
	s = started(s, cs[0])
	s.Config.QuestionLimit = 1
	if got := SelectFollowUp(s, cs[0], "stack", "", false, cs, c); got.Candidate != nil || got.State.StopReason != "question_limit" {
		t.Fatal(got)
	}
	s.Config.QuestionLimit = 24
	s.Config.MaxRoots = 1
	s.CurrentDepth = s.Config.MaxDepthPerBranch
	if got := SelectFollowUp(s, cs[0], "stack", "", false, cs, c); got.Candidate != nil {
		t.Fatal(got)
	}
	s.CurrentDepth = 0
	s.AskedMaterialIDs = append(s.AskedMaterialIDs, cs[1].MaterialID)
	if got := SelectFollowUp(s, cs[0], "context switch", "", false, cs, c); got.Candidate != nil && got.Candidate.MaterialID == cs[1].MaterialID {
		t.Fatal("repeat")
	}
}
func TestSameSeedProducesSameSelection(t *testing.T) {
	s, cs, c := graphFixture()
	s = started(s, cs[0])
	s.RandomIndex = 17
	raw, _ := json.Marshal(s)
	var restored State
	_ = json.Unmarshal(raw, &restored)
	a := SelectFollowUp(s, cs[0], "context switch stack", "", false, cs, c)
	b := SelectFollowUp(restored, cs[0], "context switch stack", "", false, cs, c)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("restart changed selection")
	}
	if len(a.State.Frontier) == 0 || len(a.Scores) < 2 {
		t.Fatal("alternatives lost")
	}
}
func TestSpecificityFrequencyAndCooldown(t *testing.T) {
	if TargetSpecificity(9, 10) <= TargetSpecificity(1, 10) || TargetFrequency(9, 10) >= TargetFrequency(1, 10) {
		t.Fatal("depth bias")
	}
	s, cs, _ := graphFixture()
	s.RecentConcepts = []string{"context_switch", "stack", "context_switch"}
	a := followScore(cs[1], cs[0], s, 1, "A", 8)
	b := followScore(cs[4], cs[0], s, 1, "A", 8)
	if a.Components["repetition_penalty"] <= b.Components["repetition_penalty"] {
		t.Fatal("concept cooldown")
	}
	for _, depth := range []int{0, 8} {
		for _, freq := range []int{1, 10} {
			for _, spec := range []int{1, 5} {
				cs[1].Frequency = freq
				cs[1].Specificity = spec
				if followScore(cs[1], cs[0], s, 1, "A", depth).Components["frequency_fit"] <= 0 {
					t.Fatal("hard frequency filter")
				}
			}
		}
	}
}
func TestGraphProbeCanSelectNotDueQuestion(t *testing.T) {
	s, cs, c := graphFixture()
	s = started(s, cs[0])
	cs[1].New = false
	cs[1].Due = false
	got := SelectFollowUp(s, cs[0], "context switch", "", false, cs, c)
	if got.Candidate == nil || got.ReviewCredit {
		t.Fatal(got)
	}
	cs[1].Due = true
	got = SelectFollowUp(s, cs[0], "context switch", "", false, cs, c)
	if !got.ReviewCredit {
		t.Fatal("due credit missing")
	}
}
func TestConfigValidation(t *testing.T) {
	c := DefaultConfig()
	if c.Validate() != nil {
		t.Fatal("invalid defaults")
	}
	for _, mutate := range []func(*Config){func(c *Config) { c.MaxRoots = 0 }, func(c *Config) { c.MaxDepthPerBranch = 0 }, func(c *Config) { c.MaxForksPerRoot = -1 }, func(c *Config) { c.Temperature = 0 }, func(c *Config) { c.QuestionLimit = 101 }, func(c *Config) { c.EarlyReviewPolicy = "credit" }} {
		c = DefaultConfig()
		mutate(&c)
		if c.Validate() == nil {
			t.Fatal(c)
		}
	}
}
