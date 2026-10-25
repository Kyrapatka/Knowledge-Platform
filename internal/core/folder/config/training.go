package config

type TrainingConfig struct {
	DefaultAlgorithmKey string `json:"default_algorithm_key"`
	PoolSize            int    `json:"pool_size"`
	RefillThreshold     *int   `json:"refill_threshold,omitempty"`
}

func DefaultRefillThreshold(algorithm string, size int) int {
	switch algorithm {
	case "interview_long_term", "interview_cram":
		return 3
	case "english_basic", "english_adaptive":
		return 5
	default:
		return size - 1
	}
}
func (c TrainingConfig) Threshold() int {
	if c.RefillThreshold != nil {
		return *c.RefillThreshold
	}
	return DefaultRefillThreshold(c.DefaultAlgorithmKey, c.PoolSize)
}

// Defaults are copied on creation; explicit thresholds are never recalculated.
func DefaultTrainingConfig(template string) TrainingConfig {
	key, size := "english_basic", 8
	switch template {
	case "interview_questions":
		key, size = "interview_long_term", 5
	case "formulas":
		key, size = "formula_adaptive", 5
	}
	threshold := DefaultRefillThreshold(key, size)
	return TrainingConfig{DefaultAlgorithmKey: key, PoolSize: size, RefillThreshold: &threshold}
}
