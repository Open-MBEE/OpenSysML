package suggest_test

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
)

// A candidate reached by its short name ranks behind one named as typed at the
// same distance and reach, before the shorter spelling wins.
func TestRankNamesBeforeShortNames(t *testing.T) {
	got := suggest.Rank([]suggest.Candidate{
		{Spelling: "SI::min", Library: true, Short: true},
		{Spelling: "DataFunctions::min", Library: true},
	})
	want := []string{"DataFunctions::min", "SI::min"}
	if !slices.Equal(got, want) {
		t.Fatalf("Rank = %v, want %v", got, want)
	}
}
