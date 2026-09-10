package postgres

import (
	"encoding/json"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
	"github.com/google/uuid"
	"time"
)

func (t *runtimeTx) SaveUndo(s model.UndoSnapshot) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err = t.db.Table("training_undo").Create(map[string]any{"event_id": s.EventID, "user_id": t.user, "plan_id": s.PlanID, "session_id": s.SessionID, "material_id": s.MaterialID, "expected_version": s.ExpectedVersion, "snapshot": raw}).Error; err != nil {
		return err
	}
	return t.db.Exec("DELETE FROM training_undo WHERE user_id=? AND id NOT IN (SELECT id FROM training_undo WHERE user_id=? ORDER BY id DESC LIMIT 3)", t.user, t.user).Error
}

func (t *runtimeTx) UndoHistory() ([]model.UndoSnapshot, error) {
	var rows []struct {
		Snapshot        []byte
		ExpectedVersion int
	}
	err := t.db.Table("training_undo").Where("user_id=?", t.user).Order("id DESC").Limit(3).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]model.UndoSnapshot, 0, len(rows))
	for _, row := range rows {
		var s model.UndoSnapshot
		if err = json.Unmarshal(row.Snapshot, &s); err != nil {
			return nil, err
		}
		s.ExpectedVersion = row.ExpectedVersion
		out = append(out, s)
	}
	return out, nil
}

func (t *runtimeTx) RestoreUndo(s model.UndoSnapshot, version int, now time.Time) error {
	// All callers hold the same per-user training lock as an answer command.
	if _, err := t.Session(s.SessionID); err != nil {
		return err
	}
	if err := t.db.Table("training_session_items").Where("session_id=?", s.SessionID).Updates(map[string]any{"state": "completed", "presentation": nil}).Error; err != nil {
		return err
	}
	for index, item := range s.Items {
		if index == 0 && item.Presentation != nil {
			item.Presentation.ID = uuid.New()
			item.Presentation.ProgressVersion = version
		}
		if err := t.SaveItem(item); err != nil {
			return err
		}
	}
	if err := t.db.Table("training_sessions").Where("id=? AND user_id=?", s.SessionID, t.user).Updates(map[string]any{"status": "active", "finished_at": nil}).Error; err != nil {
		return err
	}
	if err := t.db.Table("training_plans").Where("id=? AND user_id=?", s.PlanID, t.user).Updates(map[string]any{"status": "active", "updated_at": now}).Error; err != nil {
		return err
	}
	result := t.db.Table("training_events").Where("id=? AND user_id=? AND undone_at IS NULL", s.EventID, t.user).Update("undone_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return repository.ErrConflict
	}
	if err := t.db.Exec("DELETE FROM training_undo WHERE user_id=? AND event_id=?", t.user, s.EventID).Error; err != nil {
		return err
	}
	// Keep versions monotonic while making consecutive undos of the same card valid.
	return t.db.Table("training_undo").Where("user_id=? AND material_id=? AND plan_id=? AND expected_version=?", t.user, s.MaterialID, s.PlanID, s.Before.Version).Update("expected_version", version).Error
}
