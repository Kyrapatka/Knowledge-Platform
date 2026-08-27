package workshop

import (
	"errors"
	"testing"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
)

func TestValidateConfig(t *testing.T) {
	oldConfig := folderconfig.FolderConfig{
		Schema: folderconfig.MaterialSchema{
			Fields: []folderconfig.FieldDefinition{
				{
					Key:      "foreign",
					Label:    "Foreign",
					Required: true,
					Active:   true,
				},
				{
					Key:      "native",
					Label:    "Native",
					Required: true,
					Active:   true,
				},
			},
		},

		MetadataSchema: folderconfig.MetadataSchema{
			Fields: []folderconfig.FieldDefinition{
				{
					Key:      "level",
					Label:    "Level",
					Required: false,
					Active:   true,
				},
			},
		},

		Card: folderconfig.CardConfig{
			QuestionFields: []string{
				"foreign",
			},
			AnswerFields: []string{
				"native",
			},
		},
	}

	t.Run(
		"valid config",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			if err := ValidateConfig(
				oldConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"add new field",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields = append(
				newConfig.Schema.Fields,
				folderconfig.FieldDefinition{
					Key:      "example",
					Label:    "Example",
					Required: false,
					Active:   true,
				},
			)

			if err := ValidateConfig(
				oldConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"change label",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields[0].Label = "Word"

			if err := ValidateConfig(
				oldConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"change required",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields[1].Required = false

			if err := ValidateConfig(
				oldConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"deactivate field",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields[0].Active = false

			if err := ValidateConfig(
				oldConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"reactivate field",
		func(t *testing.T) {
			inactiveConfig := cloneConfig(oldConfig)
			inactiveConfig.Schema.Fields[0].Active = false

			newConfig := cloneConfig(inactiveConfig)
			newConfig.Schema.Fields[0].Active = true

			if err := ValidateConfig(
				inactiveConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"cannot physically remove schema field",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields = newConfig.Schema.Fields[1:]

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"cannot physically remove metadata field",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.MetadataSchema.Fields =
				[]folderconfig.FieldDefinition{}

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"duplicate schema key",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields = append(
				newConfig.Schema.Fields,
				folderconfig.FieldDefinition{
					Key:      "foreign",
					Label:    "Duplicate",
					Required: false,
					Active:   true,
				},
			)

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"duplicate metadata key",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.MetadataSchema.Fields = append(
				newConfig.MetadataSchema.Fields,
				folderconfig.FieldDefinition{
					Key:      "level",
					Label:    "Duplicate Level",
					Required: false,
					Active:   true,
				},
			)

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"empty key",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields = append(
				newConfig.Schema.Fields,
				folderconfig.FieldDefinition{
					Key:      "",
					Label:    "Empty",
					Required: false,
					Active:   true,
				},
			)

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"invalid key format",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields = append(
				newConfig.Schema.Fields,
				folderconfig.FieldDefinition{
					Key:      "Bad-Key",
					Label:    "Bad",
					Required: false,
					Active:   true,
				},
			)

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"empty label",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields[0].Label = ""

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"question field does not exist",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Card.QuestionFields =
				[]string{
					"unknown",
				}

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"answer field does not exist",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Card.AnswerFields =
				[]string{
					"unknown",
				}

			err := ValidateConfig(
				oldConfig,
				newConfig,
			)

			if !errors.Is(
				err,
				ErrInvalidConfig,
			) {
				t.Fatalf(
					"expected ErrInvalidConfig, got %v",
					err,
				)
			}
		},
	)

	t.Run(
		"inactive field is allowed in card config",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Schema.Fields[0].Active = false

			newConfig.Card.QuestionFields =
				[]string{
					"foreign",
				}

			if err := ValidateConfig(
				oldConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)

	t.Run(
		"empty card sides are allowed",
		func(t *testing.T) {
			newConfig := cloneConfig(oldConfig)

			newConfig.Card.QuestionFields =
				[]string{}

			newConfig.Card.AnswerFields =
				[]string{}

			if err := ValidateConfig(
				oldConfig,
				newConfig,
			); err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		},
	)
}

func cloneConfig(
	config folderconfig.FolderConfig,
) folderconfig.FolderConfig {
	result := config

	result.Schema.Fields = append(
		[]folderconfig.FieldDefinition(nil),
		config.Schema.Fields...,
	)

	result.MetadataSchema.Fields = append(
		[]folderconfig.FieldDefinition(nil),
		config.MetadataSchema.Fields...,
	)

	result.Card.QuestionFields = append(
		[]string(nil),
		config.Card.QuestionFields...,
	)

	result.Card.AnswerFields = append(
		[]string(nil),
		config.Card.AnswerFields...,
	)

	return result
}
