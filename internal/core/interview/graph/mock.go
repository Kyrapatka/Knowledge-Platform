package graph

import (
	"fmt"
	"github.com/google/uuid"
	"slices"
)

func mockStop(s State, reason string) Selection {
	s.StopReason = reason
	return Selection{State: s, Reason: reason, Matches: []Match{}, Scores: []Score{}}
}
func deepScore(c Candidate, s State, wrong bool) float64 {
	if s.Plan == nil || wrong {
		return 0
	}
	p := s.Plan.Strategy
	return p.DifficultyBoost*float64(c.InterviewDifficulty-1)/4 + p.RarityBoost*float64(10-c.Frequency)/9
}
func selectMockRoot(state State, candidates []Candidate) Selection {
	s := clone(state)
	if s.Plan == nil {
		p, err := BuildPlan(s, candidates)
		if err != nil {
			return mockStop(s, "no_ready_roots")
		}
		s.Plan = &p
	}
	slot := len(s.CompletedRootIDs)
	if slot >= len(s.Plan.Slots) {
		return mockStop(s, "roots_completed")
	}
	// Eligibility is independent of presentation history; novelty is relaxed only
	// after every alternative in this slot has been shown (tiny banks remain usable).
	base := s
	base.AskedMaterialIDs = nil
	pool := []Candidate{}
	all := []Candidate{}
	desired := s.Plan.Slots[slot]
	allowed := map[string]bool{}
	for _, t := range s.Plan.Topics {
		allowed[t.Key] = t.Weight > 0
	}
	for _, c := range candidates {
		if c.RootWeight > 0 && Eligible(c, base) && allowed[Classify(c)] {
			all = append(all, c)
			if Classify(c) == desired {
				pool = append(pool, c)
			}
		}
	}
	if len(pool) == 0 {
		pool = all
	}
	if len(pool) == 0 {
		return mockStop(s, "no_ready_roots")
	}
	unseen := []Candidate{}
	for _, c := range pool {
		if !slices.Contains(s.AskedMaterialIDs, c.MaterialID) {
			unseen = append(unseen, c)
		}
	}
	if len(unseen) == 0 { // Exhausted slot: prefer unseen selected topics before cycling.
		for _, c := range all {
			if !slices.Contains(s.AskedMaterialIDs, c.MaterialID) {
				unseen = append(unseen, c)
			}
		}
	}
	if len(unseen) > 0 {
		pool = unseen
	} else if len(pool) > 1 {
		filtered := []Candidate{}
		for _, c := range pool {
			if c.MaterialID != s.CurrentRootID {
				filtered = append(filtered, c)
			}
		}
		pool = filtered
	}
	out := Selection{State: s, Matches: []Match{}, Scores: []Score{}}
	byID := map[uuid.UUID]Candidate{}
	for _, c := range pool {
		score := rootScore(c, s)
		score.Score += deepScore(c, s, false)
		areaPenalty := 0.
		if slices.Contains(s.RootAreas, RootArea(c)) {
			areaPenalty = 8
		}
		conceptPenalty := 0.
		for _, l := range c.Concepts {
			if (l.Role == "primary" || l.Role == "tested") && slices.Contains(s.RootConcepts, l.Slug) {
				conceptPenalty += 2
			}
		}
		score.Score -= areaPenalty + conceptPenalty
		score.Components["root_area_penalty"] = areaPenalty
		score.Components["root_concept_penalty"] = conceptPenalty
		score.Components["deep_preference"] = deepScore(c, s, false)
		out.Scores = append(out.Scores, score)
		byID[c.MaterialID] = c
	}
	out.Scores = qualityCandidates(out.Scores)
	pick := byID[out.Scores[sample(out.Scores, &out.State)].MaterialID]
	out.State.CurrentRoot = slot + 1
	out.State.RootsUsed = slot + 1
	out.State.CurrentRootID = pick.MaterialID
	out.State.BranchAnswered = 0
	out.State.CurrentBranch = fmt.Sprintf("%d.0", slot+1)
	out.State.Frontier = nil
	out.State.ForksUsed[slot+1] = 0
	out.State.ShownRootIDs = append(out.State.ShownRootIDs, pick.MaterialID)
	out.State.RootAreas = append(out.State.RootAreas, RootArea(pick))
	for _, l := range pick.Concepts {
		if l.Role == "primary" || l.Role == "tested" {
			out.State.RootConcepts = append(out.State.RootConcepts, l.Slug)
		}
	}
	if len(out.State.RootConcepts) > 12 {
		out.State.RootConcepts = out.State.RootConcepts[len(out.State.RootConcepts)-12:]
	}
	out.Reason = "root"
	return finishSelection(out, pick, 0, true)
}

