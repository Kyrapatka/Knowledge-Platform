package interview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/seed"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BulkQuestion struct {
	SeedKey             string            `json:"seed_key"`
	Question            string            `json:"question"`
	Answer              *string           `json:"answer,omitempty"`
	ShortAnswer         *string           `json:"short_answer,omitempty"`
	Source              *string           `json:"source,omitempty"`
	Subtopic            string            `json:"subtopic"`
	FollowupWeight      int               `json:"followup_weight"`
	LevelMin            int               `json:"level_min"`
	LevelMax            int               `json:"level_max"`
	InterviewProfiles   []string          `json:"interview_profiles"`
	SeedRevision        string            `json:"seed_revision"`
	Topic               string            `json:"topic"`
	Category            string            `json:"category"`
	Domain              string            `json:"domain"`
	Frequency           int               `json:"frequency"`
	FrequencyConfidence float64           `json:"frequency_confidence"`
	InterviewDifficulty int               `json:"interview_difficulty"`
	Specificity         int               `json:"specificity"`
	RootWeight          int               `json:"root_weight"`
	Status              string            `json:"status"`
	Concepts            []QuestionConcept `json:"concepts"`
}
type ImportResult struct {
	FolderIDs   []uuid.UUID `json:"folder_ids"`
	Created     int         `json:"created"`
	Updated     int         `json:"updated"`
	Skipped     int         `json:"skipped"`
	Revision    string      `json:"revision"`
	ImportRunID uuid.UUID   `json:"import_run_id"`
}
type SeedDomain struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	QuestionCount int    `json:"question_count"`
}
type SeedBank struct {
	Version   string         `json:"version"`
	Domains   []SeedDomain   `json:"domains"`
	Questions []BulkQuestion `json:"questions"`
	Concepts  []Concept      `json:"concepts"`
	Edges     []Edge         `json:"edges"`
	Source    seed.Bank      `json:"-"`
}

