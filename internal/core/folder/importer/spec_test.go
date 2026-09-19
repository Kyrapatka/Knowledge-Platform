package importer

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
)

func TestRepositoryExamplesValidateAndMapToExistingSchemas(t *testing.T) {
	tests := []struct {
		file, template, requiredValue, expected string
	}{
		{"../../../../docs/import/examples/interview_questions.json", TemplateInterviewQuestions, "answer", "Goroutine выполняется"},
		{"../../../../docs/import/examples/english_words.json", TemplateEnglishWords, "foreign", "maintain"},
	}
	for _, test := range tests {
		t.Run(test.template, func(t *testing.T) {
			data, err := os.ReadFile(test.file)
			if err != nil {
				t.Fatal(err)
			}
			plan, preview := Parse(data)
			if !preview.Valid || len(preview.Errors) != 0 {
				t.Fatalf("example is invalid: %+v", preview.Errors)
			}
			if plan.Template != test.template || len(plan.Items) != 1 {
				t.Fatalf("unexpected plan: template=%s items=%d", plan.Template, len(plan.Items))
			}
			value := plan.Items[0].Values[test.requiredValue]
			if value == nil || !strings.Contains(*value, test.expected) {
				t.Fatalf("existing material field %q was not mapped: %#v", test.requiredValue, value)
			}
			if test.template == TemplateInterviewQuestions && plan.Items[0].Profile == nil {
				t.Fatal("interview material has no normal interview profile")
			}
		})
	}
}

func TestMinimalSupportedItems(t *testing.T) {
	tests := []struct {
		name, template, item string
	}{
		{"interview", TemplateInterviewQuestions, `{"question":"Что такое goroutine?","short_answer":"Легковесная единица выполнения."}`},
		{"english", TemplateEnglishWords, `{"word":"maintain","translation":"поддерживать"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, preview := Parse(envelopeJSON(test.template, "Folder", test.item))
			if !preview.Valid {
				t.Fatalf("minimal item is invalid: %+v", preview.Errors)
			}
		})
	}
}

func TestImportValidationErrors(t *testing.T) {
	tests := []struct {
		name  string
		data  []byte
		field string
		item  int
	}{
		{"malformed JSON", []byte(`{"format_version":`), "$", 0},
		{"missing folder name", envelopeJSON(TemplateEnglishWords, "", `{"word":"a","translation":"а"}`), "folder.name", 0},
		{"unknown template", envelopeJSON("anki", "Folder", `{}`), "folder.template", 0},
		{"interview missing question", envelopeJSON(TemplateInterviewQuestions, "Folder", `{"short_answer":"Answer"}`), "question", 1},
		{"interview missing short answer", envelopeJSON(TemplateInterviewQuestions, "Folder", `{"question":"Question?"}`), "short_answer", 1},
		{"english missing word", envelopeJSON(TemplateEnglishWords, "Folder", `{"translation":"перевод"}`), "word", 1},
		{"english missing translation", envelopeJSON(TemplateEnglishWords, "Folder", `{"word":"word"}`), "translation", 1},
		{"unsupported version", []byte(`{"format_version":2,"folder":{"name":"Folder","template":"english_words"},"items":[{"word":"a","translation":"а"}]}`), "format_version", 0},
		{"unknown item field", envelopeJSON(TemplateEnglishWords, "Folder", `{"word":"word","translation":"слово","front":"unexpected"}`), "$", 1},
		{"invalid UTF-8", []byte{0xff, 0xfe}, "$", 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, preview := Parse(test.data)
			if preview.Valid || len(preview.Errors) == 0 || len(plan.Items) != 0 {
				t.Fatalf("expected a rejected all-or-nothing plan: plan=%+v preview=%+v", plan, preview)
			}
			found := false
			for _, issue := range preview.Errors {
				if issue.Field == test.field && issue.Item == test.item {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected field %q item %d, got %+v", test.field, test.item, preview.Errors)
			}
		})
	}
}

func TestOneInvalidItemRejectsWholePlan(t *testing.T) {
	data := []byte(`{
		"format_version":1,
		"folder":{"name":"Go","description":"","template":"interview_questions"},
		"items":[
			{"question":"Valid?","short_answer":"Yes."},
			{"question":"Invalid?"},
			{"question":"Also valid?","short_answer":"Yes."}
		]
	}`)
	plan, preview := Parse(data)
	if preview.Valid || len(plan.Items) != 0 {
		t.Fatalf("partial import plan escaped validation: %+v", plan)
	}
	if len(preview.Errors) != 1 || preview.Errors[0].Item != 2 || preview.Errors[0].Field != "short_answer" {
		t.Fatalf("unexpected errors: %+v", preview.Errors)
	}
}

func TestDuplicateItemsAreRejected(t *testing.T) {
	data := []byte(`{
		"format_version":1,
		"folder":{"name":"English","template":"english_words"},
		"items":[
			{"word":"Maintain","translation":"поддерживать"},
			{"word":"  maintain  ","translation":"сохранять"}
		]
	}`)
	_, preview := Parse(data)
	if preview.Valid || len(preview.Errors) != 1 || preview.Errors[0].Item != 2 || preview.Errors[0].Field != "word" {
		t.Fatalf("duplicate was not reported clearly: %+v", preview.Errors)
	}
}

func TestItemAndFileLimits(t *testing.T) {
	items := make([]map[string]string, MaxItems+1)
	for index := range items {
		items[index] = map[string]string{"word": "word" + strings.Repeat("x", index%10), "translation": "value"}
	}
	data, err := json.Marshal(map[string]any{
		"format_version": 1,
		"folder":         map[string]string{"name": "Large", "template": TemplateEnglishWords},
		"items":          items,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, preview := Parse(data)
	if preview.Valid || preview.Errors[0].Field != "items" {
		t.Fatalf("item limit was not enforced: %+v", preview.Errors)
	}

	large := make([]byte, MaxFileSize+1)
	copy(large, `{}`)
	_, preview = Parse(large)
	if preview.Valid || !strings.Contains(preview.Errors[0].Message, "larger") {
		t.Fatalf("file limit was not enforced: %+v", preview.Errors)
	}
}

func TestDuplicateSafeFolderNamesAreOwnerScoped(t *testing.T) {
	folders := []foldermodel.Folder{{Title: "English B2"}, {Title: "english b2 (2)"}, {Title: "Another folder"}}
	if got := availableFolderName("English B2", folders); got != "English B2 (3)" {
		t.Fatalf("name = %q", got)
	}
	if got := availableFolderName("Go Interview", folders); got != "Go Interview" {
		t.Fatalf("unrelated name changed to %q", got)
	}
}

func envelopeJSON(template, name, item string) []byte {
	return []byte(`{"format_version":1,"folder":{"name":` + quote(name) + `,"description":"","template":` + quote(template) + `},"items":[` + item + `]}`)
}

func quote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
