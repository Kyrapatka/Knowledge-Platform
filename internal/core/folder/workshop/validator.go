package workshop

import (
	"fmt"
	"regexp"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
)

var fieldKeyPattern = regexp.MustCompile(
	`^[a-z][a-z0-9_]*$`,
)

func ValidateConfig(
	oldConfig folderconfig.FolderConfig,
	newConfig folderconfig.FolderConfig,
) error {
	if err := validateFieldDefinitions(
		oldConfig.Schema.Fields,
		newConfig.Schema.Fields,
		"schema",
	); err != nil {
		return err
	}

	if err := validateFieldDefinitions(
		oldConfig.MetadataSchema.Fields,
		newConfig.MetadataSchema.Fields,
		"metadata",
	); err != nil {
		return err
	}

	if err := validateCard(
		newConfig,
	); err != nil {
		return err
	}

	return nil
}

func validateFieldDefinitions(
	oldFields []folderconfig.FieldDefinition,
	newFields []folderconfig.FieldDefinition,
	fieldGroup string,
) error {
	oldByKey := make(
		map[string]folderconfig.FieldDefinition,
		len(oldFields),
	)

	for _, field := range oldFields {
		oldByKey[field.Key] = field
	}

	seen := make(
		map[string]struct{},
		len(newFields),
	)

	for _, field := range newFields {
		if field.Key == "" {
			return fmt.Errorf(
				"%w: %s field key is empty",
				ErrInvalidConfig,
				fieldGroup,
			)
		}

		if !fieldKeyPattern.MatchString(
			field.Key,
		) {
			return fmt.Errorf(
				"%w: invalid %s field key %q",
				ErrInvalidConfig,
				fieldGroup,
				field.Key,
			)
		}

		if field.Label == "" {
			return fmt.Errorf(
				"%w: %s field %q has empty label",
				ErrInvalidConfig,
				fieldGroup,
				field.Key,
			)
		}

		if _, exists := seen[field.Key]; exists {
			return fmt.Errorf(
				"%w: duplicate %s field key %q",
				ErrInvalidConfig,
				fieldGroup,
				field.Key,
			)
		}

		seen[field.Key] = struct{}{}
	}

	// Старые keys не должны физически исчезать.
	//
	// Если пользователь "удаляет" поле,
	// оно должно остаться в Config с Active=false.
	for oldKey := range oldByKey {
		if _, exists := seen[oldKey]; !exists {
			return fmt.Errorf(
				"%w: existing %s field %q cannot be removed; set active=false instead",
				ErrInvalidConfig,
				fieldGroup,
				oldKey,
			)
		}
	}

	return nil
}

func validateCard(
	config folderconfig.FolderConfig,
) error {
	allowedFields := make(
		map[string]struct{},
		len(config.Schema.Fields),
	)

	for _, field := range config.Schema.Fields {
		allowedFields[field.Key] = struct{}{}
	}

	for _, key := range config.Card.QuestionFields {
		if _, exists := allowedFields[key]; !exists {
			return fmt.Errorf(
				"%w: question field %q does not exist in schema",
				ErrInvalidConfig,
				key,
			)
		}
	}

	for _, key := range config.Card.AnswerFields {
		if _, exists := allowedFields[key]; !exists {
			return fmt.Errorf(
				"%w: answer field %q does not exist in schema",
				ErrInvalidConfig,
				key,
			)
		}
	}

	return nil
}
