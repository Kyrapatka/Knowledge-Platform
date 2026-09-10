package template

import folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"

func DefaultTemplates() []Template {
	return []Template{
		englishWordsTemplate(),
		interviewQuestionsTemplate(),
		formulasTemplate(),
	}
}

func englishWordsTemplate() Template {
	return Template{
		Key: "english_words",

		Config: folderconfig.FolderConfig{
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
					{
						Key:      "transcription",
						Label:    "Transcription",
						Required: false,
						Active:   true,
					},
					{
						Key:      "definition",
						Label:    "Definition",
						Required: false,
						Active:   true,
					},
					{
						Key:      "example",
						Label:    "Example",
						Required: false,
						Active:   true,
					},
					{
						Key:      "example_translation",
						Label:    "Example Translation",
						Required: false,
						Active:   true,
					},
					{
						Key:      "note",
						Label:    "Note",
						Required: false,
						Active:   true,
					},
					{
						Key:      "sources",
						Label:    "Sources",
						Required: false,
						Active:   true,
					},
				},
			},

			MetadataSchema: folderconfig.MetadataSchema{
				Fields: []folderconfig.FieldDefinition{{Key: "topic", Label: "Topic", Active: true}},
			},

			Card: folderconfig.CardConfig{
				QuestionFields: []string{
					"foreign",
				},

				AnswerFields: []string{
					"native",
					"transcription",
					"definition",
					"example",
					"example_translation",
					"note",
					"sources",
				},
			},
		},
	}
}

func interviewQuestionsTemplate() Template {
	return Template{
		Key: "interview_questions",

		Config: folderconfig.FolderConfig{
			Schema: folderconfig.MaterialSchema{
				Fields: []folderconfig.FieldDefinition{
					{
						Key:      "question",
						Label:    "Question",
						Required: true,
						Active:   true,
					},
					{
						Key:      "answer",
						Label:    "Answer",
						Required: true,
						Active:   true,
					},
					{
						Key:      "short_answer",
						Label:    "Short Answer",
						Required: false,
						Active:   true,
					},
					{
						Key:      "explanation",
						Label:    "Explanation",
						Required: false,
						Active:   true,
					},
					{
						Key:      "code_example",
						Label:    "Code Example",
						Required: false,
						Active:   true,
					},
					{
						Key:      "common_mistakes",
						Label:    "Common Mistakes",
						Required: false,
						Active:   true,
					},
					{
						Key:      "note",
						Label:    "Note",
						Required: false,
						Active:   true,
					},
					{
						Key:      "sources",
						Label:    "Sources",
						Required: false,
						Active:   true,
					},
				},
			},

			MetadataSchema: folderconfig.MetadataSchema{
				Fields: []folderconfig.FieldDefinition{
					{Key: "topic", Label: "Topic", Active: true},
					{
						Key:      "company",
						Label:    "Company",
						Required: false,
						Active:   true,
					},
					{
						Key:      "level",
						Label:    "Level",
						Required: false,
						Active:   true,
					},
					{
						Key:      "category",
						Label:    "Category",
						Required: false,
						Active:   true,
					},
				},
			},

			Card: folderconfig.CardConfig{
				QuestionFields: []string{
					"question",
				},

				AnswerFields: []string{
					"short_answer",
					"answer",
					"explanation",
					"code_example",
					"common_mistakes",
					"note",
					"sources",
				},
			},
		},
	}
}

func formulasTemplate() Template {
	return Template{
		Key: "formulas",

		Config: folderconfig.FolderConfig{
			Schema: folderconfig.MaterialSchema{
				Fields: []folderconfig.FieldDefinition{
					{
						Key:      "name",
						Label:    "Name",
						Required: true,
						Active:   true,
					},
					{
						Key:      "formula",
						Label:    "Formula",
						Required: true,
						Active:   true,
					},
					{
						Key:      "explanation",
						Label:    "Explanation",
						Required: false,
						Active:   true,
					},
					{
						Key:      "variables",
						Label:    "Variables",
						Required: false,
						Active:   true,
					},
					{
						Key:      "units",
						Label:    "Units",
						Required: false,
						Active:   true,
					},
					{
						Key:      "example",
						Label:    "Example",
						Required: false,
						Active:   true,
					},
					{
						Key:      "conditions",
						Label:    "Conditions",
						Required: false,
						Active:   true,
					},
					{
						Key:      "note",
						Label:    "Note",
						Required: false,
						Active:   true,
					},
					{
						Key:      "sources",
						Label:    "Sources",
						Required: false,
						Active:   true,
					},
				},
			},

			MetadataSchema: folderconfig.MetadataSchema{
				Fields: []folderconfig.FieldDefinition{{Key: "topic", Label: "Topic", Active: true}},
			},

			Card: folderconfig.CardConfig{
				QuestionFields: []string{
					"name",
				},

				AnswerFields: []string{
					"formula",
					"explanation",
					"variables",
					"units",
					"example",
					"conditions",
					"note",
					"sources",
				},
			},
		},
	}
}
