package interview

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// seedMaterial includes soft-deleted rows deliberately. Querying a raw table
// (rather than a GORM model with DeletedAt) avoids an implicit active-only scope.
// Call only inside Store.transact, which holds the importing user's row lock.
type seedMaterial struct {
	FolderID     uuid.UUID
	DeletedAt    *time.Time
	FolderActive bool
	Values       map[string]*string
	Metadata     map[string]any
}

func findSeedMaterial(db *gorm.DB, user, id uuid.UUID) (seedMaterial, bool, error) {
	var row struct {
		FolderID         uuid.UUID
		OwnerID          *uuid.UUID
		DeletedAt        *time.Time
		FolderActive     bool
		Values, Metadata []byte
	}
	err := db.Table("materials m").Select("m.folder_id,m.deleted_at,m.values,m.metadata,f.owner_id,(f.id IS NOT NULL AND f.deleted_at IS NULL AND f.template_key='interview_questions') AS folder_active").Joins("LEFT JOIN folders f ON f.id=m.folder_id").Where("m.id=?", id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return seedMaterial{}, false, nil
	}
	if err != nil {
		return seedMaterial{}, false, err
	}
	// A corrupt cross-account profile must never move another user's material.
	if row.OwnerID != nil && *row.OwnerID != user {
		return seedMaterial{}, false, ErrConflict
	}
	out := seedMaterial{FolderID: row.FolderID, DeletedAt: row.DeletedAt, FolderActive: row.FolderActive}
	if err = json.Unmarshal(row.Values, &out.Values); err != nil {
		return out, false, err
	}
	if err = json.Unmarshal(row.Metadata, &out.Metadata); err != nil {
		return out, false, err
	}
	return out, true, nil
}

func sameSeedMetadata(a, b Profile) bool {
	normalize := func(p Profile) Profile {
		p.MaterialID, p.FolderID, p.OwnerID = uuid.Nil, uuid.Nil, uuid.Nil
		p.ProfileVersion = 0
		p.CreatedAt = time.Time{}
		p.UpdatedAt = time.Time{}
		p.Concepts = append([]QuestionConcept{}, p.Concepts...)
		sort.Slice(p.Concepts, func(i, j int) bool {
			a, b := p.Concepts[i], p.Concepts[j]
			if a.Role != b.Role {
				return a.Role < b.Role
			}
			if a.Ordinal != b.Ordinal {
				return a.Ordinal < b.Ordinal
			}
			return a.Slug < b.Slug
		})
		members := []string{}
		for _, v := range p.InterviewProfiles {
			if v != "all" {
				members = append(members, v)
			}
		}
		sort.Strings(members)
		p.InterviewProfiles = members
		return p
	}
	return reflect.DeepEqual(normalize(a), normalize(b))
}

func seedContentChanged(old seedMaterial, values map[string]*string, metadata map[string]string) bool {
	for key, v := range values {
		if !reflect.DeepEqual(old.Values[key], v) {
			return true
		}
	}
	for key, v := range metadata {
		if old.Metadata[key] != v {
			return true
		}
	}
	return false
}
