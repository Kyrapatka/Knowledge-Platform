package seed

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type corpus struct {
	Version string `json:"version"`
	Domains []struct {
		Slug  string `json:"slug"`
		Count int    `json:"question_count"`
	} `json:"domains"`
	Questions []struct {
		SeedKey     string `json:"seed_key"`
		Domain      string `json:"domain"`
		Question    string `json:"question"`
		Answer      string `json:"answer"`
		Status      string `json:"status"`
		Category    string `json:"category"`
		Frequency   int    `json:"frequency"`
		Difficulty  int    `json:"interview_difficulty"`
		Specificity int    `json:"specificity"`
		RootWeight  int    `json:"root_weight"`
		Concepts    []struct {
			Slug string `json:"slug"`
			Role string `json:"role"`
		} `json:"concepts"`
	} `json:"questions"`
	Concepts []struct {
		Slug    string `json:"slug"`
		Aliases []struct {
			Alias       string `json:"alias"`
			WholeWord   bool   `json:"whole_word"`
			Constraints struct {
				RequiresAny    []string `json:"requires_any"`
				RequiresDomain string   `json:"requires_domain"`
			} `json:"constraints"`
		} `json:"aliases"`
	} `json:"concepts"`
	Edges []struct {
		From     string `json:"from_slug"`
		To       string `json:"to_slug"`
		Relation string `json:"relation"`
	} `json:"edges"`
}

func TestCorpusIntegrity(t *testing.T) {
	var b corpus
	if err := json.Unmarshal(Raw(), &b); err != nil {
		t.Fatal(err)
	}
	if b.Version != Version || len(b.Questions) != QuestionCount || len(b.Domains) != 9 {
		t.Fatalf("unexpected corpus: version=%q questions=%d domains=%d", b.Version, len(b.Questions), len(b.Domains))
	}
	concepts := map[string]bool{}
	ambiguous := map[string]bool{"context": true, "index": true, "commit": true, "stream": true, "partition": true, "pool": true, "process": true, "thread": true, "transaction": true, "map": true, "cache": true}
	for _, c := range b.Concepts {
		if c.Slug == "" || concepts[c.Slug] {
			t.Fatalf("empty or duplicate concept %q", c.Slug)
		}
		concepts[c.Slug] = true
		for _, a := range c.Aliases {
			if a.Alias == "" || !a.WholeWord {
				t.Errorf("invalid alias for %s: %q", c.Slug, a.Alias)
			}
			if ambiguous[strings.ToLower(a.Alias)] && len(a.Constraints.RequiresAny) == 0 && a.Constraints.RequiresDomain == "" {
				t.Errorf("ambiguous alias %q for %s has no context constraint", a.Alias, c.Slug)
			}
		}
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	categories := map[string]bool{"theory": true, "edge": true, "internals": true, "production": true, "design": true, "coding": true}
	for _, q := range b.Questions {
		if seen[q.SeedKey] || q.SeedKey == "" {
			t.Fatalf("duplicate or empty seed key: %q", q.SeedKey)
		}
		seen[q.SeedKey] = true
		counts[q.Domain]++
		if q.Status != "draft" || q.Answer != "" {
			t.Errorf("%s must remain an answer-free draft", q.SeedKey)
		}
		if q.Question == "" || strings.ContainsRune(q.Question, '\uFFFD') {
			t.Errorf("invalid question text in %s", q.SeedKey)
		}
		if !categories[q.Category] || q.Frequency < 1 || q.Frequency > 10 || q.Difficulty < 1 || q.Difficulty > 5 || q.Specificity < 1 || q.Specificity > 5 || q.RootWeight < 0 || q.RootWeight > 10 {
			t.Errorf("invalid profile metadata in %s", q.SeedKey)
		}
		roles := map[string]bool{}
		primary, tested := 0, 0
		for _, link := range q.Concepts {
			if !concepts[link.Slug] {
				t.Errorf("%s references missing concept %s", q.SeedKey, link.Slug)
			}
			pair := link.Slug + ":" + link.Role
			if roles[pair] {
				t.Errorf("duplicate question link %s in %s", pair, q.SeedKey)
			}
			roles[pair] = true
			switch link.Role {
			case "primary":
				primary++
			case "tested":
				tested++
			case "hook", "prerequisite":
			default:
				t.Errorf("invalid role %q", link.Role)
			}
		}
		if primary != 1 || tested < 1 {
			t.Errorf("%s needs one primary and at least one tested concept", q.SeedKey)
		}
		if q.SeedKey == "GO002" && (!roles["goroutine:tested"] || !roles["os_thread:tested"] || !roles["go_scheduler:hook"] || roles["go_scheduler:tested"]) {
			t.Error("GO002 must distinguish what it tests from its scheduler follow-up hook")
		}
	}
	expected := map[string]struct {
		Prefix string
		Count  int
	}{
		"go": {"GO", 130}, "databases": {"DB", 40}, "networking": {"NET", 35}, "git": {"GIT", 18}, "infrastructure": {"INF", 50},
		"messaging": {"MQ", 25}, "services": {"SRV", 20}, "architecture": {"ARC", 35}, "algorithms": {"ALG", 15},
	}
	for domain, want := range expected {
		if counts[domain] != want.Count {
			t.Errorf("%s: got %d questions, want %d", domain, counts[domain], want.Count)
		}
		for i := 1; i <= want.Count; i++ {
			if key := fmt.Sprintf("%s%03d", want.Prefix, i); !seen[key] {
				t.Errorf("missing source question %s", key)
			}
		}
	}
	for _, d := range b.Domains {
		if counts[d.Slug] != d.Count {
			t.Errorf("incorrect manifest count for %s", d.Slug)
		}
	}
	for _, e := range b.Edges {
		if !concepts[e.From] || !concepts[e.To] || e.Relation == "" {
			t.Errorf("invalid edge: %+v", e)
		}
	}
}

func TestRawCannotMutateSource(t *testing.T) {
	one, two := Raw(), Raw()
	one[0] = '!'
	if two[0] != '{' || Raw()[0] != '{' {
		t.Fatal("Raw exposed mutable embedded storage")
	}
}
