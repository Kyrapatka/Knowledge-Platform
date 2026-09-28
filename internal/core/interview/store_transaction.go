package interview

import (
	"context"
	"errors"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldertemplate "github.com/Kyrapatka/knowledge-platform/internal/core/folder/template"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

var ErrNotFound = gorm.ErrRecordNotFound

type storeTx struct {
	db   *gorm.DB
	user uuid.UUID
}

func (s *Store) Transact(ctx context.Context, user uuid.UUID, fn func(Transaction) error) error {
	return s.transact(ctx, user, func(db *gorm.DB) error { return fn(&storeTx{db, user}) })
}

// TransactionStore joins an existing transaction whose caller holds the user row lock.
// Folder import uses it to keep folder, materials and profiles in a single commit.
type TransactionStore struct{ db *gorm.DB }

func NewTransactionStore(db *gorm.DB) *TransactionStore { return &TransactionStore{db} }
func (s *TransactionStore) Transact(ctx context.Context, user uuid.UUID, fn func(Transaction) error) error {
	return fn(&storeTx{s.db.WithContext(ctx), user})
}
func (t *storeTx) FolderTemplate(id uuid.UUID) (string, error) {
	return folderTemplate(t.db, t.user, id)
}
func (t *storeTx) MaterialValues(folder, id uuid.UUID) (map[string]*string, error) {
	return materialValues(t.db, t.user, folder, id)
}
func (t *storeTx) Profile(id uuid.UUID) (Profile, error) { return loadProfile(t.db, id) }
func (t *storeTx) Catalog() (Catalog, error)             { return LoadCatalog(t.db, t.user) }
func (t *storeTx) PutConcept(c Concept) error            { return saveConcept(t.db, t.user, c, true) }
func (t *storeTx) SeedProfile(key string) (Profile, error) {
	var p Profile
	err := t.db.Table("interview_question_profiles").Where("owner_id=? AND seed_key=?", t.user, key).Take(&p).Error
	return p, err
}
func (t *storeTx) SeedMaterial(id uuid.UUID) (seedMaterial, bool, error) {
	return findSeedMaterial(t.db, t.user, id)
}
func (t *storeTx) PutSeedMaterial(id, actualFolder uuid.UUID, values map[string]*string, metadata map[string]string, now time.Time, materialExists bool) error {
	db := t.db
	var err error
	if materialExists {
		err = db.Exec(`UPDATE materials SET folder_id=?,deleted_at=NULL,values=values || ?::jsonb, metadata=metadata || ?::jsonb,updated_at=? WHERE id=?`, actualFolder, jsonBytes(values), jsonBytes(metadata), now, id).Error
	} else {
		// Physical deletion normally cascades to the profile. Retaining the
		// known ID also reconciles legacy orphan profiles if one is present.
		err = db.Table("materials").Create(map[string]any{"id": id, "folder_id": actualFolder, "values": jsonBytes(values), "metadata": jsonBytes(metadata), "difficulty": "medium", "created_at": now, "updated_at": now}).Error
	}

	return err
}
func (t *storeTx) SyncBank(bank SeedBank) error {
	db, user := t.db, t.user
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

	return nil
}
func (t *storeTx) SeedFolder(d SeedDomain) (uuid.UUID, error) {
	db, user := t.db, t.user
	var row struct{ FolderID uuid.UUID }
	err := db.Table("interview_seed_folders s").Select("s.folder_id").Joins("JOIN folders f ON f.id=s.folder_id AND f.deleted_at IS NULL AND f.owner_id=? AND f.template_key='interview_questions'", user).Where("s.user_id=? AND s.domain=?", user, d.Slug).Take(&row).Error
	if errors.Is(err, ErrNotFound) {
		tmpl, err := foldertemplate.NewRegistry(foldertemplate.DefaultTemplates()).Get("interview_questions")
		if err != nil {
			return uuid.Nil, err
		}
		row.FolderID = uuid.New()
		now := time.Now().UTC()
		err = db.Table("folders").Create(map[string]any{"id": row.FolderID, "owner_id": user, "title": "Interview / " + d.Name, "description": "Interview question bank · " + d.Name, "template_key": "interview_questions", "config": jsonBytes(tmpl.Config), "config_version": 1, "training_config": jsonBytes(folderconfig.DefaultTrainingConfig("interview_questions")), "training_config_version": 1, "created_at": now, "updated_at": now}).Error
		if err != nil {
			return uuid.Nil, err
		}
		if err = db.Table("interview_seed_folders").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}, {Name: "domain"}}, DoUpdates: clause.AssignmentColumns([]string{"folder_id"})}).Create(map[string]any{"user_id": user, "domain": d.Slug, "folder_id": row.FolderID}).Error; err != nil {
			return uuid.Nil, err
		}
	} else if err != nil {
		return uuid.Nil, err
	}

	return row.FolderID, nil
}
func (t *storeTx) RecordImport(out ImportResult, bank SeedBank, domains []string, started time.Time) error {
	db, user := t.db, t.user
	return db.Table("interview_bank_import_runs").Create(map[string]any{"id": out.ImportRunID, "user_id": user, "seed_revision": bank.Version, "questions_sha256": bank.Source.QuestionsSHA256, "concepts_sha256": bank.Source.ConceptsSHA256, "domains": jsonBytes(domains), "started_at": started, "finished_at": time.Now().UTC(), "status": "completed", "created_count": out.Created, "updated_count": out.Updated, "skipped_count": out.Skipped}).Error
}
