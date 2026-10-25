package model

import (
	"fmt"
	folderconfig "github.com/Kyrapatka/knowledge-platform/internal/core/folder/config"
)

// PoolPolicy admits new materials independently from recycling existing items.
type PoolPolicy struct{ PoolSize, RefillThreshold int }

func DefaultPoolPolicy(algorithm string, size int) PoolPolicy {
	return PoolPolicy{size, folderconfig.DefaultRefillThreshold(algorithm, size)}
}
func (p PoolPolicy) Validate() error {
	if p.PoolSize < 1 || p.PoolSize > 100 {
		return fmt.Errorf("pool_size must be 1..100")
	}
	if p.RefillThreshold < 0 || p.RefillThreshold >= p.PoolSize {
		return fmt.Errorf("refill_threshold must be >= 0 and less than pool_size")
	}
	return nil
}
func (p PoolPolicy) RefillSlots(active int) int {
	if active > p.RefillThreshold {
		return 0
	}
	return max(0, p.PoolSize-active)
}
func (p TrainingPlan) PoolPolicy() PoolPolicy {
	policy := DefaultPoolPolicy(p.AlgorithmKey, p.Config.PoolSize)
	if p.Config.RefillThreshold != nil {
		policy.RefillThreshold = *p.Config.RefillThreshold
	}
	return policy
}
func (c *PlanConfig) SetPoolPolicy(algorithm string, size int) {
	c.PoolSize = size
	if c.RefillThreshold == nil {
		threshold := folderconfig.DefaultRefillThreshold(algorithm, size)
		c.RefillThreshold = &threshold
	}
}
