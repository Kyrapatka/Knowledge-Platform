package graph

import (
	"encoding/json"
	"github.com/google/uuid"
	"testing"
)

func TestExactMaterialsConstrainEntireMockTraversal(t *testing.T) {
	for _, mode := range []string{"real", "balanced", "deep"} {
		for _, connected := range []bool{false, true} {
			s, pool := mockBank(8, "go")
			s.Config.InterviewMode = mode
			s.Config.DepthLevel = 3
			s.Config.QuestionLimit = 24
			allowed := []uuid.UUID{pool[1].MaterialID, pool[3].MaterialID, pool[5].MaterialID}
			s.Sources[0].MaterialIDs = allowed
			for i := range pool {
				pool[i].RootWeight = 0
				if connected {
					pool[i].Concepts = []Link{{Slug: "same", Role: "primary", Weight: 1}, {Slug: "same", Role: "answer", Weight: 1}}
				}
			}
			plan, err := BuildPlan(s, pool)
			if err != nil {
				t.Fatal(err)
			}
			total := 0
			for _, topic := range plan.Topics {
				total += topic.Available
			}
			wantAreas := 3
			if connected {
				wantAreas = 1
			}
			if total != wantAreas {
				t.Fatal("preview widened selection", total)
			}
			v := SelectRoot(s, pool)
			seen := map[uuid.UUID]bool{}
			for n := 0; n < 8 && v.Candidate != nil; n++ {
				id := v.Candidate.MaterialID
				found := false
				for _, a := range allowed {
					found = found || a == id
				}
				if !found || seen[id] {
					t.Fatal("escaped or repeated selection", mode)
				}
				seen[id] = true
				raw, _ := json.Marshal(v.State)
				var restored State
				if err := json.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				v = SelectMetadata(restored, *v.Candidate, "correct", pool, Catalog{})
			}
			if len(seen) != 3 || v.State.StopReason != "no_remaining_questions" {
				t.Fatal("wrong completion", v.State)
			}
		}
	}
}
