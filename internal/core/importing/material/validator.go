package materialimport

import (
	"fmt"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	foldermodel "github.com/Kyrapatka/knowledge-platform/internal/core/folder/model"
	materialmodel "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	materialservice "github.com/Kyrapatka/knowledge-platform/internal/core/material/service"
	"github.com/google/uuid"
	"strings"
	"unicode/utf8"
)

func normalized(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func value(values map[string]*string, key string) string {
	if v := values[key]; v != nil {
		return *v
	}
	return ""
}
func policy(p string) string {
	if p == "" {
		return "skip"
	}
	return p
}
func validPolicy(p string) bool { return p == "skip" || p == "allow" || p == "conflict" }
func normalizeItem(item Item) Item {
	clone := func(src map[string]*string) map[string]*string {
		dst := map[string]*string{}
		for k, v := range src {
			if v == nil {
				dst[k] = nil
			} else {
				s := strings.TrimSpace(*v)
				dst[k] = &s
			}
		}
		return dst
	}
	item.Values = clone(item.Values)
	item.Metadata = clone(item.Metadata)
	if item.Difficulty == "" {
		item.Difficulty = materialmodel.DifficultyMedium
	}
	return item
}
func validate(folder foldermodel.Folder, req Request, existing []materialmodel.Material) Result {
	out := Result{Valid: true, Issues: []Issue{}, Items: []ItemResult{}}
	add := func(client, path, code, severity, message string) {
		out.Issues = append(out.Issues, Issue{client, path, code, severity, message})
		if severity == "error" {
			out.Valid = false
		}
	}
	if req.CommandID == uuid.Nil {
		add("", "command_id", "command_id", "error", "A nonzero command UUID is required")
	}
	if !validPolicy(policy(req.DuplicatePolicy)) {
		add("", "duplicate_policy", "duplicate_policy", "error", "Choose skip, conflict or allow")
	}
	if folder.TemplateKey != "english_words" && folder.TemplateKey != "interview_questions" {
		add("", "folder", "unsupported_template", "error", "Quick Import supports words and interview questions")
	}
	if len(req.Items) < 1 || len(req.Items) > MaxItems {
		add("", "items", "item_limit", "error", "Expected 1 to 1000 items")
		return out
	}
	key := "question"
	if folder.TemplateKey == "english_words" {
		key = "foreign"
	}
	dbKeys := map[string]string{}
	for _, m := range existing {
		v := normalized(value(m.Values, key))
		if v != "" {
			dbKeys[v] = m.ID.String()
		}
	}
	batch := map[string]string{}
	ids := map[string]bool{}
	total := 0
	for index, raw := range req.Items {
		item := normalizeItem(raw)
		path := fmt.Sprintf("items[%d]", index)
		before := len(out.Issues)
		if item.ClientID == "" || len(item.ClientID) > 128 || ids[item.ClientID] {
			add(item.ClientID, path+".client_id", "client_id", "error", "client_id must be unique, nonempty and at most 128 bytes")
		}
		ids[item.ClientID] = true
		check := func(values map[string]*string, fields []folderconfig.FieldDefinition, part string) {
			allowed := map[string]bool{}
			for _, f := range fields {
				allowed[f.Key] = f.Active
			}
			if len(values) > 64 {
				add(item.ClientID, path+part, "field_limit", "error", "At most 64 fields")
			}
			for k, v := range values {
				if !allowed[k] || k == "transcription" || k == "pronunciation" {
					add(item.ClientID, path+part+"."+k, "unsupported_field", "error", "Unknown, inactive or pronunciation field")
				}
				if v == nil || strings.TrimSpace(value(values, k)) == "" {
					add(item.ClientID, path+part+"."+k, "empty_value", "error", "Omit empty optional fields; present fields must contain text")
					continue
				}
				total += len(*v)
				if len(*v) > 100000 || len(k) > 128 || !utf8.ValidString(*v) || strings.ContainsRune(*v, 0) {
					add(item.ClientID, path+part+"."+k, "field_limit", "error", "Invalid text or field exceeds 100000 UTF-8 bytes")
				}
			}
		}
		check(item.Values, folder.Config.Schema.Fields, ".values")
		check(item.Metadata, folder.Config.MetadataSchema.Fields, ".metadata")
		if err := materialservice.ValidateCreateInput(folder.Config, materialservice.CreateInput{Values: item.Values, Metadata: item.Metadata, Difficulty: item.Difficulty}); err != nil {
			add(item.ClientID, path, "schema", "error", err.Error())
		}
		if item.Profile != nil && folder.TemplateKey != "interview_questions" {
			add(item.ClientID, path+".profile", "profile", "error", "Profile applies only to interview questions")
		}
		if folder.TemplateKey == "interview_questions" {
			p, err := buildProfile(item)
			if err != nil {
				add(item.ClientID, path+".profile", "profile", "error", err.Error())
			} else if p.Status == "draft" {
				add(item.ClientID, path+".profile", "draft_question", "warning", "No usable answer: saved as draft and unavailable in Mock until completed")
			}
		}
		selectedPolicy := policy(req.DuplicatePolicy)
		if item.DuplicatePolicy != "" {
			selectedPolicy = item.DuplicatePolicy
		}
		if !validPolicy(selectedPolicy) {
			add(item.ClientID, path+".duplicate_policy", "duplicate_policy", "error", "Choose skip, conflict or allow")
		}
		row := ItemResult{ClientID: item.ClientID, Status: "create"}
		v := normalized(value(item.Values, key))
		if id := dbKeys[v]; id != "" {
			row.DuplicateOf = id
			row.DuplicateKind = "existing"
		} else if id := batch[v]; id != "" {
			row.DuplicateOf = id
			row.DuplicateKind = "batch"
		}
		if row.DuplicateOf != "" {
			if selectedPolicy == "skip" {
				row.Status = "skipped"
			} else if selectedPolicy == "conflict" {
				add(item.ClientID, path, "duplicate", "error", "Duplicate requires skip or explicit allow")
			}
			if selectedPolicy != "conflict" {
				add(item.ClientID, path, "duplicate", "warning", "Exact normalized duplicate: "+selectedPolicy)
			}
		}
		failed := false
		for _, i := range out.Issues[before:] {
			if i.Severity == "error" {
				failed = true
			}
		}
		if failed {
			row.Status = "invalid"
			out.Failed++
		} else if row.Status == "skipped" {
			out.Skipped++
		} else {
			out.Created++
			if v != "" {
				batch[v] = item.ClientID
			}
		}
		out.Items = append(out.Items, row)
	}
	if total > 8<<20 {
		add("", "items", "total_size", "error", "Total field values exceed 8 MiB")
	}
	if !out.Valid {
		out.Created = 0
	}
	return out
}
