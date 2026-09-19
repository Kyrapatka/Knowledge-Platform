package importer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/Kyrapatka/knowledge-platform/internal/core/interview"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
)

const (
	FormatVersion        = 1
	MaxFileSize          = 10 * 1024 * 1024
	MaxItems             = 5000
	MaxFolderNameLength  = 200
	MaxDescriptionLength = 2000
	maxQuestionLength    = 20000
	maxAnswerLength      = 100000
	maxWordLength        = 500
	maxTextLength        = 20000
	maxTaxonomyLength    = 200
	maxListItems         = 64
)

const (
	TemplateInterviewQuestions = "interview_questions"
	TemplateEnglishWords       = "english_words"
)

type FieldError struct {
	Item    int    `json:"item,omitempty"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Preview struct {
	Valid         bool         `json:"valid"`
	FormatVersion int          `json:"format_version"`
	Template      string       `json:"template"`
	FolderName    string       `json:"folder_name"`
	ItemsCount    int          `json:"items_count"`
	Warnings      []string     `json:"warnings"`
	Errors        []FieldError `json:"errors,omitempty"`
}

type plannedItem struct {
	Values     map[string]*string
	Metadata   map[string]*string
	Difficulty materialmodel.Difficulty
	Profile    *interview.ProfileRequest
}

type Plan struct {
	FormatVersion int
	FolderName    string
	Description   string
	Template      string
	Items         []plannedItem
}

type envelope struct {
	FormatVersion int               `json:"format_version"`
	Folder        envelopeFolder    `json:"folder"`
	Items         []json.RawMessage `json:"items"`
}

type envelopeFolder struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Template    string `json:"template"`
}

type interviewItem struct {
	Question    string   `json:"question"`
	ShortAnswer string   `json:"short_answer"`
	FullAnswer  string   `json:"full_answer,omitempty"`
	Source      string   `json:"source,omitempty"`
	Topic       string   `json:"topic,omitempty"`
	Subtopic    string   `json:"subtopic,omitempty"`
	Difficulty  *int     `json:"difficulty,omitempty"`
	Frequency   *int     `json:"frequency,omitempty"`
	Level       string   `json:"level,omitempty"`
	Company     string   `json:"company,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Concepts    []string `json:"concepts,omitempty"`
}

type englishItem struct {
	Word               string   `json:"word"`
	Translation        string   `json:"translation"`
	Pronunciation      string   `json:"pronunciation,omitempty"`
	Definition         string   `json:"definition,omitempty"`
	Example            string   `json:"example,omitempty"`
	ExampleTranslation string   `json:"example_translation,omitempty"`
	Notes              string   `json:"notes,omitempty"`
	Topic              string   `json:"topic,omitempty"`
	Sources            []string `json:"sources,omitempty"`
}

