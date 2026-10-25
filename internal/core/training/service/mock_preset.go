package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/Kyrapatka/knowledge-platform/internal/core/interview/graph"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"sort"
	"strings"
	"unicode/utf8"
)

type SaveMockPresetRequest struct {
	Name            string        `json:"name"`
	FolderIDs       []uuid.UUID   `json:"folder_ids"`
	Settings        *graph.Config `json:"settings"`
	ExpectedVersion int           `json:"expected_version"`
}

func resolvePresetFolders(tx repository.Tx, p model.MockPreset) (model.MockPreset, error) {
	stored := p.FolderIDs
	p.FolderIDs = []uuid.UUID{}
	p.UnavailableFolderIDs = []uuid.UUID{}
	for _, id := range stored {
		f, err := tx.Folder(id)
		if errors.Is(err, repository.ErrNotFound) {
			p.UnavailableFolderIDs = append(p.UnavailableFolderIDs, id)
			continue
		}
		if err != nil {
			return p, err
		}
		if f.TemplateKey != "interview_questions" {
			p.UnavailableFolderIDs = append(p.UnavailableFolderIDs, id)
			continue
		}
		p.FolderIDs = append(p.FolderIDs, id)
	}
	return p, nil
}
func validatePreset(tx repository.Tx, req SaveMockPresetRequest) (SaveMockPresetRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 120 || len(req.FolderIDs) > 100 {
		return req, ErrInvalid
	}
	if req.Settings != nil {
		if err := req.Settings.Validate(); err != nil {
			return req, errors.Join(ErrInvalid, err)
		}
	}
	ids := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for _, id := range req.FolderIDs {
		if id == uuid.Nil {
			return req, ErrInvalid
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		f, err := tx.Folder(id)
		if err != nil {
			return req, err
		}
		if f.TemplateKey != "interview_questions" {
			return req, fmt.Errorf("%w: presets require interview folders", ErrInvalid)
		}
		ids = append(ids, id)
	}
	req.FolderIDs = ids
	return req, nil
}
func (s *Service) MockPresets(ctx context.Context, user uuid.UUID) ([]model.MockPreset, error) {
	var out []model.MockPreset
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		var err error
		out, err = tx.MockPresets().List()
		if err != nil {
			return err
		}
		for i := range out {
			out[i], err = resolvePresetFolders(tx, out[i])
			if err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}
func (s *Service) MockPreset(ctx context.Context, user, id uuid.UUID) (model.MockPreset, error) {
	var out model.MockPreset
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		var err error
		out, err = tx.MockPresets().Get(id)
		if err != nil {
			return err
		}
		out, err = resolvePresetFolders(tx, out)
		return err
	})
	return out, err
}
func (s *Service) SaveMockPreset(ctx context.Context, user, id uuid.UUID, req SaveMockPresetRequest) (model.MockPreset, error) {
	var out model.MockPreset
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		var err error
		if id != uuid.Nil {
			out, err = tx.MockPresets().Get(id)
			if err != nil {
				return err
			}
			if out.Version != req.ExpectedVersion {
				return repository.ErrConflict
			}
		}
		req, err = validatePreset(tx, req)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		if id == uuid.Nil {
			out = model.MockPreset{ID: uuid.New(), UserID: user, Version: 1, CreatedAt: now}
		} else {
			out.Version++
		}
		out.Name, out.FolderIDs, out.Settings, out.UpdatedAt = req.Name, req.FolderIDs, req.Settings, now
		out.UnavailableFolderIDs = []uuid.UUID{}
		if id == uuid.Nil {
			return tx.MockPresets().Create(out)
		}
		return tx.MockPresets().Update(out, req.ExpectedVersion)
	})
	return out, err
}
func (s *Service) DeleteMockPreset(ctx context.Context, user, id uuid.UUID) error {
	return s.transact(ctx, user, func(tx repository.Tx) error { return tx.MockPresets().Delete(id) })
}
func (s *Service) DuplicateMockPreset(ctx context.Context, user, id uuid.UUID) (model.MockPreset, error) {
	var out model.MockPreset
	err := s.transact(ctx, user, func(tx repository.Tx) error {
		p, err := tx.MockPresets().Get(id)
		if err != nil {
			return err
		}
		p, err = resolvePresetFolders(tx, p)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		name := []rune(p.Name)
		if len(name) > 113 {
			name = name[:113]
		}
		out = model.MockPreset{ID: uuid.New(), UserID: user, Name: string(name) + " (copy)", FolderIDs: p.FolderIDs, Settings: p.Settings, Version: 1, CreatedAt: now, UpdatedAt: now, UnavailableFolderIDs: []uuid.UUID{}}
		return tx.MockPresets().Create(out)
	})
	return out, err
}

// Resolve preset settings only for a single preset, then apply explicit session
// settings. Multiple presets merge folder identities, never their settings.
func resolveMockRequest(tx repository.Tx, req StartGraphRequest) (StartGraphRequest, error) {
	if len(req.PresetIDs) == 0 {
		return req, nil
	}
	if len(req.PresetIDs) > 100 || len(req.Sources) > 0 {
		return req, ErrInvalid
	}
	seenPresets := map[uuid.UUID]bool{}
	seenFolders := map[uuid.UUID]bool{}
	ids := []uuid.UUID{}
	var single *graph.Config
	for _, id := range req.PresetIDs {
		if id == uuid.Nil {
			return req, ErrInvalid
		}
		if seenPresets[id] {
			continue
		}
		seenPresets[id] = true
		ids = append(ids, id)
		p, err := tx.MockPresets().Get(id)
		if err != nil {
			return req, err
		}
		p, err = resolvePresetFolders(tx, p)
		if err != nil {
			return req, err
		}
		single = p.Settings
		for _, fid := range p.FolderIDs {
			if !seenFolders[fid] {
				seenFolders[fid] = true
				req.Sources = append(req.Sources, model.SessionSource{FolderID: fid})
			}
		}
	}
	if len(req.Sources) == 0 || len(req.Sources) > 100 {
		return req, fmt.Errorf("%w: select 1..100 available interview folders", ErrInvalid)
	}
	sort.Slice(req.Sources, func(i, j int) bool { return req.Sources[i].FolderID.String() < req.Sources[j].FolderID.String() })
	req.PresetIDs = ids
	if len(ids) == 1 && req.Config == nil {
		req.Config = single
	}
	return req, nil
}
