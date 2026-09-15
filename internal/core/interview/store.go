package interview

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db} }
func (s *Store) transact(ctx context.Context, user uuid.UUID, fn func(*gorm.DB) error) error {
	if user == uuid.Nil {
		return gorm.ErrRecordNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		var identity struct{ ID uuid.UUID }
		if err := db.Table("users").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", user).Take(&identity).Error; err != nil {
			return err
		}
		return fn(db)
	})
}
func ownedFolder(db *gorm.DB, user, folder uuid.UUID) error {
	var f struct{ TemplateKey string }
	if err := db.Table("folders").Select("template_key").Where("id=? AND owner_id=? AND deleted_at IS NULL", folder, user).Take(&f).Error; err != nil {
		return err
	}
	if f.TemplateKey != "interview_questions" {
		return fmt.Errorf("%w: choose an interview folder", ErrInvalid)
	}
	return nil
}
func materialValues(db *gorm.DB, user, folder, material uuid.UUID) (map[string]*string, error) {
	if err := ownedFolder(db, user, folder); err != nil {
		return nil, err
	}
	var row struct{ Values []byte }
	if err := db.Table("materials").Select("values").Where("id=? AND folder_id=? AND deleted_at IS NULL", material, folder).Take(&row).Error; err != nil {
		return nil, err
	}
	var v map[string]*string
	err := json.Unmarshal(row.Values, &v)
	return v, err
}
func loadProfile(db *gorm.DB, material uuid.UUID) (Profile, error) {
	var p Profile
	if err := db.Table("interview_question_profiles").Where("material_id=?", material).Take(&p).Error; err != nil {
		return p, err
	}
	p.Concepts = []QuestionConcept{}
	err := db.Table("interview_question_concepts q").Select("c.slug,q.role,q.weight,q.ordinal").Joins("JOIN interview_concepts c ON c.id=q.concept_id").Where("q.material_id=?", material).Order("q.role,q.ordinal,c.slug").Scan(&p.Concepts).Error
	if err == nil {
		err = db.Table("interview_question_memberships").Where("material_id=?", material).Order("profile_slug").Pluck("profile_slug", &p.InterviewProfiles).Error
	}
	return p, err
}
func (s *Store) Profile(ctx context.Context, user, folder, material uuid.UUID) (Profile, error) {
	var out Profile
	err := s.transact(ctx, user, func(db *gorm.DB) error {
		if _, err := materialValues(db, user, folder, material); err != nil {
			return err
		}
		var err error
		out, err = loadProfile(db, material)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			out = Profile{MaterialID: material, FolderID: folder, Frequency: 5, FrequencyConfidence: .5, InterviewDifficulty: 2, Specificity: 2, RootWeight: 5, FollowupWeight: 5, LevelMin: 1, LevelMax: 5, Status: "draft", Concepts: []QuestionConcept{}, InterviewProfiles: []string{}}
			return nil
		}
		return err
	})
	return out, err
}
func ensureConcept(db *gorm.DB, user uuid.UUID, slug, domain, topic string) (uuid.UUID, error) {
	if !slugPattern.MatchString(slug) {
		return uuid.Nil, ErrInvalid
	}
	id := uuid.New()
	row := map[string]any{"id": id, "owner_id": user, "slug": slug, "display_name": strings.ReplaceAll(slug, "_", " "), "domain": domain, "topic": topic}
	if err := db.Table("interview_concepts").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_id"}, {Name: "slug"}}, DoNothing: true}).Create(row).Error; err != nil {
		return uuid.Nil, err
	}
	var found struct{ ID uuid.UUID }
	err := db.Table("interview_concepts").Where("owner_id=? AND slug=?", user, slug).Take(&found).Error
	return found.ID, err
}
func saveProfile(db *gorm.DB, user, folder, material uuid.UUID, req ProfileRequest) (Profile, error) {
	p := req.Profile
	if req.ExpectedVersion < 0 {
		return p, ErrInvalid
	}
	if err := p.validate(); err != nil {
		return p, err
	}
	values, err := materialValues(db, user, folder, material)
	if err != nil {
		return p, err
	}
	nonempty := func(k string) bool { return values[k] != nil && UsableContent(*values[k]) }
	if p.Status == "ready" && (!nonempty("question") || (!nonempty("answer") && !nonempty("short_answer"))) {
		return p, fmt.Errorf("%w: ready questions need a question and a usable answer", ErrInvalid)
	}
	old, err := loadProfile(db, material)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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
	if err = db.Table("interview_question_profiles").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "material_id"}}, UpdateAll: true}).Create(&p).Error; err != nil {
		return p, err
	}
	if err = db.Table("interview_question_concepts").Where("material_id=?", material).Delete(&struct{}{}).Error; err != nil {
		return p, err
	}
	for _, c := range p.Concepts {
		id, err := ensureConcept(db, user, c.Slug, p.Domain, "")
		if err != nil {
			return p, err
		}
		if err = db.Table("interview_question_concepts").Create(map[string]any{"material_id": material, "concept_id": id, "role": c.Role, "weight": c.Weight, "ordinal": c.Ordinal}).Error; err != nil {
			return p, err
		}
	}
	if err = syncProfileDictionary(db); err != nil {
		return p, err
	}
	if err = db.Table("interview_question_memberships").Where("material_id=?", material).Delete(&struct{}{}).Error; err != nil {
		return p, err
	}
	for _, slug := range p.InterviewProfiles {
		if slug == "all" {
			continue
		}
		if !validInterviewProfile(slug) {
			return p, ErrInvalid
		}
		if err = db.Table("interview_question_memberships").Create(map[string]any{"material_id": material, "profile_slug": slug}).Error; err != nil {
			return p, err
		}
	}
	return p, nil
}
func (s *Store) SaveProfile(ctx context.Context, user, folder, material uuid.UUID, req ProfileRequest) (Profile, error) {
	var out Profile
	err := s.transact(ctx, user, func(db *gorm.DB) error {
		var err error
		out, err = saveProfile(db, user, folder, material, req)
		return err
	})
	return out, err
}

