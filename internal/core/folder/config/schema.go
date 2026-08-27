package config

type MaterialSchema struct {
	Fields []FieldDefinition `json:"fields"`
}

type MetadataSchema struct {
	Fields []FieldDefinition `json:"fields"`
}

type FieldDefinition struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
	Active   bool   `json:"active"`
}
