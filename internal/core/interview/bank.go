package interview

import (
	"sort"
	"strings"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/seed"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var InterviewProfiles = []string{"go_core", "go_middle", "go_strong_middle", "go_postgres", "backend_core", "backend_full", "distributed_backend", "infrastructure", "system_design"}

func validInterviewProfile(slug string) bool {
	if slug == "all" {
		return true
	}
	for _, p := range InterviewProfiles {
		if p == slug {
			return true
		}
	}
	return false
}
func syncProfileDictionary(db *gorm.DB) error {
	for _, slug := range InterviewProfiles {
		if err := db.Table("interview_profiles").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"slug": slug, "label": strings.ReplaceAll(slug, "_", " ")}).Error; err != nil {
			return err
		}
	}
	return nil
}

// The authoritative transport has full_answer/source; the application keeps
// exactly one editable detailed answer and one source in its established keys.
func bankFromSeed(source seed.Bank) SeedBank {
	b := SeedBank{Version: source.Revision, Source: source, Domains: []SeedDomain{}, Questions: []BulkQuestion{}, Concepts: []Concept{}, Edges: []Edge{}}
	counts := map[string]int{}
	for _, q := range source.Questions {
		x := BulkQuestion{SeedKey: q.SeedKey, Question: q.Question, ShortAnswer: &q.ShortAnswer, Answer: &q.FullAnswer, Source: &q.Source, Domain: q.Domain, Topic: q.Topic, Subtopic: q.Subtopic, Category: q.Domain, Frequency: q.Frequency, FrequencyConfidence: 1, InterviewDifficulty: q.Difficulty, Specificity: q.Specificity, RootWeight: q.RootWeight, FollowupWeight: q.FollowupWeight, LevelMin: q.LevelMin, LevelMax: q.LevelMax, InterviewProfiles: q.InterviewProfiles, Status: q.Status, SeedRevision: source.Revision}
		for _, group := range []struct {
			role  string
			slugs []string
		}{{"primary", []string{q.PrimaryConcept}}, {"tested", q.TestedConcepts}, {"answer", q.AnswerConcepts}, {"hook", q.ExpectedHooks}, {"prerequisite", q.PrerequisiteConcepts}, {"wrong_fallback", q.WrongFallback}} {
			for i, slug := range group.slugs {
				x.Concepts = append(x.Concepts, QuestionConcept{Slug: slug, Role: group.role, Weight: 1, Ordinal: i})
			}
		}
		b.Questions = append(b.Questions, x)
		counts[q.Domain]++
	}
	names := map[string]string{"go": "Go", "db": "Databases", "database": "Databases", "net": "Networking", "networking": "Networking", "git": "Git", "infra": "Infrastructure", "infrastructure": "Infrastructure", "mq": "Messaging", "algorithms": "Algorithms", "system_design": "System design", "architecture": "Architecture"}
	for slug, count := range counts {
		name := names[slug]
		if name == "" {
			name = strings.ReplaceAll(slug, "_", " ")
		}
		b.Domains = append(b.Domains, SeedDomain{slug, name, count})
	}
	sort.Slice(b.Domains, func(i, j int) bool { return b.Domains[i].Slug < b.Domains[j].Slug })
	return b
}

func syncBankConcepts(db *gorm.DB, user uuid.UUID, b seed.Bank) error {
	rows := make([]map[string]any, 0, len(b.Concepts))
	for _, c := range b.Concepts {
		rows = append(rows, map[string]any{"id": uuid.New(), "owner_id": user, "slug": c.Slug, "display_name": c.Label, "domain": "", "topic": ""})
	}
	if err := db.Table("interview_concepts").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_id"}, {Name: "slug"}}, DoNothing: true}).CreateInBatches(rows, 200).Error; err != nil {
		return err
	}
	var stored []struct {
		ID   uuid.UUID
		Slug string
	}
	if err := db.Table("interview_concepts").Select("id,slug").Where("owner_id=?", user).Find(&stored).Error; err != nil {
		return err
	}
	ids := map[string]uuid.UUID{}
	for _, c := range stored {
		ids[c.Slug] = c.ID
	}
	canonicalIDs := []uuid.UUID{}
	for _, c := range b.Concepts {
		canonicalIDs = append(canonicalIDs, ids[c.Slug])
	}
	if err := db.Table("interview_concept_aliases").Where("concept_id IN ? AND seed_managed=TRUE", canonicalIDs).Delete(&struct{}{}).Error; err != nil {
		return err
	}
	// Quarantine known homonyms, including aliases left by the previous importer.
	blocked := map[string]bool{}
	for _, a := range b.AmbiguousAliases {
		blocked[a.NormalizedAlias] = true
	}
	var old []struct {
		ID                    uuid.UUID
		NormalizedAlias, Slug string
	}
	if err := db.Table("interview_concept_aliases a").Select("a.id,a.normalized_alias,c.slug").Joins("JOIN interview_concepts c ON c.id=a.concept_id").Where("c.owner_id=?", user).Find(&old).Error; err != nil {
		return err
	}
	remove := []uuid.UUID{}
	for _, a := range old {
		global, exists := b.AliasMap[a.NormalizedAlias]
		if blocked[a.NormalizedAlias] || (exists && global != a.Slug) {
			remove = append(remove, a.ID)
		}
	}
	if len(remove) > 0 {
		if err := db.Table("interview_concept_aliases").Where("id IN ?", remove).Delete(&struct{}{}).Error; err != nil {
			return err
		}
	}
	aliases := []map[string]any{}
	for alias, slug := range b.AliasMap {
		aliases = append(aliases, map[string]any{"id": uuid.New(), "concept_id": ids[slug], "alias": alias, "normalized_alias": alias, "language": "any", "weight": 1., "whole_word": true, "constraints": jsonBytes(AliasConstraints{}), "seed_managed": true})
	}
	if len(aliases) > 0 {
		if err := db.Table("interview_concept_aliases").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "concept_id"}, {Name: "normalized_alias"}, {Name: "language"}}, DoNothing: true}).CreateInBatches(aliases, 200).Error; err != nil {
			return err
		}
	}
	return db.Table("interview_bank_alias_policies").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_id"}}, DoUpdates: clause.AssignmentColumns([]string{"seed_revision", "scoped_aliases", "ambiguous_aliases", "updated_at"})}).Create(map[string]any{"owner_id": user, "seed_revision": b.Revision, "scoped_aliases": jsonBytes(b.ScopedAliases), "ambiguous_aliases": jsonBytes(b.AmbiguousAliases), "updated_at": time.Now().UTC()}).Error
}
