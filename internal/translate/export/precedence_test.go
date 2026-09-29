package export

import "testing"

// A kept notation binds as its expression does, whatever comment follows it.
func TestNotationBindingIgnoresTrailingComment(t *testing.T) {
	for _, text := range []string{"a + b", "a + b /* sum */", "a + b // sum"} {
		if got := notationBinding(text); got != bindAdditive {
			t.Errorf("notationBinding(%q) = %d, want %d (additive)", text, got, bindAdditive)
		}
	}
}
