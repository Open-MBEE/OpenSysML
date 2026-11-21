package suggest

import (
	"fmt"
	"testing"
)

func TestEditDistance(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"mass", "mass", 0},
		{"mass", "mas", 1},
		{"mass", "masss", 1},
		{"mass", "maas", 1},
		{"mass", "msas", 1},
		{"abcd", "badc", 2},
		{"kitten", "sitting", 3},
		{"Länge", "Lange", 1},
		{"a", "PowerInterfaceDefinition", 23},
	}
	// One distancer runs every case twice, so a shorter or longer earlier pair must leave nothing behind.
	var d distancer
	for range 2 {
		for _, c := range cases {
			if got := EditDistance(c.a, c.b); got != c.want {
				t.Errorf("EditDistance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
			}
			if got := d.distance(c.a, c.b); got != c.want {
				t.Errorf("reused distance(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
			}
		}
	}
}

// A search allocates its buffers once, not once per candidate.
func TestNeighboursAllocationsDoNotGrowWithCandidates(t *testing.T) {
	candidates := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("candidate%04d", i)
		}
		return out
	}
	few, many := candidates(10), candidates(1000)
	allocs := func(c []string) float64 {
		return testing.AllocsPerRun(20, func() {
			if hits := Neighbours("unrelatedword", c); len(hits) != 0 {
				t.Fatalf("unexpected neighbours %v", hits)
			}
		})
	}
	if a, b := allocs(few), allocs(many); a != b {
		t.Errorf("Neighbours allocated %v times over 10 candidates and %v over 1000", a, b)
	}
}
