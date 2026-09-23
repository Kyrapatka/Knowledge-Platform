package graph

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// Product preset, not an empirical claim about all backend interviews.
type TopicGroup struct {
	Key, Label string
	Weight     float64
	Terms      []string
}

var topicGroups = []TopicGroup{
	{"go", "Go / Runtime / Concurrency", 45, []string{"go", "golang", "concurrency", "runtime", "goroutine", "goroutines", "channels", "interfaces", "memory"}},
	{"sql", "SQL / Databases", 20, []string{"sql", "postgres", "postgresql", "database", "databases", "transactions", "indexes", "mysql"}},
	{"http", "HTTP / API / Networks", 12, []string{"http", "https", "api", "rest", "tcp", "network", "networks", "networking", "dns", "tls", "grpc"}},
	{"architecture", "Architecture / System Design", 10, []string{"architecture", "system design", "scalability", "caching", "design"}},
	{"messaging", "Messaging / Distributed Systems", 8, []string{"messaging", "distributed", "kafka", "rabbitmq", "queues", "queue", "event driven", "outbox"}},
	{"ops", "Testing / Operations", 5, []string{"testing", "tests", "docker", "ci cd", "observability", "metrics", "logging", "kubernetes", "devops", "infrastructure", "infra", "tooling", "git"}},
	{"other", "Other topics", 5, nil},
}

func KnownTopic(key string) bool {
	for _, g := range topicGroups {
		if g.Key == key {
			return true
		}
	}
	return false
}
func canonical(v string) string {
	return " " + strings.Join(strings.FieldsFunc(strings.ToLower(v), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }), " ") + " "
}
func Classify(c Candidate) string {
	// Canonical domain is authoritative; finer metadata and folder names are fallbacks.
	values := []string{c.Domain, c.Topic, c.Subtopic}
	for _, l := range c.Concepts {
		values = append(values, l.Slug)
	}
	values = append(values, c.Keywords, c.FolderTitle)
	for _, v := range values {
		v = canonical(v)
		for _, g := range topicGroups {
			for _, term := range g.Terms {
				if strings.Contains(v, canonical(term)) {
					return g.Key
				}
			}
		}
	}
	return "other"
}
func RootArea(c Candidate) string {
	area := primary(c)
	if area == "" {
		area = c.Subtopic
	}
	if area == "" {
		area = c.Topic
	}
	if area == "" {
		area = c.MaterialID.String()
	}
	return Classify(c) + ":" + strings.TrimSpace(canonical(area))
}

type TopicAllocation struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Available int     `json:"available"`
	Weight    float64 `json:"weight"`
	Roots     int     `json:"roots"`
}
type Strategy struct {
	TargetRoots       int     `json:"target_roots"`
	TargetBranch      int     `json:"target_branch"`
	DifficultyBoost   float64 `json:"difficulty_boost"`
	RarityBoost       float64 `json:"rarity_boost"`
	FollowProbability float64 `json:"follow_probability"`
	ConceptHops       int     `json:"concept_hops"`
	RootSwitchPenalty float64 `json:"root_switch_penalty"`
}
type InterviewPlan struct {
	Mode       string            `json:"mode"`
	DepthLevel int               `json:"depth_level"`
	Strategy   Strategy          `json:"strategy"`
	Topics     []TopicAllocation `json:"topics"`
	Slots      []string          `json:"slots"`
}

