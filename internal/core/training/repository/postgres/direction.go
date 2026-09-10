package postgres

import "github.com/google/uuid"

func (t *runtimeTx) LastEnglishDirection(material uuid.UUID) (string, error) {
	var directions []string
	err := t.db.Table("training_events").Where("user_id=? AND material_id=? AND undone_at IS NULL AND direction IN ('foreign','native') AND action IN ('correct','wrong')", t.user, material).Order("progress_version_after DESC, created_at DESC, id").Limit(1).Pluck("direction", &directions).Error
	if len(directions) == 0 {
		return "", err
	}
	return directions[0], err
}
