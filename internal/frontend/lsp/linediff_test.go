package lsp

import (
	"math/rand"
	"strings"
	"testing"
)

// applyHunks rebuilds the new side from a and the hunks, the way an editor
// applying the edits would; any hunk out of order or overlapping fails.
func applyHunks(t *testing.T, a, b []string, hunks []hunk) []string {
	t.Helper()
	var out []string
	oldPos, newPos := 0, 0
	for i, h := range hunks {
		if h.oldStart < oldPos || h.oldEnd < h.oldStart || h.newStart < newPos || h.newEnd < h.newStart {
			t.Fatalf("hunk %d %+v is out of order or inverted (old at %d, new at %d)", i, h, oldPos, newPos)
		}
		if i > 0 && h.oldStart == oldPos && h.newStart == newPos {
			t.Fatalf("hunk %d %+v abuts the one before it; they should have merged", i, h)
		}
		if h.oldStart-oldPos != h.newStart-newPos {
			t.Fatalf("hunk %d %+v: %d shared old lines before it but %d new", i, h, h.oldStart-oldPos, h.newStart-newPos)
		}
		for k := 0; k < h.oldStart-oldPos; k++ {
			if a[oldPos+k] != b[newPos+k] {
				t.Fatalf("hunk %d %+v: line %d before it is not shared (%q vs %q)", i, h, oldPos+k, a[oldPos+k], b[newPos+k])
			}
		}
		out = append(out, a[oldPos:h.oldStart]...)
		out = append(out, b[h.newStart:h.newEnd]...)
		oldPos, newPos = h.oldEnd, h.newEnd
	}
	if len(a)-oldPos != len(b)-newPos {
		t.Fatalf("after the hunks, %d old lines remain but %d new", len(a)-oldPos, len(b)-newPos)
	}
	out = append(out, a[oldPos:]...)
	return out
}

// lcsLength is the textbook quadratic longest-common-subsequence length, the
// reference for the diff's minimality.
func lcsLength(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else {
				cur[j] = max(prev[j], cur[j-1])
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// diffStrings diffs two line lists through the same interning formatEdits uses.
func diffStrings(a, b []string) []hunk {
	keys := lineKeys{ids: map[string]int{}}
	return diffLines(keys.of(a, 0), keys.of(b, 0))
}

func checkDiff(t *testing.T, a, b []string) []hunk {
	t.Helper()
	hunks := diffStrings(a, b)
	if got := applyHunks(t, a, b, hunks); strings.Join(got, "|") != strings.Join(b, "|") {
		t.Fatalf("diffLines(%q, %q) = %+v\napplied: %q", a, b, hunks, got)
	}
	edits := 0
	for _, h := range hunks {
		edits += (h.oldEnd - h.oldStart) + (h.newEnd - h.newStart)
	}
	if want := len(a) + len(b) - 2*lcsLength(a, b); edits != want {
		t.Fatalf("diffLines(%q, %q) = %+v: %d line edits, a shortest script has %d", a, b, hunks, edits, want)
	}
	return hunks
}

func TestDiffLinesHandCases(t *testing.T) {
	cases := []struct {
		name string
		a, b []string
		want []hunk
	}{
		{"both empty", nil, nil, nil},
		{"identical", []string{"a", "b"}, []string{"a", "b"}, nil},
		{"all inserted", nil, []string{"a", "b"}, []hunk{{0, 0, 0, 2}}},
		{"all deleted", []string{"a", "b"}, nil, []hunk{{0, 2, 0, 0}}},
		{"replace middle", []string{"a", "b", "c"}, []string{"a", "x", "c"}, []hunk{{1, 2, 1, 2}}},
		{"insert at end", []string{"a"}, []string{"a", "b"}, []hunk{{1, 1, 1, 2}}},
		{"insert at start", []string{"a"}, []string{"b", "a"}, []hunk{{0, 0, 0, 1}}},
		{"delete a blank of two", []string{"x", "", "", "y"}, []string{"x", "", "y"}, []hunk{{2, 3, 2, 2}}},
		{"two separate hunks", []string{"a", "b", "c", "d", "e"}, []string{"a", "B", "c", "d", "E"}, []hunk{{1, 2, 1, 2}, {4, 5, 4, 5}}},
		{"unequal replacement", []string{"a", "b", "c", "d"}, []string{"a", "x", "d"}, []hunk{{1, 3, 1, 2}}},
		{"nothing shared", []string{"a", "b"}, []string{"c", "d", "e"}, []hunk{{0, 2, 0, 3}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkDiff(t, tc.a, tc.b)
			if len(got) != len(tc.want) {
				t.Fatalf("hunks = %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("hunks = %+v, want %+v", got, tc.want)
				}
			}
		})
	}
}

// Every shortest script over small random inputs applies cleanly; the
// deliberately tiny alphabet makes shared and repeated lines common.
func TestDiffLinesRandomIsMinimalAndApplies(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	alphabet := []string{"a", "b", "c", ""}
	for iter := 0; iter < 5000; iter++ {
		a := make([]string, rng.Intn(9))
		for i := range a {
			a[i] = alphabet[rng.Intn(len(alphabet))]
		}
		// b is a mutated a about half the time, random otherwise, so both
		// mostly-shared and mostly-different pairs are covered.
		var b []string
		if rng.Intn(2) == 0 {
			b = append(b, a...)
			for n := rng.Intn(4); n > 0 && len(b) > 0; n-- {
				at := rng.Intn(len(b))
				switch rng.Intn(3) {
				case 0:
					b = append(b[:at], b[at+1:]...)
				case 1:
					b = append(b[:at+1], b[at:]...)
					b[at] = alphabet[rng.Intn(len(alphabet))]
				default:
					b[at] = alphabet[rng.Intn(len(alphabet))]
				}
			}
		} else {
			b = make([]string, rng.Intn(9))
			for i := range b {
				b[i] = alphabet[rng.Intn(len(alphabet))]
			}
		}
		checkDiff(t, a, b)
	}
}

// A long document with a few scattered changes diffs into exactly those hunks.
func TestDiffLinesLongDocumentSparseChanges(t *testing.T) {
	a := make([]string, 6000)
	for i := range a {
		a[i] = strings.Repeat("x", i%7)
	}
	b := append([]string(nil), a...)
	b[10] = "changed"
	b = append(b[:2000], b[2001:]...) // delete one line
	b = append(b[:4000], append([]string{"inserted"}, b[4000:]...)...)
	hunks := checkDiff(t, a, b)
	if len(hunks) != 3 {
		t.Fatalf("hunks = %+v, want three", hunks)
	}
}
