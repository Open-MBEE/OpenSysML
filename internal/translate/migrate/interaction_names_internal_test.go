package migrate

import "testing"

func TestInheritedOccurrenceNames(t *testing.T) {
	names := inheritedOccurrenceNames()
	for _, name := range []string{"self", "this", "portionOfLife", "suboccurrences", "spaceTimeEnclosedOccurrences"} {
		if !names[name] {
			t.Errorf("missing inherited occurrence member %q", name)
		}
	}
	if names["that"] || names["earlierOccurrence"] || names["isEmpty"] {
		t.Error("nested members and private imports are not inherited by Occurrence")
	}
	if got := freshIn(names, "self"); got != "self2" {
		t.Errorf("fresh self = %q, want self2", got)
	}
	if got := freshIn(names, "self"); got != "self3" {
		t.Errorf("second fresh self = %q, want self3", got)
	}
	if got := freshIn(names, ""); got != "" {
		t.Errorf("anonymous name = %q, want empty", got)
	}
	if !inheritedOccurrenceNames()["self"] || inheritedOccurrenceNames()["self2"] {
		t.Error("callers must receive independent name sets")
	}
	if !occurrenceNames()["self"] || occurrenceNames()["self2"] {
		t.Error("fresh names must not mutate the shared set")
	}
}
