package service

import (
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
)

// Question contains only what the selected mode allows before attempting the
// exercise. Answer is the self-check reveal payload, like other training cards.
func formulaCard(mode model.PracticeMode, e model.FormulaExercise, c folderconfig.FolderConfig, values map[string]*string, question, answer []model.CardField) ([]model.CardField, []model.CardField) {
	problem := model.CardField{Key: "exercise_problem", Label: "Problem", Value: e.Problem}
	solution := model.CardField{Key: "exercise_solution", Label: "Solution", Value: e.Solution}
	result := model.CardField{Key: "exercise_answer", Label: "Answer", Value: e.Answer}
	support := append([]model.CardField(nil), question...)
	support = append(support, answer...)
	support = append(support, fields([]string{"explanation", "variables", "conditions", "units"}, c, values)...)
	// A configurable Card may already include those explanatory fields.
	unique := make([]model.CardField, 0, len(support))
	seen := map[string]bool{}
	for _, field := range support {
		if !seen[field.Key] {
			unique = append(unique, field)
			seen[field.Key] = true
		}
	}
	reveal := append(append([]model.CardField(nil), unique...), solution, result)
	shown := []model.CardField{problem}
	switch mode {
	case model.PracticeWorked:
		shown = append(shown, reveal...)
	case model.PracticeFaded:
		if e.Hint != "" {
			shown = append(shown, model.CardField{Key: "exercise_hint", Label: "Hint", Value: e.Hint})
		}
	}
	return shown, reveal
}
