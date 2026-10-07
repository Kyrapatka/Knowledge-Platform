package interview

import "testing"

func TestNormalizedCopyQuestion(t *testing.T) {
	for _, v := range []string{" Question TEXT? ", "question\ttext?", "QUESTION\n\nTEXT?", "\u00a0Question\u2003text?\u00a0"} {
		if got := normalizedQuestion(v); got != "question text?" {
			t.Fatalf("normalized %q as %q", v, got)
		}
	}
	if normalizedQuestion("Почему  ТРАНЗАКЦИЯ?") != normalizedQuestion("почему транзакция?") {
		t.Fatal("non-ASCII case comparison")
	}
}