func StrategyFor(c Config) Strategy {
	n := min(c.QuestionLimit, max(1, int(math.Ceil(float64(c.QuestionLimit)/6))+2))
	s := Strategy{TargetRoots: n, TargetBranch: 4, FollowProbability: .8, ConceptHops: 1}
	if c.InterviewMode == "deep" {
		d := max(1, min(3, c.DepthLevel))
		s.TargetBranch = 4 + 2*d + 2*(d/3)
		s.TargetRoots = max(1, int(math.Ceil(float64(c.QuestionLimit)/float64(s.TargetBranch))))
		s.DifficultyBoost = float64(d) * 1.2
		s.RarityBoost = float64(d) * .7
		s.FollowProbability = .9 + float64(d)*.03
		s.ConceptHops = 1 + (d / 2)
		s.RootSwitchPenalty = float64(d) * .4
	}
	return s
}
func BuildPlan(state State, candidates []Candidate) (InterviewPlan, error) {
	c := state.Config
	if err := c.Validate(); err != nil {
		return InterviewPlan{}, err
	}
	p := InterviewPlan{Mode: c.InterviewMode, DepthLevel: c.DepthLevel, Strategy: StrategyFor(c), Topics: []TopicAllocation{}, Slots: []string{}}
	areas := map[string]map[string]bool{}
	for _, candidate := range candidates {
		if candidate.RootWeight <= 0 || !Eligible(candidate, state) {
			continue
		}
		key := Classify(candidate)
		if areas[key] == nil {
			areas[key] = map[string]bool{}
		}
		areas[key][RootArea(candidate)] = true
	}
	total, capacity := 0., 0
	for _, g := range topicGroups {
		available := len(areas[g.Key])
		if available == 0 {
			continue
		}
		w := g.Weight
		if c.InterviewMode == "balanced" {
			w = 1
		}
		if c.InterviewMode == "custom" {
			w = c.CustomWeights[g.Key]
		}
		p.Topics = append(p.Topics, TopicAllocation{Key: g.Key, Label: g.Label, Available: available, Weight: w})
		total += w
		if w > 0 {
			capacity += available
		}
	}
	if total <= 0 {
		return p, fmt.Errorf("choose at least one available topic with a positive weight; check ready questions, profile and level")
	}
	for i := range p.Topics {
		p.Topics[i].Weight /= total
	}
	p.Strategy.TargetRoots = min(p.Strategy.TargetRoots, capacity)
	// Capped largest-remainder allocation. Re-run only on topics with spare capacity.
	remaining := p.Strategy.TargetRoots
	tie := map[string]float64{}
	for _, t := range p.Topics {
		tie[t.Key] = nextRandom(&state)
	}
	for remaining > 0 {
		sum := 0.
		for _, t := range p.Topics {
			if t.Roots < t.Available {
				sum += t.Weight
			}
		}
		if sum <= 0 {
			break
		}
		type fraction struct {
			i         int
			remainder float64
		}
		fractions := []fraction{}
		budget := remaining
		for i := range p.Topics {
			t := &p.Topics[i]
			if t.Weight <= 0 || t.Roots >= t.Available {
				continue
			}
			quota := float64(budget) * t.Weight / sum
			add := min(t.Available-t.Roots, int(math.Floor(quota)))
			t.Roots += add
			remaining -= add
			if t.Roots < t.Available {
				fractions = append(fractions, fraction{i, quota - math.Floor(quota)})
			}
		}
		sort.Slice(fractions, func(i, j int) bool {
			a, b := fractions[i], fractions[j]
			if math.Abs(a.remainder-b.remainder) > 1e-9 {
				return a.remainder > b.remainder
			}
			return tie[p.Topics[a.i].Key] < tie[p.Topics[b.i].Key]
		})
		for _, f := range fractions {
			if remaining == 0 {
				break
			}
			p.Topics[f.i].Roots++
			remaining--
		}
	}
	// Smooth weighted ordering interleaves allocated topics without a fixed rotation.
	used := make([]int, len(p.Topics))
	for len(p.Slots) < p.Strategy.TargetRoots {
		best := -1
		score := -math.MaxFloat64
		for i, t := range p.Topics {
			if used[i] >= t.Roots {
				continue
			}
			deficit := float64((len(p.Slots)+1)*t.Roots)/float64(p.Strategy.TargetRoots) - float64(used[i]) + tie[t.Key]*.001
			if deficit > score {
				best, score = i, deficit
			}
		}
		if best < 0 {
			break
		}
		used[best]++
		p.Slots = append(p.Slots, p.Topics[best].Key)
	}
	return p, nil
}
