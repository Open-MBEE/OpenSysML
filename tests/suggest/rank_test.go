package suggest_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
)

// TestBudget locks how many edits a name of each length justifies as a typo: a
// short name is close to too much to guess from.
func TestBudget(t *testing.T) {
	tests := []struct {
		length int
		want   int
	}{
		{length: 1, want: 0},
		{length: 2, want: 0},
		{length: 3, want: 1},
		{length: 5, want: 1},
		{length: 6, want: 2},
		{length: 8, want: 2},
		{length: 9, want: 3},
		{length: 40, want: 3},
	}

	for _, tt := range tests {
		if got := suggest.Budget(tt.length); got != tt.want {
			t.Errorf("suggest.Budget(%d) = %d, want %d", tt.length, got, tt.want)
		}
	}
}

// TestRank covers each ranking rule: how far a candidate is from the typed
// name, how the user would reach it, whose declaration it is, and that a better
// candidate rules a worse one out rather than being listed beside it.
func TestRank(t *testing.T) {
	inScope := func(name string, d int) suggest.Candidate {
		return suggest.Candidate{Spelling: name, Distance: d, InScope: true}
	}
	qualified := func(name string, d int) suggest.Candidate {
		return suggest.Candidate{Spelling: name, Distance: d, Library: true}
	}

	tests := []struct {
		name  string
		cands []suggest.Candidate
		want  []string
	}{
		{
			name:  "none",
			cands: nil,
			want:  nil,
		},
		{
			name:  "closer wins alone",
			cands: []suggest.Candidate{inScope("Wheel", 1), inScope("Wheels", 2)},
			want:  []string{"Wheel"},
		},
		{
			name:  "two edits further than the best is not the same guess",
			cands: []suggest.Candidate{inScope("Wheel", 1), inScope("Barrel", 3)},
			want:  []string{"Wheel"},
		},
		{
			name:  "a name usable as written beats one only a path reaches",
			cands: []suggest.Candidate{inScope("Wheel", 1), qualified("SysML::Systems::TriggerKind::when", 1)},
			want:  []string{"Wheel"},
		},
		{
			name:  "the user's own declaration beats a bundled library name",
			cands: []suggest.Candidate{inScope("Sensor", 1), suggest.Candidate{Spelling: "Sensing", Distance: 1, InScope: true, Library: true}},
			want:  []string{"Sensor"},
		},
		{
			name:  "an unimported library name is offered by its path",
			cands: []suggest.Candidate{qualified("ScalarValues::Integer", 0), qualified("VectorFunctions::inner", 1)},
			want:  []string{"ScalarValues::Integer"},
		},
		{
			name:  "neither rules the other out: both are offered, closest first",
			cands: []suggest.Candidate{qualified("ScalarValues::Integer", 0), inScope("Intake", 1)},
			want:  []string{"ScalarValues::Integer", "Intake"},
		},
		{
			name: "at most Limit are offered",
			cands: []suggest.Candidate{
				inScope("Aaaa", 1), inScope("Bbbb", 1), inScope("Cccc", 1), inScope("Dddd", 1),
			},
			want: []string{"Aaaa", "Bbbb", "Cccc"},
		},
		{
			name:  "equal candidates order by length then name",
			cands: []suggest.Candidate{inScope("Wheels", 1), inScope("Wheel", 1), inScope("Wheal", 1)},
			want:  []string{"Wheal", "Wheel", "Wheels"},
		},
		{
			name:  "the same spelling is offered once",
			cands: []suggest.Candidate{inScope("Wheel", 1), inScope("Wheel", 1)},
			want:  []string{"Wheel"},
		},
		{
			name:  "an empty spelling is not offered",
			cands: []suggest.Candidate{{Distance: 0, InScope: true}, inScope("Wheel", 1)},
			want:  []string{"Wheel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggest.Rank(tt.cands)
			if len(got) != len(tt.want) {
				t.Fatalf("suggest.Rank(...) = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("suggest.Rank(...) = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

// TestNeighboursSpendBudgetOnCaseButRankOnIt: a wrong case is within the budget,
// yet a name spelled as typed is the closer guess.
func TestNeighboursSpendBudgetOnCaseButRankOnIt(t *testing.T) {
	got := suggest.Neighbours("Intger", []string{"Integer", "inter", "integer"})
	want := map[string]int{"Integer": 1, "inter": 2, "integer": 2}
	if len(got) != len(want) {
		t.Fatalf("suggest.Neighbours(...) = %v, want %d candidates", got, len(want))
	}
	for _, n := range got {
		if want[n.Name] != n.Distance {
			t.Errorf("distance of %q = %d, want %d", n.Name, n.Distance, want[n.Name])
		}
	}
	if got[0].Name != "Integer" {
		t.Errorf("suggest.Neighbours(...) = %v, want Integer first", got)
	}
}
