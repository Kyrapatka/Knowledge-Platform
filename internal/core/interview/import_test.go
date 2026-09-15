package interview

import "testing"

func TestSeedEdgesParse(t *testing.T) {
	b, err := ReadSeed()
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Questions) != 368 || len(b.Domains) != 15 || len(b.Edges) != 92 {
		t.Fatalf("corpus counts: %d/%d/%d", len(b.Questions), len(b.Domains), len(b.Edges))
	}
	for _, e := range b.Edges {
		if e.From == "" || e.To == "" {
			t.Fatal(e)
		}
	}
}
func TestReadyProfileRequiresPrimaryAndTestedConcept(t *testing.T) {
	p := Profile{Frequency: 5, FrequencyConfidence: .5, InterviewDifficulty: 2, Specificity: 2, RootWeight: 5, Status: "ready"}
	if p.validate() == nil {
		t.Fatal("ready without concepts")
	}
	p.Concepts = []QuestionConcept{{Slug: "goroutine", Role: "primary"}, {Slug: "goroutine", Role: "tested"}}
	if err := p.validate(); err != nil {
		t.Fatal(err)
	}
}
