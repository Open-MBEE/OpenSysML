package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// runOccurrenceConformance performs the occurrence a case names as an interaction
// and checks the messages were realized in the order the case states, or that the
// run failed as it states.
func runOccurrenceConformance(t *testing.T, ctx *Context, idx *symbols.Index, path string, expected ExpectedOutcome) {
	t.Helper()
	rootScope := idx.DocumentRoot(path)
	sym := namedOrFoundSymbol(t, idx, expected.Evaluate, rootScope, ast.DefOccurrence, ast.UsageOccurrence)
	run, err := ctx.ExecuteOccurrence(sym)
	if expected.Error != "" {
		if err == nil {
			t.Fatalf("expected the occurrence to fail with %q, it realized %v", expected.Error, transferNames(run))
		}
		if !strings.Contains(err.Error(), expected.Error) {
			t.Fatalf("the occurrence failed with %q, want an error containing %q", err, expected.Error)
		}
		return
	}
	if err != nil {
		t.Fatalf("ExecuteOccurrence failed: %v", err)
	}
	if got := transferNames(run); !slices.Equal(got, expected.Transfers) {
		t.Errorf("transfers = %v, want %v", got, expected.Transfers)
	}
	for _, tr := range run.Transfers {
		if tr.From == nil || tr.To == nil {
			t.Errorf("transfer %s has no lifeline object at an end: from %v to %v", tr.Name, tr.From, tr.To)
		}
	}
}

func transferNames(run *OccurrenceRun) []string {
	if run == nil {
		return nil
	}
	names := make([]string, 0, len(run.Transfers))
	for _, tr := range run.Transfers {
		names = append(names, tr.Name)
	}
	return names
}
