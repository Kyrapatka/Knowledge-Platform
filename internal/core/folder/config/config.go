package config

type FolderConfig struct {
	Schema         MaterialSchema `json:"schema"`
	MetadataSchema MetadataSchema `json:"metadata_schema"`
	Card           CardConfig     `json:"card"`
}