func normalizeAlias(s string) string {
	s = strings.ToLower(norm.NFKC.String(s))
	s = strings.NewReplacer("ё", "е", "–", "-", "—", "-", "‑", "-").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// Catalog is account scoped, including edges and aliases shared across folders.
func LoadCatalog(db *gorm.DB, user uuid.UUID) (Catalog, error) {
	out := Catalog{Concepts: []Concept{}, Edges: []Edge{}}
	if err := db.Table("interview_concepts").Where("owner_id=?", user).Order("slug").Find(&out.Concepts).Error; err != nil {
		return out, err
	}
	var aliases []struct {
		ConceptID       uuid.UUID
		Alias, Language string
		Weight          float64
		WholeWord       bool
		Constraints     []byte
	}
	if err := db.Table("interview_concept_aliases a").Select("a.*").Joins("JOIN interview_concepts c ON c.id=a.concept_id").Where("c.owner_id=?", user).Order("a.normalized_alias").Scan(&aliases).Error; err != nil {
		return out, err
	}
	byID := map[uuid.UUID]int{}
	for i := range out.Concepts {
		byID[out.Concepts[i].ID] = i
		out.Concepts[i].Aliases = []Alias{}
	}
	for _, a := range aliases {
		var c AliasConstraints
		if err := json.Unmarshal(a.Constraints, &c); err != nil {
			return out, err
		}
		i := byID[a.ConceptID]
		out.Concepts[i].Aliases = append(out.Concepts[i].Aliases, Alias{a.Alias, a.Language, a.Weight, a.WholeWord, c})
	}
	err := db.Table("interview_concept_edges e").Select(`a.slug AS "from",b.slug AS "to",e.relation,e.weight`).Joins("JOIN interview_concepts a ON a.id=e.from_concept_id").Joins("JOIN interview_concepts b ON b.id=e.to_concept_id").Where("a.owner_id=? AND b.owner_id=?", user, user).Order("a.slug,b.slug,e.relation").Scan(&out.Edges).Error
	return out, err
}
func (s *Store) Catalog(ctx context.Context, user uuid.UUID) (Catalog, error) {
	return LoadCatalog(s.db.WithContext(ctx), user)
}
func saveConcept(db *gorm.DB, user uuid.UUID, c Concept, overwrite bool) error {
	if !slugPattern.MatchString(c.Slug) || len(c.DisplayName) > 200 || len(c.Aliases) > 100 || len(c.Domain) > 96 || len(c.Topic) > 200 {
		return ErrInvalid
	}
	id, err := ensureConcept(db, user, c.Slug, c.Domain, c.Topic)
	if err != nil {
		return err
	}
	if overwrite {
		var old struct{ Version int }
		if err = db.Table("interview_concepts").Where("id=?", id).Take(&old).Error; err != nil {
			return err
		}
		if c.Version > 0 && c.Version != old.Version {
			return ErrConflict
		}
		if err = db.Table("interview_concepts").Where("id=?", id).Updates(map[string]any{"display_name": c.DisplayName, "domain": c.Domain, "topic": c.Topic, "version": old.Version + 1, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err = db.Table("interview_concept_aliases").Where("concept_id=?", id).Delete(&struct{}{}).Error; err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, a := range c.Aliases {
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
		if seen[key+":"+a.Language] {
			continue
		}
		seen[key+":"+a.Language] = true
		row := map[string]any{"id": uuid.New(), "concept_id": id, "alias": a.Alias, "normalized_alias": key, "language": a.Language, "weight": a.Weight, "whole_word": a.WholeWord, "constraints": jsonBytes(a.Constraints)}
		if err = db.Table("interview_concept_aliases").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "concept_id"}, {Name: "normalized_alias"}, {Name: "language"}}, DoNothing: true}).Create(row).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) SaveConcept(ctx context.Context, user uuid.UUID, c Concept) (Concept, error) {
	var out Concept
	err := s.transact(ctx, user, func(db *gorm.DB) error {
		if err := saveConcept(db, user, c, true); err != nil {
			return err
		}
		catalog, err := LoadCatalog(db, user)
		if err != nil {
			return err
		}
		for _, v := range catalog.Concepts {
			if v.Slug == c.Slug {
				out = v
				return nil
			}
		}
		return gorm.ErrRecordNotFound
	})
	return out, err
}
