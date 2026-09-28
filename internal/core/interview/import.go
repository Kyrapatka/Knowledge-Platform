package interview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/seed"
	"github.com/google/uuid"
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
	// Internal committed outcome details; excluded from the existing API contract.
	CreatedMaterials []CreatedMaterial `json:"-"`
	FolderIDs        []uuid.UUID       `json:"folder_ids"`
	Created          int               `json:"created"`
	Updated          int               `json:"updated"`
	Skipped          int               `json:"skipped"`
	Revision         string            `json:"revision"`
	ImportRunID      uuid.UUID         `json:"import_run_id"`
}
type CreatedMaterial struct{ ID, FolderID uuid.UUID }
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
func importQuestions(tx Transaction, user, folder uuid.UUID, questions []BulkQuestion) (ImportResult, error) {
	out := ImportResult{FolderIDs: []uuid.UUID{folder}}
	if len(questions) == 0 || len(questions) > 1000 {
		return out, ErrInvalid
	}
	if err := ownedFolder(tx, folder); err != nil {
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
		existing, err := tx.SeedProfile(q.SeedKey)
		exists := err == nil
		if err != nil && !errors.Is(err, ErrNotFound) {
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
		var oldMaterial seedMaterial
		materialExists, restored := false, false
		if exists {
			oldMaterial, materialExists, err = tx.SeedMaterial(id)
			if err != nil {
				return out, err
			}
			restored = !materialExists || oldMaterial.DeletedAt != nil || !oldMaterial.FolderActive
			if restored {
				// Keep material identity and progress, but never resurrect a link
				// to a deleted folder. The target was validated by ownedFolder.
				actualFolder = folder
			} else {
				actualFolder = oldMaterial.FolderID
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
		if materialExists {
			for key, incoming := range values {
				// Seed answers fill empty fields; they never replace a user's
				// completed answer, including while restoring deleted materials.
				if key != "question" && incoming != nil && oldMaterial.Values[key] != nil && UsableContent(*oldMaterial.Values[key]) {
					delete(values, key)
				}
			}
		}
		metadata := map[string]string{"topic": q.Topic, "category": q.Category}
		if exists && !restored && existing.FolderID == actualFolder && !seedContentChanged(oldMaterial, values, metadata) {
			full, err := tx.Profile(id)
			if err != nil {
				return out, err
			}
			if sameSeedMetadata(full, p) {
				out.Skipped++
				continue
			}
		}
		err = tx.PutSeedMaterial(id, actualFolder, values, metadata, now, materialExists)
		if err != nil {
			return out, err
		}
		if _, err = saveProfile(tx, user, actualFolder, id, ProfileRequest{Profile: p, ExpectedVersion: existing.ProfileVersion}); err != nil {
			return out, err
		}
		if exists && !restored {
			out.Updated++
		} else {
			out.Created++
		}
		if !materialExists {
			out.CreatedMaterials = append(out.CreatedMaterials, CreatedMaterial{ID: id, FolderID: actualFolder})
		}
	}
	return out, nil
}
func (s *Service) Bulk(ctx context.Context, user, folder uuid.UUID, questions []BulkQuestion) (ImportResult, error) {
	var out ImportResult
	err := s.repository.Transact(ctx, user, func(tx Transaction) error {
		var err error
		out, err = importQuestions(tx, user, folder, questions)
		return err
	})
	return out, err
}
func (s *Service) ImportSeed(ctx context.Context, user uuid.UUID, domains []string) (ImportResult, error) {
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
	err = s.repository.Transact(ctx, user, func(tx Transaction) error {
		started := time.Now().UTC()
		if err := tx.SyncBank(bank); err != nil {
			return err
		}
		for _, d := range bank.Domains {
			if len(wanted) > 0 && !wanted[d.Slug] {
				continue
			}
			folderID, err := tx.SeedFolder(d)
			if err != nil {
				return err
			}
			qs := []BulkQuestion{}
			for _, q := range bank.Questions {
				if q.Domain == d.Slug {
					qs = append(qs, q)
				}
			}
			result, err := importQuestions(tx, user, folderID, qs)
			if err != nil {
				return err
			}
			out.FolderIDs = append(out.FolderIDs, folderID)
			out.Created += result.Created
			out.CreatedMaterials = append(out.CreatedMaterials, result.CreatedMaterials...)
			out.Updated += result.Updated
			out.Skipped += result.Skipped
		}
		out.Revision = bank.Version
		out.ImportRunID = uuid.New()
		return tx.RecordImport(out, bank, domains, started)
	})
	return out, err
}
