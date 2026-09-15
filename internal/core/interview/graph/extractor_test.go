package graph

import (
	"github.com/google/uuid"
	"testing"
)

func testCatalog() Catalog {
	c := Catalog{QuestionCount: 100, DocumentFrequency: map[string]int{"context": 90, "context_switch": 2, "service": 95, "work_stealing": 1}}
	add := func(slug, text string, con Constraints) {
		c.Aliases = append(c.Aliases, Alias{ConceptID: uuid.New(), Slug: slug, Text: text, Language: "any", WholeWord: true, Weight: 1, Constraints: con})
	}
	for slug, text := range map[string]string{"goroutine": "goroutine", "context": "context", "context_switch": "context switch", "mutex": "мьютекс", "waitgroup": "вейтгрупп", "service": "service", "work_stealing": "work stealing", "select": "select"} {
		add(slug, text, Constraints{})
	}
	add("goroutine", "горутина", Constraints{})
	add("context_switch", "контекст свитч", Constraints{})
	add("git_commit", "commit", Constraints{RequiresDomain: "git", RequiresAny: []string{"branch", "git"}})
	add("kafka_commit", "commit", Constraints{RequiresAny: []string{"offset", "consumer", "kafka"}})
	return c
}
func TestExtractorAcceptance(t *testing.T) {
	cases := []struct {
		name, text, want, source string
		opts                     ExtractOptions
	}{
		{"TestExactAlias", "goroutine", "goroutine", "", ExtractOptions{}},
		{"TestLongestAliasWins", "context switch", "context_switch", "exact_phrase", ExtractOptions{}},
		{"TestRussianAlias", "горутина", "goroutine", "", ExtractOptions{Language: "ru"}},
		{"TestTransliteratedAlias", "контекст свитч", "context_switch", "exact_phrase", ExtractOptions{Language: "ru"}},
		{"TestFuzzyAliasLowWeight", "вейтгруп", "waitgroup", "fuzzy", ExtractOptions{}},
		{"TestAmbiguousCommitWithoutContext", "commit", "", "", ExtractOptions{}},
		{"TestKafkaCommitWithOffsetContext", "commit offset consumer", "kafka_commit", "", ExtractOptions{}},
		{"TestGitCommitWithBranchContext", "commit branch", "git_commit", "", ExtractOptions{}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			m := Extract(tt.text, tt.opts, testCatalog())
			if tt.want == "" {
				if len(m) != 0 {
					t.Fatal(m)
				}
				return
			}
			if len(m) != 1 || m[0].Slug != tt.want || tt.source != "" && m[0].Source != tt.source {
				t.Fatalf("got %+v", m)
			}
			if tt.source == "fuzzy" && m[0].Strength > .45 {
				t.Fatal("fuzzy too strong", m)
			}
		})
	}
}
func TestPromptEchoDiscount(t *testing.T) {
	c := testCatalog()
	plain := Extract("goroutine", ExtractOptions{}, c)[0]
	echo := Extract("What is goroutine?", ExtractOptions{CurrentQuestion: "What is goroutine?"}, c)[0]
	if echo.Strength >= plain.Strength*.1 {
		t.Fatal(echo, plain)
	}
}
func TestCodeBlockDiscount(t *testing.T) {
	c := testCatalog()
	plain := Extract("goroutine", ExtractOptions{}, c)[0]
	inline := Extract("`goroutine`", ExtractOptions{}, c)[0]
	block := Extract("```goroutine```", ExtractOptions{}, c)[0]
	if !(block.Strength < inline.Strength && inline.Strength < plain.Strength) {
		t.Fatal(block, inline, plain)
	}
}
func TestManyConceptsAreCapped(t *testing.T) {
	m := Extract("goroutine context switch мьютекс service work stealing", ExtractOptions{MaxConcepts: 2}, testCatalog())
	if len(m) != 2 {
		t.Fatal(m)
	}
}
func TestRepeatedConceptAndIDF(t *testing.T) {
	c := testCatalog()
	once := Extract("goroutine", ExtractOptions{}, c)
	many := Extract("goroutine goroutine goroutine", ExtractOptions{}, c)
	if len(many) != 1 || once[0].Strength != many[0].Strength {
		t.Fatal(many)
	}
	m := Extract("service work stealing", ExtractOptions{}, c)
	if m[0].Slug != "work_stealing" {
		t.Fatal(m)
	}
}
func TestLanguageWholeWordAndNormalization(t *testing.T) {
	c := testCatalog()
	if len(Extract("mygoroutiness", ExtractOptions{}, c)) != 0 {
		t.Fatal("whole words ignored")
	}
	c.Aliases[0].Language = "fr"
	a := Alias{Slug: "substring", Text: "rut", Language: "en", Weight: 1, WholeWord: false}
	c.Aliases = []Alias{a}
	if len(Extract("brutal", ExtractOptions{Language: "en"}, c)) != 1 || len(Extract("brutal", ExtractOptions{Language: "ru"}, c)) != 0 {
		t.Fatal("language/substring")
	}
	if Normalize("Ёж — ＧＯ") != "еж - go" {
		t.Fatal(Normalize("Ёж — ＧＯ"))
	}
}
func TestUnknownDetectorIsConservative(t *testing.T) {
	if !LowInformation("Не знаю.") || !LowInformation("I don't know") || LowInformation("I am not sure which implementation you mean, but goroutines share an address space and have growing stacks.") {
		t.Fatal("low-information detector")
	}
}
