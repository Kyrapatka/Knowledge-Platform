package model

// PoolPolicy separates admitting new materials from recycling current items.
// Explicit custom pool sizes remain supported; their threshold is capped below
// the target. Formula training keeps its existing refill-on-every-free-slot rule.
type PoolPolicy struct{ PoolSize, RefillThreshold int }

func DefaultPoolPolicy(algorithm string, size int) PoolPolicy {
	threshold := size - 1
	switch algorithm {
	case "interview_long_term", "interview_cram":
		threshold = min(3, threshold)
	case "english_basic", "english_adaptive":
		threshold = min(5, threshold)
	}
	return PoolPolicy{size, max(0, threshold)}
}
func (p PoolPolicy) RefillSlots(active int) int {
	if active > p.RefillThreshold {
		return 0
	}
	return max(0, p.PoolSize-active)
}
func (p TrainingPlan) PoolPolicy() PoolPolicy {
	policy := DefaultPoolPolicy(p.AlgorithmKey, p.Config.PoolSize)
	if value := p.Config.RefillThreshold; value != nil && *value >= 0 && *value < p.Config.PoolSize {
		policy.RefillThreshold = *value
	}
	return policy
}
func (c *PlanConfig) SetPoolPolicy(algorithm string, size int) {
	policy := DefaultPoolPolicy(algorithm, size)
	c.PoolSize = size
	c.RefillThreshold = &policy.RefillThreshold
}