func ReadSeed() (SeedBank, error) {
	var b SeedBank
	var raw struct {
		Edges []struct {
			From     string  `json:"from_slug"`
			To       string  `json:"to_slug"`
			Relation string  `json:"relation"`
			Weight   float64 `json:"weight"`
		} `json:"edges"`
	}
	source, err := seed.Load()
	if err != nil {
		return b, err
	}
	b = bankFromSeed(source)
	if err := json.Unmarshal(seed.Raw(), &raw); err != nil {
		return b, err
	}
	b.Edges = make([]Edge, 0, len(raw.Edges))
	for _, e := range raw.Edges {
		if e.From == "" || e.To == "" {
			return b, fmt.Errorf("seed edge has empty endpoint")
		}
		b.Edges = append(b.Edges, Edge{e.From, e.To, e.Relation, e.Weight})
	}
	return b, nil
}
func importQuestions(db *gorm.DB, user, folder uuid.UUID, questions []BulkQuestion) (ImportResult, error) {
	out := ImportResult{FolderIDs: []uuid.UUID{folder}}
	if len(questions) == 0 || len(questions) > 1000 {
		return out, ErrInvalid
	}
	if err := ownedFolder(db, user, folder); err != nil {
		return out, err
	}
	seen := map[string]bool{}
	for _, q := range questions {
		if strings.TrimSpace(q.SeedKey) == "" || len(q.SeedKey) > 100 || strings.TrimSpace(q.Question) == "" || len(q.Question) > 20000 || len(q.Topic) > 200 || len(q.Category) > 96 || seen[q.SeedKey] {
			return out, fmt.Errorf("%w: invalid or duplicate seed question", ErrInvalid)
		}
		seen[q.SeedKey] = true
		if q.Status == "" {
			q.Status = "draft"
		}
		p := Profile{SeedKey: &q.SeedKey, Domain: q.Domain, Frequency: q.Frequency, FrequencyConfidence: q.FrequencyConfidence, InterviewDifficulty: q.InterviewDifficulty, Specificity: q.Specificity, RootWeight: q.RootWeight, Status: q.Status, Concepts: q.Concepts, FollowupWeight: q.FollowupWeight, LevelMin: q.LevelMin, LevelMax: q.LevelMax, Topic: q.Topic, Subtopic: q.Subtopic, InterviewProfiles: q.InterviewProfiles, SeedRevision: q.SeedRevision}
		if err := p.validate(); err != nil {
			return out, err
		}
		var existing Profile
		err := db.Table("interview_question_profiles").Where("owner_id=? AND seed_key=?", user, q.SeedKey).Take(&existing).Error
		exists := err == nil
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return out, err
		}
		actualFolder := folder
		if exists {
			actualFolder = existing.FolderID
			if existing.Status != "draft" {
				p.Status = existing.Status
			}
		}
		id := existing.MaterialID
		if exists {
			var n int64
			if err = db.Table("materials m").Joins("JOIN folders f ON f.id=m.folder_id").Where("m.id=? AND m.deleted_at IS NULL AND f.deleted_at IS NULL", id).Count(&n).Error; err != nil {
				return out, err
			}
			if n == 0 {
				out.Skipped++
				continue
			}
		} else {
			id = uuid.New()
		}
		now := time.Now().UTC()
		values := map[string]*string{"question": &q.Question}
		if q.Answer != nil {
			values["answer"] = q.Answer
		}
		if q.ShortAnswer != nil {
			values["short_answer"] = q.ShortAnswer
		}
		if q.Source != nil {
			values["sources"] = q.Source
		}
		if exists {
			oldValues, err := materialValues(db, user, actualFolder, id)
			if err != nil {
				return out, err
			}
			for key, incoming := range values {
				if incoming != nil && !UsableContent(*incoming) && oldValues[key] != nil && UsableContent(*oldValues[key]) {
					delete(values, key)
				}
			}
		}
		metadata := map[string]string{"topic": q.Topic, "category": q.Category}
		if exists {
			err = db.Exec(`UPDATE materials SET values=values || ?::jsonb, metadata=metadata || ?::jsonb,updated_at=? WHERE id=?`, jsonBytes(values), jsonBytes(metadata), now, id).Error
		} else {
			err = db.Table("materials").Create(map[string]any{"id": id, "folder_id": folder, "values": jsonBytes(values), "metadata": jsonBytes(metadata), "difficulty": "medium", "created_at": now, "updated_at": now}).Error
		}
		if err != nil {
			return out, err
		}
		if _, err = saveProfile(db, user, actualFolder, id, ProfileRequest{Profile: p, ExpectedVersion: existing.ProfileVersion}); err != nil {
			return out, err
		}
		if exists {
			out.Updated++
		} else {
			out.Created++
		}
	}
	return out, nil
}
func (s *Store) Bulk(ctx context.Context, user, folder uuid.UUID, questions []BulkQuestion) (ImportResult, error) {
	var out ImportResult
	err := s.transact(ctx, user, func(db *gorm.DB) error {
		var err error
		out, err = importQuestions(db, user, folder, questions)
		return err
	})
	return out, err
}
func (s *Store) ImportSeed(ctx context.Context, user uuid.UUID, domains []string) (ImportResult, error) {
	out := ImportResult{FolderIDs: []uuid.UUID{}}
	bank, err := ReadSeed()
	if err != nil {
		return out, err
	}
	wanted := map[string]bool{}
	for _, domain := range domains {
		if wanted[domain] {
			return out, ErrInvalid
		}
		wanted[domain] = true
	}
	allowed := map[string]bool{}
	for _, d := range bank.Domains {
		allowed[d.Slug] = true
	}
	for d := range wanted {
		if !allowed[d] {
			return out, ErrInvalid
		}
	}
	err = s.transact(ctx, user, func(db *gorm.DB) error {
		started := time.Now().UTC()
		if err := syncBankConcepts(db, user, bank.Source); err != nil {
			return err
		}
		if err := syncProfileDictionary(db); err != nil {
			return err
		}
		for _, e := range bank.Edges {
			var endpoints []Concept
			if err := db.Table("interview_concepts").Where("owner_id=? AND slug IN ?", user, []string{e.From, e.To}).Find(&endpoints).Error; err != nil {
				return err
			}
			var a, b uuid.UUID
			for _, c := range endpoints {
				if c.Slug == e.From {
					a = c.ID
				}
				if c.Slug == e.To {
					b = c.ID
				}
			}
			if a == uuid.Nil || b == uuid.Nil {
				continue
			}
			if e.Weight == 0 {
				e.Weight = 1
			}
			if err := db.Table("interview_concept_edges").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"from_concept_id": a, "to_concept_id": b, "relation": e.Relation, "weight": e.Weight}).Error; err != nil {
				return err
			}
		}
		for _, d := range bank.Domains {
			if len(wanted) > 0 && !wanted[d.Slug] {
				continue
			}
			var row struct{ FolderID uuid.UUID }
			err := db.Table("interview_seed_folders s").Select("s.folder_id").Joins("JOIN folders f ON f.id=s.folder_id AND f.deleted_at IS NULL").Where("s.user_id=? AND s.domain=?", user, d.Slug).Take(&row).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tmpl, err := foldertemplate.NewRegistry(foldertemplate.DefaultTemplates()).Get("interview_questions")
				if err != nil {
					return err
				}
				row.FolderID = uuid.New()
				now := time.Now().UTC()
				err = db.Table("folders").Create(map[string]any{"id": row.FolderID, "owner_id": user, "title": "Interview / " + d.Name, "description": "Interview question bank · " + d.Name, "template_key": "interview_questions", "config": jsonBytes(tmpl.Config), "config_version": 1, "training_config": jsonBytes(folderconfig.DefaultTrainingConfig("interview_questions")), "training_config_version": 1, "created_at": now, "updated_at": now}).Error
				if err != nil {
					return err
				}
				if err = db.Table("interview_seed_folders").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "domain"}}, DoUpdates: clause.AssignmentColumns([]string{"folder_id"})}).Create(map[string]any{"user_id": user, "domain": d.Slug, "folder_id": row.FolderID}).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			qs := []BulkQuestion{}
			for _, q := range bank.Questions {
				if q.Domain == d.Slug {
					qs = append(qs, q)
				}
			}
			result, err := importQuestions(db, user, row.FolderID, qs)
			if err != nil {
				return err
			}
			out.FolderIDs = append(out.FolderIDs, row.FolderID)
			out.Created += result.Created
			out.Updated += result.Updated
			out.Skipped += result.Skipped
		}
		out.Revision = bank.Version
		out.ImportRunID = uuid.New()
		return db.Table("interview_bank_import_runs").Create(map[string]any{"id": out.ImportRunID, "user_id": user, "seed_revision": bank.Version, "questions_sha256": bank.Source.QuestionsSHA256, "concepts_sha256": bank.Source.ConceptsSHA256, "domains": jsonBytes(domains), "started_at": started, "finished_at": time.Now().UTC(), "status": "completed", "created_count": out.Created, "updated_count": out.Updated, "skipped_count": out.Skipped}).Error
	})
	return out, err
}
