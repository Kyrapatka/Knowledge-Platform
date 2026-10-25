package materialimport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	"regexp"
	"strings"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	v := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "_"), "_")
	if v == "" {
		h := sha256.Sum256([]byte(s))
		v = "concept_" + hex.EncodeToString(h[:8])
	}
	if len(v) > 96 {
		v = strings.TrimRight(v[:96], "_")
	}
	return v
}
func buildProfile(item Item) (interview.ProfileRequest, error) {
	p := interview.Profile{Frequency: 5, FrequencyConfidence: 0.5, InterviewDifficulty: 3, Specificity: 2, RootWeight: 5, FollowupWeight: 5, LevelMin: 1, LevelMax: 5, Status: "draft", Topic: value(item.Metadata, "topic"), Subtopic: value(item.Metadata, "category"), Concepts: []interview.QuestionConcept{}, InterviewProfiles: []string{}}
	candidates := []string{}
	if in := item.Profile; in != nil {
		if in.Frequency != nil {
			p.Frequency = *in.Frequency
		}
		if in.Difficulty != nil {
			p.InterviewDifficulty = *in.Difficulty
		}
		if in.LevelMin != nil {
			if *in.LevelMin < 1 {
				return interview.ProfileRequest{}, ErrInvalid
			}
			p.LevelMin = *in.LevelMin
		}
		if in.LevelMax != nil {
			if *in.LevelMax < 1 {
				return interview.ProfileRequest{}, ErrInvalid
			}
			p.LevelMax = *in.LevelMax
		}
		if in.Topic != "" {
			p.Topic = in.Topic
		}
		if in.Subtopic != "" {
			p.Subtopic = in.Subtopic
		}
		candidates = in.Concepts
	}
	if len(candidates) > 31 {
		return interview.ProfileRequest{}, fmt.Errorf("%w: at most 31 concepts", ErrInvalid)
	}
	if len(candidates) == 0 {
		if p.Subtopic != "" {
			candidates = []string{p.Subtopic}
		} else if p.Topic != "" {
			candidates = []string{p.Topic}
		} else {
			candidates = []string{"imported_question"}
		}
	}
	p.Domain = slug(p.Topic)
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" || len(candidate) > 200 {
			return interview.ProfileRequest{}, ErrInvalid
		}
		s := slug(candidate)
		if seen[s] {
			continue
		}
		seen[s] = true
		if len(p.Concepts) == 0 {
			p.Concepts = append(p.Concepts, interview.QuestionConcept{Slug: s, Role: "primary", Weight: 1})
		}
		p.Concepts = append(p.Concepts, interview.QuestionConcept{Slug: s, Role: "tested", Weight: 1, Ordinal: len(seen) - 1})
	}
	if interview.UsableContent(value(item.Values, "question")) && (interview.UsableContent(value(item.Values, "short_answer")) || interview.UsableContent(value(item.Values, "answer"))) {
		p.Status = "ready"
	}
	err := interview.ValidateProfile(&p)
	return interview.ProfileRequest{Profile: p}, err
}