func Parse(data []byte) (Plan, Preview) {
	preview := Preview{Warnings: []string{}, Errors: []FieldError{}}
	if len(data) == 0 {
		return Plan{}, invalid(preview, "$", "The file is empty.")
	}
	if len(data) > MaxFileSize {
		return Plan{}, invalid(preview, "$", fmt.Sprintf("The file is larger than %d MB.", MaxFileSize/(1024*1024)))
	}
	if !utf8.Valid(data) {
		return Plan{}, invalid(preview, "$", "The file must use valid UTF-8 encoding.")
	}

	var raw envelope
	if err := decodeStrict(data, &raw); err != nil {
		return Plan{}, invalid(preview, "$", "The file is not valid JSON: "+jsonErrorMessage(err))
	}
	preview.FormatVersion = raw.FormatVersion
	preview.Template = raw.Folder.Template
	preview.FolderName = raw.Folder.Name
	preview.ItemsCount = len(raw.Items)
	if raw.FormatVersion != FormatVersion {
		return Plan{}, invalid(preview, "format_version", "Unsupported import format version. Expected format_version 1.")
	}
	name := strings.TrimSpace(raw.Folder.Name)
	if name == "" {
		preview.Errors = append(preview.Errors, FieldError{Field: "folder.name", Message: "Folder name is required."})
	} else if utf8.RuneCountInString(name) > MaxFolderNameLength {
		preview.Errors = append(preview.Errors, FieldError{Field: "folder.name", Message: fmt.Sprintf("Folder name must be at most %d characters.", MaxFolderNameLength)})
	}
	description := strings.TrimSpace(raw.Folder.Description)
	if utf8.RuneCountInString(description) > MaxDescriptionLength {
		preview.Errors = append(preview.Errors, FieldError{Field: "folder.description", Message: fmt.Sprintf("Folder description must be at most %d characters.", MaxDescriptionLength)})
	}
	if raw.Folder.Template != TemplateInterviewQuestions && raw.Folder.Template != TemplateEnglishWords {
		preview.Errors = append(preview.Errors, FieldError{Field: "folder.template", Message: fmt.Sprintf("This file uses an unsupported template: %q.", raw.Folder.Template)})
	}
	if len(raw.Items) == 0 {
		preview.Errors = append(preview.Errors, FieldError{Field: "items", Message: "Add at least one item to import."})
	} else if len(raw.Items) > MaxItems {
		preview.Errors = append(preview.Errors, FieldError{Field: "items", Message: fmt.Sprintf("A file can contain at most %d items.", MaxItems)})
	}
	if len(preview.Errors) > 0 {
		return Plan{}, preview
	}

	plan := Plan{FormatVersion: raw.FormatVersion, FolderName: name, Description: description, Template: raw.Folder.Template, Items: make([]plannedItem, 0, len(raw.Items))}
	seen := make(map[string]int, len(raw.Items))
	for index, itemJSON := range raw.Items {
		var item plannedItem
		var key string
		var errors []FieldError
		if raw.Folder.Template == TemplateInterviewQuestions {
			item, key, errors = parseInterviewItem(itemJSON, index+1)
		} else {
			item, key, errors = parseEnglishItem(itemJSON, index+1)
		}
		preview.Errors = append(preview.Errors, errors...)
		if key != "" {
			if first, exists := seen[key]; exists {
				preview.Errors = append(preview.Errors, FieldError{Item: index + 1, Field: duplicateField(raw.Folder.Template), Message: fmt.Sprintf("This item duplicates item %d.", first)})
			} else {
				seen[key] = index + 1
			}
		}
		plan.Items = append(plan.Items, item)
	}
	if len(preview.Errors) > 0 {
		return Plan{}, preview
	}
	preview.Valid = true
	preview.FolderName = name
	return plan, preview
}

