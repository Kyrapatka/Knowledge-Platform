package service

import (
	"crypto/rand"

	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
	material "github.com/Kyrapatka/knowledge-platform/internal/core/material/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/model"
	"github.com/Kyrapatka/knowledge-platform/internal/core/training/repository"
)

func englishCard(tx repository.Tx, card *model.Presentation, config folderconfig.FolderConfig, m material.Material) error {
	words := fields([]string{"foreign", "native"}, config, m.Values)
	if len(words) != 2 {
		return nil
	}
	previous, err := tx.LastEnglishDirection(m.ID)
	if err != nil {
		return err
	}
	direction := "foreign"
	if previous == "foreign" {
		direction = "native"
	} else if previous == "" {
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			return err
		}
		if b[0]&1 == 1 {
			direction = "native"
		}
	}
	question, answer := words[0], words[1]
	if direction == "native" {
		question, answer = answer, question
	}
	card.Direction = direction
	card.Question = []model.CardField{question}
	extras := card.Answer
	card.Answer = []model.CardField{answer}
	for _, field := range extras {
		if field.Key != "foreign" && field.Key != "native" && field.Key != "example" {
			card.Answer = append(card.Answer, field)
		}
	}
	example := fields([]string{"example"}, config, m.Values)
	if len(example) > 0 {
		card.Example = example[0].Value
		card.ForeignWord = words[0].Value
	}
	return nil
}
