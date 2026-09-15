// Package seed validates and embeds the authoritative Interview Question Bank.
package seed

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// Version identifies the source corpus, independently of user edits and profiles.
const Version = "2026-09-14-chat-restored-1"

// QuestionCount is the number of question profiles in the supplied specification.
const QuestionCount = 368

const ConceptCount = 1470

//go:embed interview_questions.json
var questionsJSON []byte

//go:embed interview_concepts.json
var conceptsJSON []byte

//go:embed bank.json
var bank []byte

// Raw returns the legacy corpus solely for retaining its existing concept edges.
// Deprecated: question and concept imports must use Load, never this old corpus.
func Raw() []byte {
	return append([]byte(nil), bank...)
}

// RawFiles returns copies of the supplied JSON bytes, without reformatting them.
func RawFiles() (questions, concepts []byte) {
	return append([]byte(nil), questionsJSON...), append([]byte(nil), conceptsJSON...)
}

type Question struct {
	SeedKey              string   `json:"seed_key"`
	Question             string   `json:"question"`
	ShortAnswer          string   `json:"short_answer"`
	FullAnswer           string   `json:"full_answer"`
	Source               string   `json:"source"`
	Domain               string   `json:"domain"`
	Topic                string   `json:"topic"`
	Subtopic             string   `json:"subtopic"`
	PrimaryConcept       string   `json:"primary_concept"`
	TestedConcepts       []string `json:"tested_concepts"`
	AnswerConcepts       []string `json:"answer_concepts"`
	ExpectedHooks        []string `json:"expected_hooks"`
	PrerequisiteConcepts []string `json:"prerequisite_concepts"`
	WrongFallback        []string `json:"wrong_fallback"`
	Frequency            int      `json:"frequency"`
	Difficulty           int      `json:"difficulty"`
	Specificity          int      `json:"specificity"`
	RootWeight           int      `json:"root_weight"`
	FollowupWeight       int      `json:"followup_weight"`
	LevelMin             int      `json:"level_min"`
	LevelMax             int      `json:"level_max"`
	InterviewProfiles    []string `json:"interview_profiles"`
	Status               string   `json:"status"`
}

type Concept struct {
	Slug    string   `json:"slug"`
	Label   string   `json:"label"`
	Aliases []string `json:"aliases"`
}

type AliasConstraints struct {
	RequiresAny    []string `json:"requires_any,omitempty"`
	RequiresDomain string   `json:"requires_domain,omitempty"`
}

// ScopedAlias is quarantined from global lookup, including entries whose
// constraints are empty: those are unresolved homonyms, not safe synonyms.
type ScopedAlias struct {
	ConceptSlug     string           `json:"concept_slug"`
	Alias           string           `json:"alias"`
	Language        string           `json:"language"`
	Weight          float64          `json:"weight"`
	WholeWord       bool             `json:"whole_word"`
	Constraints     AliasConstraints `json:"constraints"`
	Provenance      string           `json:"provenance"`
	NormalizedAlias string           `json:"normalized_alias"`
}

type AmbiguousAlias struct {
	NormalizedAlias string   `json:"normalized_alias"`
	ConceptSlugs    []string `json:"concept_slugs"`
}

type Report struct {
	Revision             string         `json:"revision"`
	QuestionsSHA256      string         `json:"questions_sha256"`
	ConceptsSHA256       string         `json:"concepts_sha256"`
	Questions            int            `json:"questions"`
	Concepts             int            `json:"concepts"`
	Aliases              int            `json:"aliases"`
	ScopedAliases        int            `json:"scoped_aliases"`
	AmbiguousAliases     int            `json:"ambiguous_aliases"`
	ConceptsWithoutAlias int            `json:"concepts_without_alias"`
	Relations            int            `json:"relations"`
	PrimaryRelations     int            `json:"primary_relations"`
	ProfileMemberships   int            `json:"profile_memberships"`
	CountsByPrefix       map[string]int `json:"counts_by_prefix"`
}

type Bank struct {
	Revision          string
	Version           string
	QuestionsSHA256   string
	ConceptsSHA256    string
	Questions         []Question
	Concepts          []Concept
	AliasMap          map[string]string
	ScopedAliases     []ScopedAlias
	AmbiguousAliases  []AmbiguousAlias
	CountsByPrefix    map[string]int
	Report            Report
	questionSchema    int
	conceptSchema     int
	declaredQuestions int
	declaredConcepts  int
	declaredAliases   int
	conceptRevision   string
}