func invalid(preview Preview, field, message string) Preview {
	preview.Errors = append(preview.Errors, FieldError{Field: field, Message: message})
	return preview
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func jsonErrorMessage(err error) string {
	message := err.Error()
	if strings.HasPrefix(message, "json: unknown field ") {
		return "unknown field " + strings.TrimPrefix(message, "json: unknown field ")
	}
	return message
}

func parseInterviewItem(data []byte, number int) (plannedItem, string, []FieldError) {
	var source interviewItem
	if err := decodeStrict(data, &source); err != nil {
		return plannedItem{}, "", []FieldError{{Item: number, Field: "$", Message: "Question item is invalid: " + jsonErrorMessage(err)}}
	}
	errors := []FieldError{}
	question := strings.TrimSpace(source.Question)
	shortAnswer := strings.TrimSpace(source.ShortAnswer)
	requireText(&errors, number, "question", question, maxQuestionLength)
	requireText(&errors, number, "short_answer", shortAnswer, maxAnswerLength)
	if shortAnswer == "." {
		errors = append(errors, FieldError{Item: number, Field: "short_answer", Message: "short_answer must contain a real answer, not a placeholder."})
	}
	optionalText(&errors, number, "full_answer", source.FullAnswer, maxAnswerLength)
	optionalText(&errors, number, "source", source.Source, maxTextLength)
	for field, value := range map[string]string{"topic": source.Topic, "subtopic": source.Subtopic, "level": source.Level, "company": source.Company} {
		optionalText(&errors, number, field, value, maxTaxonomyLength)
	}
	difficulty := 2
	if source.Difficulty != nil {
		difficulty = *source.Difficulty
		if difficulty < 1 || difficulty > 5 {
			errors = append(errors, FieldError{Item: number, Field: "difficulty", Message: "difficulty must be between 1 and 5."})
		}
	}
	frequency := 5
	if source.Frequency != nil {
		frequency = *source.Frequency
		if frequency < 1 || frequency > 10 {
			errors = append(errors, FieldError{Item: number, Field: "frequency", Message: "frequency must be between 1 and 10."})
		}
	}
	validateList(&errors, number, "keywords", source.Keywords)
	validateList(&errors, number, "concepts", source.Concepts)
	if len(errors) > 0 {
		return plannedItem{}, normalizedKey(question), errors
	}

	values := map[string]*string{"question": ptr(question), "short_answer": ptr(shortAnswer)}
	put(values, "answer", source.FullAnswer, true)
	put(values, "sources", source.Source, true)
	metadata := map[string]*string{}
	put(metadata, "topic", source.Topic, false)
	put(metadata, "category", source.Subtopic, false)
	put(metadata, "level", source.Level, false)
	put(metadata, "company", source.Company, false)
	putJSONList(metadata, "keywords", source.Keywords)
	putJSONList(metadata, "concepts", source.Concepts)

	concepts := profileConcepts(source.Concepts, source.Keywords, source.Topic, source.Subtopic)
	levelMin, levelMax := interviewLevel(source.Level)
	profile := interview.ProfileRequest{Profile: interview.Profile{
		Frequency: frequency, FrequencyConfidence: .5, InterviewDifficulty: difficulty,
		Specificity: 2, RootWeight: 5, FollowupWeight: 5, LevelMin: levelMin, LevelMax: levelMax,
		Topic: strings.TrimSpace(source.Topic), Subtopic: strings.TrimSpace(source.Subtopic),
		Domain: domainSlug(source.Topic), Status: "ready", Concepts: concepts, InterviewProfiles: []string{},
	}, ExpectedVersion: 0}

	return plannedItem{Values: values, Metadata: metadata, Difficulty: materialDifficulty(difficulty), Profile: &profile}, normalizedKey(question), nil
}

func parseEnglishItem(data []byte, number int) (plannedItem, string, []FieldError) {
	var source englishItem
	if err := decodeStrict(data, &source); err != nil {
		return plannedItem{}, "", []FieldError{{Item: number, Field: "$", Message: "English word item is invalid: " + jsonErrorMessage(err)}}
	}
	errors := []FieldError{}
	word := strings.TrimSpace(source.Word)
	translation := strings.TrimSpace(source.Translation)
	requireText(&errors, number, "word", word, maxWordLength)
	requireText(&errors, number, "translation", translation, maxTextLength)
	for field, value := range map[string]string{
		"pronunciation": source.Pronunciation, "definition": source.Definition,
		"example": source.Example, "example_translation": source.ExampleTranslation,
		"notes": source.Notes, "topic": source.Topic,
	} {
		optionalText(&errors, number, field, value, maxTextLength)
	}
	validateList(&errors, number, "sources", source.Sources)
	if len(errors) > 0 {
		return plannedItem{}, normalizedKey(word), errors
	}
	values := map[string]*string{"foreign": ptr(word), "native": ptr(translation)}
	put(values, "transcription", source.Pronunciation, false)
	put(values, "definition", source.Definition, false)
	put(values, "example", source.Example, false)
	put(values, "example_translation", source.ExampleTranslation, false)
	put(values, "note", source.Notes, true)
	if len(source.Sources) > 0 {
		joined := strings.Join(source.Sources, "\n")
		values["sources"] = &joined
	}
	metadata := map[string]*string{}
	put(metadata, "topic", source.Topic, false)
	return plannedItem{Values: values, Metadata: metadata, Difficulty: materialmodel.DifficultyMedium}, normalizedKey(word), nil
}

func requireText(errors *[]FieldError, item int, field, value string, limit int) {
	if value == "" {
		*errors = append(*errors, FieldError{Item: item, Field: field, Message: field + " is required."})
		return
	}
	optionalText(errors, item, field, value, limit)
}

func optionalText(errors *[]FieldError, item int, field, value string, limit int) {
	if utf8.RuneCountInString(value) > limit {
		*errors = append(*errors, FieldError{Item: item, Field: field, Message: fmt.Sprintf("%s must be at most %d characters.", field, limit)})
	}
}

func validateList(errors *[]FieldError, item int, field string, values []string) {
	if len(values) > maxListItems {
		*errors = append(*errors, FieldError{Item: item, Field: field, Message: fmt.Sprintf("%s can contain at most %d values.", field, maxListItems)})
		return
	}
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || utf8.RuneCountInString(trimmed) > maxTaxonomyLength {
			*errors = append(*errors, FieldError{Item: item, Field: field, Message: fmt.Sprintf("Every %s value must contain 1 to %d characters.", field, maxTaxonomyLength)})
			return
		}
		key := normalizedKey(trimmed)
		if seen[key] {
			*errors = append(*errors, FieldError{Item: item, Field: field, Message: field + " must not contain duplicates."})
			return
		}
		seen[key] = true
	}
}

