package interview

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"time"
)

// Service owns interview invariants and import orchestration; repositories own atomicity.
type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Profile(ctx context.Context, user, folder, material uuid.UUID) (Profile, error) {
	var out Profile
	err := s.repository.Transact(ctx, user, func(tx Transaction) error {
		if _, err := ownedValues(tx, folder, material); err != nil {
			return err
		}
		var err error
		out, err = tx.Profile(material)
		if errors.Is(err, ErrNotFound) {
			out = Profile{MaterialID: material, FolderID: folder, Frequency: 5, FrequencyConfidence: .5, InterviewDifficulty: 2, Specificity: 2, RootWeight: 5, FollowupWeight: 5, LevelMin: 1, LevelMax: 5, Status: "draft", Concepts: []QuestionConcept{}, InterviewProfiles: []string{}}
			return nil
		}
		return err
	})
	return out, err
}

func saveProfile(tx Transaction, user, folder, material uuid.UUID, req ProfileRequest) (Profile, error) {
	p := req.Profile
	if req.ExpectedVersion < 0 {
		return p, ErrInvalid
	}
	if err := p.validate(); err != nil {
		return p, err
	}
	values, err := ownedValues(tx, folder, material)
	if err != nil {
		return p, err
	}
	nonempty := func(k string) bool { return values[k] != nil && UsableContent(*values[k]) }
	if p.Status == "ready" && (!nonempty("question") || (!nonempty("answer") && !nonempty("short_answer"))) {
		return p, fmt.Errorf("%w: ready questions need a question and a usable answer", ErrInvalid)
	}
	old, err := tx.Profile(material)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return p, err
	}
	if old.ProfileVersion != req.ExpectedVersion {
		return p, ErrConflict
	}
	now := time.Now().UTC()
	p.MaterialID = material
	p.FolderID = folder
	p.OwnerID = user
	p.ProfileVersion = old.ProfileVersion + 1
	p.CreatedAt = old.CreatedAt
	p.UpdatedAt = now
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if old.SeedKey != nil {
		p.SeedKey = old.SeedKey
	}
	return p, tx.PutProfile(p)
}

func (s *Service) SaveProfile(ctx context.Context, user, folder, material uuid.UUID, req ProfileRequest) (Profile, error) {
	var out Profile
	err := s.repository.Transact(ctx, user, func(tx Transaction) error {
		var err error
		out, err = saveProfile(tx, user, folder, material, req)
		return err
	})
	return out, err
}

func ownedFolder(tx Transaction, folder uuid.UUID) error {
	template, err := tx.FolderTemplate(folder)
	if err != nil {
		return err
	}
	if template != "interview_questions" {
		return fmt.Errorf("%w: choose an interview folder", ErrInvalid)
	}
	return nil
}
func ownedValues(tx Transaction, folder, material uuid.UUID) (map[string]*string, error) {
	if err := ownedFolder(tx, folder); err != nil {
		return nil, err
	}
	return tx.MaterialValues(folder, material)
}
func (s *Service) Catalog(ctx context.Context, user uuid.UUID) (Catalog, error) {
	var out Catalog
	err := s.repository.Transact(ctx, user, func(tx Transaction) error { var err error; out, err = tx.Catalog(); return err })
	return out, err
}
func validateConcept(c *Concept) error {
	if !slugPattern.MatchString(c.Slug) || len(c.DisplayName) > 200 || len(c.Aliases) > 100 || len(c.Domain) > 96 || len(c.Topic) > 200 {
		return ErrInvalid
	}

	for i := range c.Aliases {
		a := &c.Aliases[i]
		key := normalizeAlias(a.Alias)
		if key == "" || len(key) > 240 || len(a.Language) > 24 || len(a.Constraints.RequiresAny) > 20 || len(a.Constraints.RequiresDomain) > 96 {
			return ErrInvalid
		}
		if a.Language == "" {
			a.Language = "any"
		}
		if a.Weight == 0 {
			a.Weight = 1
		}
		if a.Weight < 0 || a.Weight > 2 {
			return ErrInvalid
		}

	}
	return nil
}
func (s *Service) SaveConcept(ctx context.Context, user uuid.UUID, c Concept) (Concept, error) {
	if err := validateConcept(&c); err != nil {
		return Concept{}, err
	}
	var out Concept
	err := s.repository.Transact(ctx, user, func(tx Transaction) error {
		if err := tx.PutConcept(c); err != nil {
			return err
		}
		catalog, err := tx.Catalog()
		if err != nil {
			return err
		}
		for _, v := range catalog.Concepts {
			if v.Slug == c.Slug {
				out = v
				return nil
			}
		}
		return ErrNotFound
	})
	return out, err
}
