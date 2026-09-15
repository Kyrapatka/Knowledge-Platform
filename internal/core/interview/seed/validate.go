package seed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var prefixCounts = map[string]int{"GO": 130, "DB": 40, "NET": 35, "GIT": 18, "INF": 50, "MQ": 25, "SRV": 20, "ARC": 35, "ALG": 15}
var profiles = map[string]bool{"go_core": true, "go_middle": true, "go_strong_middle": true, "go_postgres": true, "backend_core": true, "backend_full": true, "distributed_backend": true, "infrastructure": true, "system_design": true, "all": true}
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)

// NormalizeAlias implements the source-file policy without semantic rewriting.
// Slugs and punctuation stay distinct: canonical metadata resolves by slug.
func NormalizeAlias(value string) string {
	value = cases.Fold().String(norm.NFKC.String(value))
	value = strings.Map(func(r rune) rune {
		switch r {
		case 'ё':
			return 'е'
		case '‐', '‑', '‒', '–', '—', '―', '−', '﹘', '﹣', '－':
			return '-'
		}
		return r
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func (b Bank) Validate() error {
	if b.questionSchema != 1 || b.conceptSchema != 1 {
		return fmt.Errorf("unsupported schema versions: questions=%d concepts=%d", b.questionSchema, b.conceptSchema)
	}
	if b.Revision == "" || b.Revision != b.conceptRevision {
		return fmt.Errorf("question and concept revisions must match")
	}
	if len(b.Questions) != QuestionCount || b.declaredQuestions != len(b.Questions) {
		return fmt.Errorf("expected %d questions, got %d (manifest %d)", QuestionCount, len(b.Questions), b.declaredQuestions)
	}
	if len(b.Concepts) != ConceptCount || b.declaredConcepts != len(b.Concepts) {
		return fmt.Errorf("expected %d concepts, got %d (manifest %d)", ConceptCount, len(b.Concepts), b.declaredConcepts)
	}
	if len(b.AliasMap) != b.declaredAliases {
		return fmt.Errorf("alias_count=%d, actual=%d", b.declaredAliases, len(b.AliasMap))
	}
	if len(b.CountsByPrefix) != len(prefixCounts) {
		return fmt.Errorf("invalid prefix count manifest")
	}
	for prefix, count := range prefixCounts {
		if b.CountsByPrefix[prefix] != count {
			return fmt.Errorf("prefix %s: expected %d", prefix, count)
		}
	}
	concepts := map[string]bool{}
	global := map[string]string{}
	for _, c := range b.Concepts {
		if !slugPattern.MatchString(c.Slug) || concepts[c.Slug] {
			return fmt.Errorf("invalid or duplicate concept slug %q", c.Slug)
		}
		if !validText(c.Label) || c.Aliases == nil {
			return fmt.Errorf("concept %s requires label and aliases array (which may be empty)", c.Slug)
		}
		concepts[c.Slug] = true
		local := map[string]bool{}
		for _, alias := range c.Aliases {
			n := NormalizeAlias(alias)
			if !validText(alias) || n == "" || local[n] {
				return fmt.Errorf("concept %s has empty or duplicate normalized alias %q", c.Slug, alias)
			}
			local[n] = true
			if other, ok := global[n]; ok && other != c.Slug {
				return fmt.Errorf("global alias %q collides between %s and %s", n, other, c.Slug)
			}
			global[n] = c.Slug
		}
	}
	if len(global) != len(b.AliasMap) {
		return fmt.Errorf("concept aliases and alias_map counts differ: %d vs %d", len(global), len(b.AliasMap))
	}
	for alias, slug := range b.AliasMap {
		if alias == "" || NormalizeAlias(alias) != alias || !concepts[slug] || global[alias] != slug {
			return fmt.Errorf("alias_map entry %q -> %q is not a normalized, unambiguous concept alias", alias, slug)
		}
	}
	seen := map[string]bool{}
	for _, q := range b.Questions {
		if seen[q.SeedKey] {
			return fmt.Errorf("duplicate seed_key %q", q.SeedKey)
		}
		seen[q.SeedKey] = true
		if !validText(q.Question) || strings.TrimSpace(q.Question) == "." {
			return fmt.Errorf("%s question must contain its real question text", q.SeedKey)
		}
		if q.ShortAnswer != "." || q.FullAnswer != "." || q.Source != "." || q.Status != "draft" {
			return fmt.Errorf("%s must retain dot answer/source placeholders and draft status", q.SeedKey)
		}
		if !slugPattern.MatchString(q.Domain) || !slugPattern.MatchString(q.Topic) || !slugPattern.MatchString(q.Subtopic) {
			return fmt.Errorf("%s requires valid domain/topic/subtopic", q.SeedKey)
		}
		if q.Frequency < 1 || q.Frequency > 10 || q.Difficulty < 1 || q.Difficulty > 5 || q.Specificity < 1 || q.Specificity > 5 || q.RootWeight < 0 || q.RootWeight > 10 || q.FollowupWeight < 0 || q.FollowupWeight > 10 || q.LevelMin < 1 || q.LevelMax > 5 || q.LevelMin > q.LevelMax {
			return fmt.Errorf("%s numeric metadata outside supported ranges", q.SeedKey)
		}
		if !concepts[q.PrimaryConcept] {
			return fmt.Errorf("%s has missing primary concept %q", q.SeedKey, q.PrimaryConcept)
		}
		for _, relation := range []struct {
			name  string
			slugs []string
		}{{"tested", q.TestedConcepts}, {"answer", q.AnswerConcepts}, {"hook", q.ExpectedHooks}, {"prerequisite", q.PrerequisiteConcepts}, {"wrong_fallback", q.WrongFallback}} {
			if relation.slugs == nil || (relation.name == "tested" && len(relation.slugs) == 0) {
				return fmt.Errorf("%s requires %s array", q.SeedKey, relation.name)
			}
			links := map[string]bool{}
			for _, slug := range relation.slugs {
				if !concepts[slug] || links[slug] {
					return fmt.Errorf("%s %s: missing or duplicate concept %q", q.SeedKey, relation.name, slug)
				}
				links[slug] = true
			}
		}
		memberships := map[string]bool{}
		if len(q.InterviewProfiles) == 0 {
			return fmt.Errorf("%s has no interview profiles", q.SeedKey)
		}
		for _, p := range q.InterviewProfiles {
			if !profiles[p] || memberships[p] {
				return fmt.Errorf("%s has invalid or duplicate profile %q", q.SeedKey, p)
			}
			memberships[p] = true
		}
	}
	for prefix, count := range prefixCounts {
		for i := 1; i <= count; i++ {
			key := fmt.Sprintf("%s%03d", prefix, i)
			if !seen[key] {
				return fmt.Errorf("missing expected seed_key %s", key)
			}
		}
	}
	return b.validateScopedAliases(concepts, global)
}

func (b Bank) validateScopedAliases(concepts map[string]bool, global map[string]string) error {
	ambiguous := map[string]map[string]bool{}
	for _, a := range b.AmbiguousAliases {
		if a.NormalizedAlias == "" || NormalizeAlias(a.NormalizedAlias) != a.NormalizedAlias || len(a.ConceptSlugs) < 2 || ambiguous[a.NormalizedAlias] != nil || global[a.NormalizedAlias] != "" {
			return fmt.Errorf("invalid ambiguous alias %q", a.NormalizedAlias)
		}
		members := map[string]bool{}
		for _, slug := range a.ConceptSlugs {
			if !concepts[slug] || members[slug] {
				return fmt.Errorf("ambiguous alias %q has invalid concept %q", a.NormalizedAlias, slug)
			}
			members[slug] = true
		}
		ambiguous[a.NormalizedAlias] = members
	}
	seen := map[string]bool{}
	scopedByName := map[string]map[string]bool{}
	for _, a := range b.ScopedAliases {
		n := NormalizeAlias(a.Alias)
		if !validText(a.Alias) || n != a.NormalizedAlias || !concepts[a.ConceptSlug] || (global[n] != "" && global[n] != a.ConceptSlug) {
			return fmt.Errorf("invalid scoped alias %q -> %q", a.Alias, a.ConceptSlug)
		}
		if a.Language != "any" && a.Language != "en" && a.Language != "ru" {
			return fmt.Errorf("scoped alias %q has unsupported language %q", a.Alias, a.Language)
		}
		if math.IsNaN(a.Weight) || math.IsInf(a.Weight, 0) || a.Weight <= 0 || a.Weight > 1 {
			return fmt.Errorf("scoped alias %q has invalid weight", a.Alias)
		}
		if a.Constraints.RequiresDomain != "" && !slugPattern.MatchString(a.Constraints.RequiresDomain) {
			return fmt.Errorf("scoped alias %q has invalid domain constraint", a.Alias)
		}
		contexts := map[string]bool{}
		for _, context := range a.Constraints.RequiresAny {
			normalized := NormalizeAlias(context)
			if normalized == "" || contexts[normalized] {
				return fmt.Errorf("scoped alias %q has empty or duplicate context constraint", a.Alias)
			}
			contexts[normalized] = true
		}
		// Source entries without conditions are allowed only when explicitly
		// declared ambiguous. They remain quarantined; no automatic sense merge.
		if a.Constraints.RequiresDomain == "" && len(a.Constraints.RequiresAny) == 0 && !ambiguous[n][a.ConceptSlug] {
			return fmt.Errorf("unconstrained scoped alias %q is not declared ambiguous", a.Alias)
		}
		constraintBytes, _ := json.Marshal(a.Constraints)
		key := a.ConceptSlug + "\x00" + n + "\x00" + a.Language + "\x00" + string(constraintBytes)
		if seen[key] {
			return fmt.Errorf("duplicate scoped alias %q for %s", a.Alias, a.ConceptSlug)
		}
		seen[key] = true
		if scopedByName[n] == nil {
			scopedByName[n] = map[string]bool{}
		}
		scopedByName[n][a.ConceptSlug] = true
	}
	for name, members := range ambiguous {
		if len(scopedByName[name]) != len(members) {
			return fmt.Errorf("ambiguous alias %q does not match scoped senses", name)
		}
		for slug := range members {
			if !scopedByName[name][slug] {
				return fmt.Errorf("ambiguous alias %q missing scoped sense %s", name, slug)
			}
		}
	}
	return nil
}

func validText(s string) bool {
	return strings.TrimSpace(s) != "" && utf8.ValidString(s) && !strings.ContainsRune(s, '\uFFFD')
}

// Duplicate object keys must not silently overwrite a question metric or alias.
func validateJSON(data []byte, source string) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("%s is not valid UTF-8", source)
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := walkJSON(d, source); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("%s contains trailing JSON content", source)
	}
	return nil
}

func walkJSON(d *json.Decoder, path string) error {
	t, err := d.Token()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			keyToken, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return fmt.Errorf("%s contains duplicate object key %q", path, key)
			}
			seen[key] = true
			if err := walkJSON(d, path+"."+key); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; d.More(); i++ {
			if err := walkJSON(d, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter in %s", path)
	}
	_, err = d.Token()
	return err
}

func validateRequiredFields(q, c []byte) error {
	for _, source := range []struct {
		data  []byte
		name  string
		top   []string
		lists map[string][]string
	}{
		{q, "questions", []string{"schema_version", "seed_revision", "question_count", "counts_by_prefix", "questions"}, map[string][]string{"questions": {"seed_key", "question", "short_answer", "full_answer", "source", "domain", "topic", "subtopic", "primary_concept", "tested_concepts", "answer_concepts", "expected_hooks", "prerequisite_concepts", "wrong_fallback", "frequency", "difficulty", "specificity", "root_weight", "followup_weight", "level_min", "level_max", "interview_profiles", "status"}}},
		{c, "concepts", []string{"schema_version", "seed_revision", "concept_count", "alias_count", "concepts", "alias_map", "scoped_aliases", "ambiguous_aliases"}, map[string][]string{"concepts": {"slug", "label", "aliases"}, "scoped_aliases": {"concept_slug", "alias", "language", "weight", "whole_word", "constraints", "normalized_alias"}, "ambiguous_aliases": {"normalized_alias", "concept_slugs"}}},
	} {
		var root map[string]json.RawMessage
		if err := json.Unmarshal(source.data, &root); err != nil {
			return fmt.Errorf("%s: %w", source.name, err)
		}
		if err := requireFields(root, source.top, source.name); err != nil {
			return err
		}
		for list, fields := range source.lists {
			var rows []map[string]json.RawMessage
			if err := json.Unmarshal(root[list], &rows); err != nil {
				return fmt.Errorf("%s: %w", list, err)
			}
			for i, row := range rows {
				if err := requireFields(row, fields, fmt.Sprintf("%s[%d]", list, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func requireFields(row map[string]json.RawMessage, fields []string, path string) error {
	for _, field := range fields {
		value, ok := row[field]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("%s missing required field %s", path, field)
		}
	}
	return nil
}
