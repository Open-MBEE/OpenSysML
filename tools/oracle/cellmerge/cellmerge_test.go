package cellmerge

import (
	"errors"
	"strings"
	"testing"
)

func merge(t *testing.T, base, ours, theirs string) *Result {
	t.Helper()
	result, err := Merge([]byte(base), []byte(ours), []byte(theirs))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func requireConflict(t *testing.T, base, ours, theirs, reason string) {
	t.Helper()
	_, err := Merge([]byte(base), []byte(ours), []byte(theirs))
	var conflict *Conflict
	if !errors.As(err, &conflict) || !strings.Contains(conflict.Reason, reason) {
		t.Fatalf("want a conflict naming %q, got %v", reason, err)
	}
}

const records = "# rule\troot\tours\tpilot\nA::x\ttraining\t1\t9\nB::y\ttraining\t2\t9\nC::z\ttraining\t3\t9\n"

// Counts moved on neighbouring lines, which git's line merge conflicts on, merge.
func TestNeighbouringLinesMovedByEachSideMerge(t *testing.T) {
	ours := strings.Replace(records, "A::x\ttraining\t1", "A::x\ttraining\t5", 1)
	theirs := strings.Replace(records, "B::y\ttraining\t2", "B::y\ttraining\t7", 1)
	result := merge(t, records, ours, theirs)
	want := "# rule\troot\tours\tpilot\nA::x\ttraining\t5\t9\nB::y\ttraining\t7\t9\nC::z\ttraining\t3\t9\n"
	if string(result.Content) != want || len(result.Summed) != 0 {
		t.Fatalf("got %q (summed %v), want %q", result.Content, result.Summed, want)
	}
}

// One count moved by both sides takes the sum of the movements and is reported for the check.
func TestACountBothSidesMovedIsSummedAndReported(t *testing.T) {
	ours := strings.Replace(records, "B::y\ttraining\t2", "B::y\ttraining\t4", 1)
	theirs := strings.Replace(records, "B::y\ttraining\t2", "B::y\ttraining\t7", 1)
	result := merge(t, records, ours, theirs)
	if !strings.Contains(string(result.Content), "B::y\ttraining\t9\t9\n") {
		t.Fatalf("want 4 + 7 - 2 = 9, got %q", result.Content)
	}
	if len(result.Summed) != 1 || !strings.HasPrefix(result.Summed[0], "B::y\ttraining\t9") {
		t.Fatalf("summed = %v, want the one line", result.Summed)
	}
}

func TestBothSidesMovingACountAlikeIsNotSummed(t *testing.T) {
	moved := strings.Replace(records, "B::y\ttraining\t2", "B::y\ttraining\t4", 1)
	if result := merge(t, records, moved, moved); string(result.Content) != moved || len(result.Summed) != 0 {
		t.Fatalf("got %q (summed %v)", result.Content, result.Summed)
	}
}

func TestLinesAddedAndRemovedByEitherSideMerge(t *testing.T) {
	ours := strings.Replace(records, "A::x\ttraining\t1\t9\n", "", 1) + "D::w\ttraining\t4\t9\n"
	theirs := strings.Replace(records, "B::y\ttraining\t2\t9\n", "B::y\ttraining\t2\t9\nBB::n\ttraining\t8\t9\n", 1)
	result := merge(t, records, ours, theirs)
	want := "# rule\troot\tours\tpilot\nB::y\ttraining\t2\t9\nBB::n\ttraining\t8\t9\nC::z\ttraining\t3\t9\nD::w\ttraining\t4\t9\n"
	if string(result.Content) != want {
		t.Fatalf("got %q, want %q", result.Content, want)
	}
}

func TestALineOneSideRemovedAndTheOtherMovedConflicts(t *testing.T) {
	removed := strings.Replace(records, "B::y\ttraining\t2\t9\n", "", 1)
	moved := strings.Replace(records, "B::y\ttraining\t2", "B::y\ttraining\t7", 1)
	requireConflict(t, records, removed, moved, "removed")
	requireConflict(t, records, moved, removed, "removed")
}

func TestALineBothSidesAddedWithDifferentCountsConflicts(t *testing.T) {
	requireConflict(t, records, records+"D::w\ttraining\t4\t9\n", records+"D::w\ttraining\t5\t9\n", "both sides added")
}

// The same field under two parents is two cells, and the elements of a list are told apart by position.
func TestIndentedDocumentsAreAddressedUnderTheirParents(t *testing.T) {
	base := "{\n  \"ours\": {\n    \"value\": 1\n  },\n  \"pilot\": {\n    \"value\": 1\n  },\n  \"roots\": [\n    {\n      \"files\": 3\n    },\n    {\n      \"files\": 3\n    }\n  ]\n}\n"
	ours := strings.Replace(base, "\"ours\": {\n    \"value\": 1", "\"ours\": {\n    \"value\": 2", 1)
	theirs := strings.Replace(base, "\"files\": 3\n    }\n  ]", "\"files\": 8\n    }\n  ]", 1)
	want := strings.Replace(ours, "\"files\": 3\n    }\n  ]", "\"files\": 8\n    }\n  ]", 1)
	if result := merge(t, base, ours, theirs); string(result.Content) != want {
		t.Fatalf("got %s, want %s", result.Content, want)
	}
}

// Several counts on one line are cells of their own.
func TestEveryCountOfALineIsACell(t *testing.T) {
	base := "root examples: models 56, skipped 0\n  A::x: equal 5, different 1, symbol-missing 0\n"
	ours := strings.Replace(base, "equal 5", "equal 6", 1)
	theirs := strings.Replace(strings.Replace(base, "different 1", "different 0", 1), "models 56", "models 57", 1)
	want := "root examples: models 57, skipped 0\n  A::x: equal 6, different 0, symbol-missing 0\n"
	if result := merge(t, base, ours, theirs); string(result.Content) != want {
		t.Fatalf("got %q, want %q", result.Content, want)
	}
}

// A digit inside a name, a version, a date or a digest is text: only a standing count is a cell.
func TestOnlyStandingCountsAreCells(t *testing.T) {
	for raw, want := range map[string]int{
		"A01-001\tpass":                   0,
		"\"pilotTag\": \"2026-09.1\",":    0,
		"\"digest\": \"sha256:07e537e5\"": 0,
		"\"files\": 100,":                 1,
		"Feature::type\ttraining\t1\t20":  2,
		"\"recorded\": \"2026-11-15\"":    0,
		"equal 5, different 1":            2,
		"\"ratio\": 0.5,":                 0,
		"\"padded\": 007,":                0,
	} {
		if got := len(split(raw).counts); got != want {
			t.Errorf("%q: %d counts, want %d", raw, got, want)
		}
		if got := split(raw).text(); got != raw {
			t.Errorf("%q does not round-trip: %q", raw, got)
		}
	}
}

// A digest or date both sides rewrote is not a count: keeping both lines is no version of the file.
func TestTextBothSidesReplacedConflicts(t *testing.T) {
	doc := func(date string) string {
		return "{\n  \"provenance\": {\n    \"pilotJava\": \"21\",\n    \"recorded\": \"" + date + "\"\n  },\n  \"files\": 3\n}\n"
	}
	requireConflict(t, doc("2026-11-14"), doc("2026-11-15"), doc("2026-11-16"), "2026-11-14")
	requireConflict(t, "\"digest\": \"sha256:aaa\"\n", "\"digest\": \"sha256:bbb\"\n", "\"digest\": \"sha256:ccc\"\n", "sha256:aaa")
	for _, sides := range [][2]string{{"2026-11-15", "2026-11-15"}, {"2026-11-15", "2026-11-14"}, {"2026-11-14", "2026-11-16"}} {
		want := sides[0]
		if want == "2026-11-14" {
			want = sides[1]
		}
		if got := string(merge(t, doc("2026-11-14"), doc(sides[0]), doc(sides[1])).Content); got != doc(want) {
			t.Errorf("ours %s, theirs %s: got %q, want %q", sides[0], sides[1], got, doc(want))
		}
	}
}

func TestAMergeOfUnmovedVersionsIsTheFile(t *testing.T) {
	for _, content := range []string{records, strings.TrimSuffix(records, "\n"), "", "{\n  \"a\": [\n    1,\n    1\n  ]\n}\n"} {
		if result := merge(t, content, content, content); string(result.Content) != content {
			t.Errorf("got %q, want %q", result.Content, content)
		}
	}
}