func Load() (Bank, error) { return Parse(questionsJSON, conceptsJSON) }

// LoadFiles validates both files entirely in memory; it never contacts a DB.
func LoadFiles(questionsPath, conceptsPath string) (Bank, error) {
	q, err := os.ReadFile(questionsPath)
	if err != nil {
		return Bank{}, fmt.Errorf("read questions: %w", err)
	}
	c, err := os.ReadFile(conceptsPath)
	if err != nil {
		return Bank{}, fmt.Errorf("read concepts: %w", err)
	}
	return Parse(q, c)
}

func ValidateFiles(questionsPath, conceptsPath string) (Report, error) {
	b, err := LoadFiles(questionsPath, conceptsPath)
	return b.Report, err
}

func Parse(questionBytes, conceptBytes []byte) (Bank, error) {
	if err := validateJSON(questionBytes, "questions"); err != nil {
		return Bank{}, err
	}
	if err := validateJSON(conceptBytes, "concepts"); err != nil {
		return Bank{}, err
	}
	if err := validateRequiredFields(questionBytes, conceptBytes); err != nil {
		return Bank{}, err
	}
	var q struct {
		SchemaVersion int            `json:"schema_version"`
		Revision      string         `json:"seed_revision"`
		Count         int            `json:"question_count"`
		Counts        map[string]int `json:"counts_by_prefix"`
		Questions     []Question     `json:"questions"`
	}
	var c struct {
		SchemaVersion    int               `json:"schema_version"`
		Revision         string            `json:"seed_revision"`
		Count            int               `json:"concept_count"`
		AliasCount       int               `json:"alias_count"`
		Concepts         []Concept         `json:"concepts"`
		AliasMap         map[string]string `json:"alias_map"`
		ScopedAliases    []ScopedAlias     `json:"scoped_aliases"`
		AmbiguousAliases []AmbiguousAlias  `json:"ambiguous_aliases"`
	}
	if err := json.Unmarshal(questionBytes, &q); err != nil {
		return Bank{}, fmt.Errorf("questions: %w", err)
	}
	if err := json.Unmarshal(conceptBytes, &c); err != nil {
		return Bank{}, fmt.Errorf("concepts: %w", err)
	}
	b := Bank{Revision: q.Revision, Version: q.Revision, Questions: q.Questions, Concepts: c.Concepts, AliasMap: c.AliasMap, ScopedAliases: c.ScopedAliases, AmbiguousAliases: c.AmbiguousAliases, CountsByPrefix: q.Counts, questionSchema: q.SchemaVersion, conceptSchema: c.SchemaVersion, declaredQuestions: q.Count, declaredConcepts: c.Count, declaredAliases: c.AliasCount, conceptRevision: c.Revision}
	qh, ch := sha256.Sum256(questionBytes), sha256.Sum256(conceptBytes)
	b.QuestionsSHA256, b.ConceptsSHA256 = hex.EncodeToString(qh[:]), hex.EncodeToString(ch[:])
	if err := b.Validate(); err != nil {
		return Bank{}, err
	}
	b.Report = b.report()
	return b, nil
}

func (b Bank) report() Report {
	r := Report{Revision: b.Revision, QuestionsSHA256: b.QuestionsSHA256, ConceptsSHA256: b.ConceptsSHA256, Questions: len(b.Questions), Concepts: len(b.Concepts), Aliases: len(b.AliasMap), ScopedAliases: len(b.ScopedAliases), AmbiguousAliases: len(b.AmbiguousAliases), PrimaryRelations: len(b.Questions), CountsByPrefix: map[string]int{}}
	for k, v := range b.CountsByPrefix {
		r.CountsByPrefix[k] = v
	}
	for _, q := range b.Questions {
		r.Relations += len(q.TestedConcepts) + len(q.AnswerConcepts) + len(q.ExpectedHooks) + len(q.PrerequisiteConcepts) + len(q.WrongFallback)
		for _, p := range q.InterviewProfiles {
			if p != "all" {
				r.ProfileMemberships++
			}
		}
	}
	for _, c := range b.Concepts {
		if len(c.Aliases) == 0 {
			r.ConceptsWithoutAlias++
		}
	}
	return r
}
