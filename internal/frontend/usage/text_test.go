package usage

import (
	"strings"
	"testing"
)

func TestWrapKeepsWordsWholeWithinTheWidth(t *testing.T) {
	text := "one two three four five six seven"
	got := Wrap(text, 10)
	for _, line := range strings.Split(got, "\n") {
		if len(line) > 10 {
			t.Errorf("line %q is wider than 10", line)
		}
	}
	if strings.Join(strings.Fields(got), " ") != text {
		t.Errorf("wrapping changed the words: %q", got)
	}
}
