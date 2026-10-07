package model

import (
	"encoding/json"
	"testing"
)

func TestPoolAdmissionPolicy(t *testing.T) {
	for _, tc := range []struct {
		key             string
		size, threshold int
		slots           []int
	}{
		{"interview_long_term", 5, 3, []int{0, 0, 2, 3, 4, 5}},
		{"interview_cram", 5, 3, []int{0, 0, 2, 3, 4, 5}},
		{"english_basic", 8, 5, []int{0, 0, 0, 3, 4, 5, 6, 7, 8}},
		{"english_adaptive", 8, 5, []int{0, 0, 0, 3, 4, 5, 6, 7, 8}},
		{"formula_adaptive", 5, 4, []int{0, 1, 2, 3, 4, 5}},
		{"english_basic", 2, 1, []int{0, 1, 2}},
		{"interview_cram", 1, 0, []int{0, 1}},
	} {
		t.Run(tc.key+string(rune('0'+tc.size)), func(t *testing.T) {
			p := DefaultPoolPolicy(tc.key, tc.size)
			if p.RefillThreshold != tc.threshold {
				t.Fatal(p)
			}
			for removed, want := range tc.slots {
				if got := p.RefillSlots(tc.size - removed); got != want {
					t.Fatalf("active %d: got %d want %d", tc.size-removed, got, want)
				}
			}
			if p.RefillSlots(tc.size+1) != 0 {
				t.Fatal("overfull pool admitted candidates")
			}
		})
	}
}
func TestPoolPolicyLegacyAndSnapshot(t *testing.T) {
	var c PlanConfig
	if err := json.Unmarshal([]byte(`{"pool_size":5}`), &c); err != nil {
		t.Fatal(err)
	}
	p := TrainingPlan{AlgorithmKey: "interview_long_term", Config: c}
	if p.PoolPolicy().RefillThreshold != 3 {
		t.Fatal("legacy policy")
	}
	c.SetPoolPolicy("english_basic", 8)
	raw, _ := json.Marshal(c)
	var loaded PlanConfig
	if err := json.Unmarshal(raw, &loaded); err != nil || loaded.RefillThreshold == nil || *loaded.RefillThreshold != 5 {
		t.Fatal("snapshot lost threshold", err)
	}
	loaded.SetPoolPolicy("english_basic", 3)
	if *loaded.RefillThreshold != 2 {
		t.Fatal("custom target invalid")
	}
}
