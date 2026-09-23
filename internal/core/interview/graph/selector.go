package graph

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

func clone(s State) State {
	raw, _ := json.Marshal(s)
	var out State
	_ = json.Unmarshal(raw, &out)
	if out.ForksUsed == nil {
		out.ForksUsed = map[int]int{}
	}
	return out
}
func Eligible(c Candidate, s State) bool {
	if s.PracticeOnly && s.Plan != nil {
		allowed := false
		for _, topic := range s.Plan.Topics {
			if topic.Key == Classify(c) && topic.Weight > 0 {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	if strings.TrimSpace(c.Question) == "" || strings.TrimSpace(c.Question) == "." || c.Status == "archived" || (c.Status != "ready" && !(s.Config.IncludeDraft && c.Status == "draft")) || (!s.Config.IncludeDraft && !c.HasAnswer) {
		return false
	}
	if c.LevelMin > 0 && s.Config.Level < c.LevelMin || c.LevelMax > 0 && s.Config.Level > c.LevelMax {
		return false
	}
	if s.Config.Profile != "" && s.Config.Profile != "all" {
		found := false
		for _, profile := range c.Profiles {
			if profile == s.Config.Profile {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, id := range s.AskedMaterialIDs {
		if id == c.MaterialID {
			return false
		}
	}
	for _, source := range s.Sources {
		if source.FolderID == c.FolderID {
			if len(source.Topics) == 0 {
				return true
			}
			for _, topic := range source.Topics {
				if topic == c.Topic || (topic == "__none__" && c.Topic == "") {
					return true
				}
			}
		}
	}
	return false
}
func primary(c Candidate) string {
	for _, l := range c.Concepts {
		if l.Role == "primary" {
			return l.Slug
		}
	}
	return ""
}
func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }
func TargetSpecificity(depth, maxDepth int) float64 {
	z := clamp(float64(depth)/float64(maxDepth), 0, 1)
	return clamp(1+4*math.Pow(z, 1.1), 1, 5)
}
func TargetFrequency(depth, maxDepth int) float64 {
	z := clamp(float64(depth)/float64(maxDepth), 0, 1)
	return clamp(9.5-5.5*math.Pow(z, 1.15), 3, 10)
}
func penalty(c Candidate, s State) float64 {
	p := 0.
	focus := primary(c)
	for _, recent := range s.RecentConcepts {
		if focus != "" && recent == focus {
			p += .8
		}
	}
	if len(s.RecentConcepts) >= 2 && s.RecentConcepts[len(s.RecentConcepts)-2] == focus {
		p += 2.4
	}
	return p
}
func rootScore(c Candidate, s State) Score {
	need := clamp(c.LearningNeed, 0, 1)
	if c.Due || c.New {
		need = math.Max(.85, need)
	}
	parts := map[string]float64{"root_weight": float64(c.RootWeight) / 10, "frequency": float64(c.Frequency-1) / 9, "learning_need": need, "novelty": 1, "repetition_penalty": penalty(c, s)}
	score := 1.8*parts["root_weight"] + 1.4*parts["frequency"] + .15*need + .6 - parts["repetition_penalty"]
	return Score{c.MaterialID, c.SeedKey, c.Question, score, "root", "F", parts}
}
func nextRandom(s *State) float64 {
	s.RandomIndex++
	z := uint64(s.RandomSeed) + s.RandomIndex*0x9e3779b97f4a7c15
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	return float64(z>>11) * (1.0 / (1 << 53))
}
func sample(scores []Score, s *State) int {
	if len(scores) == 1 {
		return 0
	}
	max := scores[0].Score
	for _, c := range scores {
		if c.Score > max {
			max = c.Score
		}
	}
	weights := make([]float64, len(scores))
	total := 0.
	for i, c := range scores {
		weights[i] = math.Exp((c.Score - max) / s.Config.Temperature)
		total += weights[i]
	}
	target := nextRandom(s) * total
	for i, w := range weights {
		target -= w
		if target <= 0 {
			return i
		}
	}
	return len(scores) - 1
}

// Randomness only chooses among competitive candidates, with a stable order
// independent of SQL order or Go map iteration.
func qualityCandidates(scores []Score) []Score {
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score != scores[j].Score {
			return scores[i].Score > scores[j].Score
		}
		return scores[i].MaterialID.String() < scores[j].MaterialID.String()
	})
	if len(scores) == 0 {
		return scores
	}
	n := 0
	for n < len(scores) && n < 5 && scores[n].Score >= scores[0].Score-1.75 {
		n++
	}
	return scores[:n]
}
func boundFrontier(s *State) {
	limit := s.Config.MaxFrontierSize
	if limit < 1 {
		limit = 12
	}
	kept := []FrontierEntry{}
	for _, f := range s.Frontier {
		if f.Status != "available" || f.RootIndex != s.CurrentRoot {
			continue
		}
		asked := false
		for _, id := range s.AskedMaterialIDs {
			if id == f.MaterialID {
				asked = true
				break
			}
		}
		if !asked {
			kept = append(kept, f)
		}
	}
	rank := func(f FrontierEntry) float64 { return f.Score - .04*float64(max(0, s.QuestionsAsked-f.AddedAt)) }
	sort.SliceStable(kept, func(i, j int) bool {
		if rank(kept[i]) != rank(kept[j]) {
			return rank(kept[i]) > rank(kept[j])
		}
		return kept[i].MaterialID.String() < kept[j].MaterialID.String()
	})
	if len(kept) > limit {
		kept = kept[:limit]
	}
	s.Frontier = kept
}
func finishSelection(out Selection, c Candidate, depth int, root bool) Selection {
	s := &out.State
	s.StopReason = ""
	s.CurrentDepth = depth
	s.QuestionsAsked++
	s.AskedMaterialIDs = append(s.AskedMaterialIDs, c.MaterialID)
	boundFrontier(s)
	id := c.MaterialID
	s.LastMaterialID = &id
	if p := primary(c); p != "" {
		s.RecentConcepts = append(s.RecentConcepts, p)
		if len(s.RecentConcepts) > 3 {
			s.RecentConcepts = s.RecentConcepts[len(s.RecentConcepts)-3:]
		}
	}
	out.Candidate = &c
	out.ReviewCredit = !s.PracticeOnly && !s.Config.IncludeDraft && c.Status == "ready" && c.HasAnswer && (c.Due || (root && c.New))
	return out
}
func SelectRoot(state State, candidates []Candidate) Selection {
	if state.PracticeOnly {
		return selectMockRoot(state, candidates)
	}
	out := Selection{State: clone(state), Matches: []Match{}, Scores: []Score{}}
	if state.QuestionsAsked >= state.Config.QuestionLimit {
		out.State.StopReason = "question_limit"
		out.Reason = "question_limit"
		return out
	}
	if state.RootsUsed >= state.Config.MaxRoots {
		out.State.StopReason = "graph_exhausted"
		out.Reason = "graph_exhausted"
		return out
	}
	eligible := map[string]Candidate{}
	for _, c := range candidates {
		if Eligible(c, state) && c.RootWeight > 0 {
			out.Scores = append(out.Scores, rootScore(c, state))
			eligible[c.MaterialID.String()] = c
		}
	}
	sort.Slice(out.Scores, func(i, j int) bool { return out.Scores[i].MaterialID.String() < out.Scores[j].MaterialID.String() })
	if len(out.Scores) == 0 {
		out.Reason = "graph_exhausted"
		if state.RootsUsed == 0 {
			out.Reason = "no_ready_roots"
		}
		out.State.StopReason = out.Reason
		return out
	}
	out.Scores = qualityCandidates(out.Scores)
	chosen := out.Scores[sample(out.Scores, &out.State)]
	out.State.RootsUsed++
	out.State.CurrentRoot = out.State.RootsUsed
	out.State.CurrentBranch = fmt.Sprintf("%d.0", out.State.CurrentRoot)
	out.Reason = "root"
	return finishSelection(out, eligible[chosen.MaterialID.String()], 0, true)
}
func relevance(c Candidate, matches []Match, catalog Catalog) (float64, string) {
	best := 0.
	tier := ""
	for _, l := range c.Concepts {
		if l.Role != "primary" && l.Role != "tested" && l.Role != "prerequisite" {
			continue
		}
		weight := l.Weight
		if weight == 0 {
			weight = 1
		}
		for _, m := range matches {
			if l.Slug == m.Slug {
				r := m.Strength * weight
				t := "A"
				if l.Role == "prerequisite" {
					r *= .6
					t = "B"
				}
				if tier == "" || t < tier || t == tier && r > best {
					best, tier = r, t
				}
			}
		}
	}
	if tier != "" {
		return best, tier
	}
	for _, e := range catalog.Edges {
		for _, m := range matches {
			if e.From != m.Slug {
				continue
			}
			for _, l := range c.Concepts {
				if (l.Role == "tested" || l.Role == "primary") && l.Slug == e.To {
					w := e.Weight
					if w == 0 {
						w = 1
					}
					best = math.Max(best, m.Strength*w*.65)
					tier = "C"
				}
			}
		}
	}
	return best, tier
}
func followScore(c, current Candidate, s State, rel float64, tier string, depth int) Score {
	spec := math.Exp(-math.Pow(float64(c.Specificity)-TargetSpecificity(depth, s.Config.MaxDepthPerBranch), 2) / (2 * 1.1 * 1.1))
	freq := math.Exp(-math.Pow(float64(c.Frequency)-TargetFrequency(depth, s.Config.MaxDepthPerBranch), 2) / 8)
	continuity, cross := 0., s.Config.CrossTopicPenalty
	if c.Topic == current.Topic && c.Domain == current.Domain {
		continuity = .8
		cross = 0
	} else if c.Domain == current.Domain {
		continuity = .4
		cross *= .4
	} else if tier == "C" || tier == "A" {
		cross *= .35
	}
	ambiguity := 0.
	if rel < .4 {
		ambiguity = .25
	}
	parts := map[string]float64{"relevance": rel, "specificity_fit": spec, "frequency_fit": freq, "followup_weight": clamp(float64(c.FollowupWeight)/10, 0, 1), "learning_need": clamp(c.LearningNeed, 0, 1), "continuity": continuity, "novelty": 1, "repetition_penalty": penalty(c, s), "ambiguity_penalty": ambiguity, "cross_topic_penalty": cross}
	score := 3.2*rel + 1.5*spec + 1.0*freq + .6*parts["followup_weight"] + .12*parts["learning_need"] + 1.2*continuity + .5 - parts["repetition_penalty"] - ambiguity - cross
	reason := map[string]string{"A": "direct_concept_match", "B": "related_concept_match", "C": "concept_edge", "D": "frontier", "E": "same_topic"}[tier]
	return Score{c.MaterialID, c.SeedKey, c.Question, score, reason, tier, parts}
}
func SelectFollowUp(state State, current Candidate, answer, language string, wrong bool, candidates []Candidate, catalog Catalog) Selection {
	out := Selection{State: clone(state), Matches: []Match{}, Scores: []Score{}}
	if state.QuestionsAsked >= state.Config.QuestionLimit {
		out.State.StopReason = "question_limit"
		out.Reason = "question_limit"
		return out
	}
	matches := Extract(answer, ExtractOptions{language, current.Domain, current.Question, state.Config.MaxDetectedConcepts}, catalog)
	if wrong {
		for i := range matches {
			matches[i].Strength *= .65
		}
	}
	out.Matches = matches
	unknown := LowInformation(answer)
	if unknown {
		matches = nil
		for _, l := range current.Concepts {
			if l.Role != "hook" {
				matches = append(matches, Match{Slug: l.Slug, Strength: .65, Source: "fallback"})
			}
		}
	} else if len(matches) == 0 || matches[0].Strength < .1 {
		matches = nil
		for _, l := range current.Concepts {
			if l.Role == "hook" {
				matches = append(matches, Match{Slug: l.Slug, Strength: .55, Source: "fallback"})
			}
		}
		out.Matches = matches
	}
	return selectMatches(state, current, matches, unknown, false, false, candidates, catalog)
}

// SelectMetadata is the runtime and CLI entry point. Routing only reads
// question metadata; neither aliases nor user-authored answer prose are parsed.
func SelectMetadata(state State, current Candidate, action string, candidates []Candidate, catalog Catalog) Selection {
	if state.PracticeOnly {
		return selectMockNext(state, current, action, candidates, catalog)
	}
	wrong := action == "wrong"
	nextRoute := action == "next_route"
	matches := []Match{}
	roles := []string{"answer", "hook"}
	if wrong {
		roles = []string{"wrong_fallback", "prerequisite"}
	}
	seen := map[string]bool{}
	for _, role := range roles {
		links := append([]Link(nil), current.Concepts...)
		sort.SliceStable(links, func(i, j int) bool { return links[i].Ordinal < links[j].Ordinal })
		for _, l := range links {
			if l.Role != role || seen[l.Slug] {
				continue
			}
			seen[l.Slug] = true
			weight := l.Weight
			if weight <= 0 {
				weight = 1
			}
			weight /= 1 + .15*float64(max(0, l.Ordinal))
			if role == "hook" {
				weight *= .55
			}
			if wrong && role == "prerequisite" {
				weight *= .8
			}
			matches = append(matches, Match{Slug: l.Slug, Strength: weight, Source: role})
		}
	}
	if wrong {
		for _, l := range current.Concepts {
			if (l.Role == "primary" || l.Role == "tested") && !seen[l.Slug] {
				seen[l.Slug] = true
				matches = append(matches, Match{Slug: l.Slug, Strength: .5, Source: "remediation"})
			}
		}
	}
	if nextRoute {
		matches = nil
	}
	if !nextRoute {
		priority := []string{"answer", "hook"}
		if wrong {
			priority = []string{"wrong_fallback", "prerequisite", "remediation"}
		}
		for _, source := range priority {
			group := []Match{}
			for _, m := range matches {
				if m.Source == source {
					group = append(group, m)
				}
			}
			found := false
			for _, c := range candidates {
				if !Eligible(c, state) || c.MaterialID == current.MaterialID {
					continue
				}
				if wrong && (c.Specificity > current.Specificity || c.InterviewDifficulty > current.InterviewDifficulty) {
					continue
				}
				if _, tier := relevance(c, group, catalog); tier != "" {
					found = true
					break
				}
			}
			if found {
				matches = group
				break
			}
		}
	}
	return selectMatches(state, current, matches, wrong, nextRoute, true, candidates, catalog)
}

func selectMatches(state State, current Candidate, matches []Match, remediate, nextRoute, metadata bool, candidates []Candidate, catalog Catalog) Selection {
	out := Selection{State: clone(state), Matches: matches, Scores: []Score{}}
	if state.QuestionsAsked >= state.Config.QuestionLimit {
		out.State.StopReason, out.Reason = "question_limit", "question_limit"
		return out
	}
	eligible := map[string]Candidate{}
	for _, c := range candidates {
		if c.MaterialID != current.MaterialID && Eligible(c, state) {
			eligible[c.MaterialID.String()] = c
		}
	}
	depth := state.CurrentDepth + 1
	if remediate {
		depth = max(0, state.CurrentDepth-1)
	}
	if !nextRoute && depth <= state.Config.MaxDepthPerBranch {
		bestTier := "Z"
		alternatives := []Score{}
		for _, c := range eligible {
			if remediate && ((!metadata && c.Specificity >= current.Specificity) || (metadata && (c.Specificity > current.Specificity || c.InterviewDifficulty > current.InterviewDifficulty))) {
				continue
			}
			rel, tier := relevance(c, matches, catalog)
			if tier == "" {
				continue
			}
			score := followScore(c, current, state, rel, tier, depth)
			if remediate {
				score.Reason = "remediation"
			} else if metadata && tier == "A" {
				score.Reason = "answer_concept"
			}
			alternatives = append(alternatives, score)
			if tier < bestTier {
				bestTier = tier
				out.Scores = nil
			}
			if tier == bestTier {
				out.Scores = append(out.Scores, score)
			}
		}
		if len(out.Scores) > 0 {
			sort.Slice(out.Scores, func(i, j int) bool { return out.Scores[i].MaterialID.String() < out.Scores[j].MaterialID.String() })
			out.Scores = qualityCandidates(out.Scores)
			chosen := sample(out.Scores, &out.State)
			pick := out.Scores[chosen]
			// Preserve weaker semantic tiers too, in stable priority order. A
			// strong direct match should not erase a discovered edge alternative.
			sort.Slice(alternatives, func(i, j int) bool {
				if alternatives[i].Tier != alternatives[j].Tier {
					return alternatives[i].Tier < alternatives[j].Tier
				}
				if alternatives[i].Score != alternatives[j].Score {
					return alternatives[i].Score > alternatives[j].Score
				}
				return alternatives[i].MaterialID.String() < alternatives[j].MaterialID.String()
			})
			for _, score := range alternatives {
				if score.MaterialID == pick.MaterialID {
					continue
				}
				seen := false
				for _, f := range out.State.Frontier {
					if f.MaterialID == score.MaterialID && f.Status == "available" {
						seen = true
						break
					}
				}
				if !seen && score.Score >= pick.Score-1.75 {
					out.State.Frontier = append(out.State.Frontier, FrontierEntry{OriginMaterialID: current.MaterialID, MaterialID: score.MaterialID, RootIndex: state.CurrentRoot, Branch: state.CurrentBranch, SourceDepth: state.CurrentDepth, Reason: score.Reason, Status: "available", Score: score.Score, AddedAt: state.QuestionsAsked})
				}
			}
			out.Reason = pick.Reason
			return finishSelection(out, eligible[pick.MaterialID.String()], depth, false)
		}
	}
	// Backtrack before fallback so discovered alternatives survive a deep branch.
	if out.State.ForksUsed[state.CurrentRoot] < state.Config.MaxForksPerRoot {
		for i := range out.State.Frontier {
			f := &out.State.Frontier[i]
			if f.Status != "available" || f.RootIndex != state.CurrentRoot {
				continue
			}
			c, ok := eligible[f.MaterialID.String()]
			if !ok || f.SourceDepth+1 > state.Config.MaxDepthPerBranch {
				f.Status = "discarded"
				continue
			}
			if remediate && (f.SourceDepth+1 > state.CurrentDepth || c.Specificity > current.Specificity || c.InterviewDifficulty > current.InterviewDifficulty) {
				continue
			}
			f.Status = "used"
			out.State.ForksUsed[state.CurrentRoot]++
			out.State.CurrentBranch = fmt.Sprintf("%d.%d", state.CurrentRoot, out.State.ForksUsed[state.CurrentRoot])
			depth = f.SourceDepth + 1
			out.Reason = "frontier"
			out.Scores = []Score{followScore(c, current, state, .4, "D", depth)}
			return finishSelection(out, c, depth, false)
		}
	}
	if !remediate && !nextRoute && depth <= state.Config.MaxDepthPerBranch {
		for _, c := range eligible {
			if c.Topic == current.Topic && c.Domain == current.Domain {
				out.Scores = append(out.Scores, followScore(c, current, state, .15, "E", depth))
			}
		}
		if len(out.Scores) > 0 {
			sort.Slice(out.Scores, func(i, j int) bool { return out.Scores[i].MaterialID.String() < out.Scores[j].MaterialID.String() })
			out.Scores = qualityCandidates(out.Scores)
			pick := out.Scores[sample(out.Scores, &out.State)]
			out.Reason = "same_topic"
			return finishSelection(out, eligible[pick.MaterialID.String()], depth, false)
		}
	}
	next := SelectRoot(out.State, candidates)
	next.Matches = out.Matches
	return next
}
