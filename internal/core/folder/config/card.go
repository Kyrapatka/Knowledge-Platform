package config

type CardConfig struct {
	QuestionFields []string `json:"question_fields"`
	AnswerFields   []string `json:"answer_fields"`
}