func put(target map[string]*string, key, value string, keepEmpty bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" || keepEmpty && value != "" {
		target[key] = ptr(trimmed)
	}
}

func putJSONList(target map[string]*string, key string, values []string) {
	if len(values) == 0 {
		return
	}
	encoded, _ := json.Marshal(values)
	text := string(encoded)
	target[key] = &text
}

func ptr(value string) *string { return &value }

func normalizedKey(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func duplicateField(template string) string {
	if template == TemplateEnglishWords {
		return "word"
	}
	return "question"
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func conceptSlug(value string) string {
	normalized := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "_"), "_")
	if normalized == "" {
		hash := sha256.Sum256([]byte(value))
		normalized = "concept_" + hex.EncodeToString(hash[:4])
	}
	if len(normalized) > 96 {
		normalized = strings.TrimRight(normalized[:96], "_")
	}
	return normalized
}

func profileConcepts(primary, keywords []string, topic, subtopic string) []interview.QuestionConcept {
	candidates := append([]string{}, primary...)
	if len(candidates) == 0 {
		candidates = append(candidates, keywords...)
	}
	if len(candidates) == 0 && strings.TrimSpace(subtopic) != "" {
		candidates = append(candidates, subtopic)
	}
	if len(candidates) == 0 && strings.TrimSpace(topic) != "" {
		candidates = append(candidates, topic)
	}
	if len(candidates) == 0 {
		candidates = []string{"imported_question"}
	}
	unique := []string{}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		slug := conceptSlug(candidate)
		if !seen[slug] {
			seen[slug] = true
			unique = append(unique, slug)
		}
		if len(unique) == 31 {
			break
		}
	}
	result := []interview.QuestionConcept{{Slug: unique[0], Role: "primary", Weight: 1, Ordinal: 0}}
	for index, slug := range unique {
		result = append(result, interview.QuestionConcept{Slug: slug, Role: "tested", Weight: 1, Ordinal: index})
	}
	return result
}

func domainSlug(topic string) string {
	if strings.TrimSpace(topic) == "" {
		return ""
	}
	return conceptSlug(topic)
}

func interviewLevel(value string) (int, int) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "junior":
		return 1, 1
	case "junior+", "junior plus":
		return 2, 2
	case "middle":
		return 3, 3
	case "strong middle", "strong_middle":
		return 4, 4
	case "senior":
		return 5, 5
	default:
		return 1, 5
	}
}

func materialDifficulty(value int) materialmodel.Difficulty {
	if value <= 2 {
		return materialmodel.DifficultyEasy
	}
	if value >= 4 {
		return materialmodel.DifficultyHard
	}
	return materialmodel.DifficultyMedium
}
