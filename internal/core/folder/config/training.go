package config

type TrainingConfig struct {
	DefaultAlgorithmKey string `json:"default_algorithm_key"`
	PoolSize            int    `json:"pool_size"`
}

// These defaults are copied on folder creation, not consulted by running plans.
func DefaultTrainingConfig(template string) TrainingConfig {
	switch template {
	case "interview_questions":
		return TrainingConfig{"interview_long_term", 5}
	case "formulas":
		return TrainingConfig{"formula_adaptive", 5}
	default:
		return TrainingConfig{"english_basic", 8}
	}
}
