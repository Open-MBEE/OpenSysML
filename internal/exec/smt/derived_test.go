package smt

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// derivedPins are the outcomes a case whose action holds a feature valued by an
// expression over other features must prove, per feature: `y = x + 1` read after
// two racing writers of `x` stands on the choice between them, as does `z`
// written from it, rather than on the value `y` held at the start.
var derivedPins = map[string]map[string][]string{
	"action_order_write_from_bound_read": {"x": {"1", "2"}, "y": {"2", "3"}, "z": {"12", "13"}, "w": {"1"}},
}

// TestDerivedFeaturesAgreeWithExplore runs every conformance action holding a
// feature valued by an expression over other features through the encoding and
// the exhaustive exploration: the encoding admits the action, the outcomes the
// solver enumerates are the exploration's, every witness replays, and the set of
// values the solver proves for each feature is the set the exploration reached.
func TestDerivedFeaturesAgreeWithExplore(t *testing.T) {
	solver := requireSolver(t)
	cases := refereeCases(t)
	covered := 0
	for _, name := range slices.Sorted(maps.Keys(cases)) {
		c := cases[name]
		d, action := corpusDocument(t, name, c)
		budget := analysis.Budget{Depth: c.solverMoves()}
		if !holdsDerived(t, d, action, budget) {
			continue
		}
		covered++
		t.Run(name, func(t *testing.T) {
			encoding, refusal := encodeDocument(t, d, action, budget)
			if errors.Is(refusal, ErrMalformedFlow) {
				// A calculation holds derived features too but performs no flow to encode.
				t.Logf("outside the stage: %v", refusal)
				return
			}
			if refusal != nil {
				t.Fatalf("refused: %v", refusal)
			}
			exploration := explore(t, c.exploreBudget(), d.fresh(budget), action)
			if !exploration.Complete() {
				t.Fatalf("exploration %s", exploration.Status())
			}
			outcomes := compareOutcomes(t, solver, encoding, d, action, budget, exploration)
			if !outcomes.agreeing || outcomes.replayed != outcomes.witnesses {
				t.Errorf("%d witnesses, %d replayed, agreeing %v", outcomes.witnesses, outcomes.replayed, outcomes.agreeing)
			}
			explored, proved := valuesPerFeature(outcomes.explored), valuesPerFeature(outcomes.found)
			for feature, values := range explored {
				if !slices.Equal(values, proved[feature]) {
					t.Errorf("%s: the exploration reaches %v, the solver proves %v", feature, values, proved[feature])
				}
			}
			for feature, values := range derivedPins[name] {
				if !slices.Equal(proved[feature], values) {
					t.Errorf("%s: the solver proves %v, the pin says %v", feature, proved[feature], values)
				}
			}
		})
	}
	if covered == 0 {
		t.Fatal("no conformance action holds a feature valued by an expression over other features")
	}
	t.Logf("%d actions hold a feature valued by an expression over other features", covered)
}

// holdsDerived reports whether the action, started as the encoding is given it,
// holds a feature valued by an expression over other features.
func holdsDerived(t *testing.T, d *document, action *symbols.Symbol, budget analysis.Budget) bool {
	t.Helper()
	ctx, err := d.fresh(budget)()
	if err != nil {
		t.Fatalf("context: %v", err)
	}
	graph, held := started(t, ctx, action)
	for _, attr := range held.Features() {
		if _, derived := derivation(graph, attr); derived {
			return true
		}
	}
	return false
}

// valuesPerFeature projects outcome identities (`x = 1; y = 2`) onto the sorted
// set of values each feature takes across them.
func valuesPerFeature(identities []string) map[string][]string {
	values := make(map[string]map[string]bool)
	for _, identity := range identities {
		for _, binding := range strings.Split(identity, "; ") {
			name, value, ok := strings.Cut(binding, " = ")
			if !ok {
				continue
			}
			if values[name] == nil {
				values[name] = make(map[string]bool)
			}
			values[name][value] = true
		}
	}
	out := make(map[string][]string, len(values))
	for name, set := range values {
		out[name] = slices.Sorted(maps.Keys(set))
	}
	return out
}