func selectMockNext(state State, current Candidate, action string, candidates []Candidate, catalog Catalog) Selection {
	s := clone(state)
	if action == "next_route" || action == "next_root" {
		s.SkippedRootIDs = append(s.SkippedRootIDs, s.CurrentRootID)
		return selectMockRoot(s, candidates)
	}
	s.AnsweredQuestions++
	s.BranchAnswered++
	complete := func() Selection {
		s.CompletedRootIDs = append(s.CompletedRootIDs, s.CurrentRootID)
		if s.AnsweredQuestions >= s.Config.QuestionLimit {
			return mockStop(s, "question_limit")
		}
		return selectMockRoot(s, candidates)
	}
	if s.Plan == nil {
		return complete()
	}
	remainingSlots := len(s.Plan.Slots) - len(s.CompletedRootIDs)
	remainingQuestions := s.Config.QuestionLimit - s.AnsweredQuestions
	if remainingQuestions <= remainingSlots-1 {
		return complete()
	}
	target := (remainingQuestions + s.BranchAnswered + remainingSlots - 1) / remainingSlots
	// Leave enough budget for every planned root; permit uneven branch lengths.
	if s.BranchAnswered >= target+1 || s.CurrentDepth >= s.Config.MaxDepthPerBranch {
		return complete()
	}
	continueProbability := 1 - (1-s.Plan.Strategy.FollowProbability)/(1+s.Plan.Strategy.RootSwitchPenalty)
	if s.BranchAnswered >= max(2, target-1) && nextRandom(&s) > continueProbability {
		return complete()
	}
	if s.BranchAnswered >= target && remainingSlots > 1 {
		return complete()
	}
	wrong := action == "wrong"
	matches := []Match{}
	roles := []string{"answer", "hook", "primary", "tested"}
	if wrong {
		roles = []string{"wrong_fallback", "prerequisite", "primary", "tested"}
	}
	for i, role := range roles {
		for _, l := range current.Concepts {
			if l.Role == role {
				w := l.Weight
				if w <= 0 {
					w = 1
				}
				matches = append(matches, Match{Slug: l.Slug, Strength: w / (1 + float64(i)*.4), Source: role})
			}
		}
		// Preserve metadata routing priority. Root primary/tested concepts are a
		// fallback, not competition for an explicit answer or remediation route.
		found := false
		for _, c := range candidates {
			if !Eligible(c, s) || c.MaterialID == current.MaterialID {
				continue
			}
			if wrong && (c.InterviewDifficulty > current.InterviewDifficulty || c.Specificity > current.Specificity) {
				continue
			}
			if _, tier := relevance(c, matches, catalog); tier != "" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if s.Plan.Strategy.ConceptHops > 1 && !wrong {
		extra := []Match{}
		for _, m := range matches {
			for _, e := range catalog.Edges {
				if e.From == m.Slug {
					extra = append(extra, Match{Slug: e.To, Strength: m.Strength * .45, Source: "related"})
				}
			}
		}
		matches = append(matches, extra...)
	}
	out := Selection{State: s, Matches: matches, Scores: []Score{}}
	byID := map[uuid.UUID]Candidate{}
	for _, c := range candidates {
		if !Eligible(c, s) || c.MaterialID == current.MaterialID {
			continue
		}
		if wrong && (c.InterviewDifficulty > current.InterviewDifficulty || c.Specificity > current.Specificity) {
			continue
		}
		rel, tier := relevance(c, matches, catalog)
		if tier == "" && RootArea(c) == RootArea(current) {
			rel, tier = .3, "E"
		}
		if tier == "" {
			continue
		} // A follow-up must have a semantic connection.
		score := followScore(c, current, s, rel, tier, s.CurrentDepth+1)
		boost := deepScore(c, s, wrong)
		score.Score += boost + s.Plan.Strategy.RootSwitchPenalty
		score.Components["deep_preference"] = boost
		if wrong {
			score.Reason = "remediation"
		}
		out.Scores = append(out.Scores, score)
		byID[c.MaterialID] = c
	}
	if len(out.Scores) == 0 {
		// Reuse the existing frontier as a bounded backtracking stack. This keeps
		// a deep branch coherent when its current chain has no unused successor.
		if out.State.ForksUsed[s.CurrentRoot] < s.Config.MaxForksPerRoot {
			byID := map[uuid.UUID]Candidate{}
			for _, c := range candidates {
				byID[c.MaterialID] = c
			}
			for i := range out.State.Frontier {
				f := &out.State.Frontier[i]
				c, ok := byID[f.MaterialID]
				if !ok || f.Status != "available" || f.RootIndex != s.CurrentRoot || !Eligible(c, s) || f.SourceDepth+1 > s.Config.MaxDepthPerBranch {
					continue
				}
				if wrong && (c.InterviewDifficulty > current.InterviewDifficulty || c.Specificity > current.Specificity) {
					continue
				}
				f.Status = "used"
				out.State.ForksUsed[s.CurrentRoot]++
				out.State.CurrentBranch = fmt.Sprintf("%d.%d", s.CurrentRoot, out.State.ForksUsed[s.CurrentRoot])
				out.Reason = "frontier"
				out.Scores = []Score{followScore(c, current, s, .4, "D", f.SourceDepth+1)}
				return finishSelection(out, c, f.SourceDepth+1, false)
			}
		}
		return complete()
	}
	alternatives := append([]Score(nil), out.Scores...)
	out.Scores = qualityCandidates(out.Scores)
	chosen := out.Scores[sample(out.Scores, &out.State)]
	for _, score := range alternatives {
		if score.MaterialID == chosen.MaterialID || score.Score < chosen.Score-1.75 {
			continue
		}
		exists := false
		for _, f := range out.State.Frontier {
			if f.MaterialID == score.MaterialID && f.Status == "available" {
				exists = true
				break
			}
		}
		if !exists {
			out.State.Frontier = append(out.State.Frontier, FrontierEntry{OriginMaterialID: current.MaterialID, MaterialID: score.MaterialID, RootIndex: s.CurrentRoot, Branch: s.CurrentBranch, SourceDepth: s.CurrentDepth, Reason: score.Reason, Status: "available", Score: score.Score, AddedAt: s.QuestionsAsked})
		}
	}
	out.Reason = chosen.Reason
	return finishSelection(out, byID[chosen.MaterialID], s.CurrentDepth+1, false)
}
