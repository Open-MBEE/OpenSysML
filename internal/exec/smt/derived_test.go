package smt

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
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

const derivedSrc = `package test {
	private import ScalarValues::*;
	action def Gate {
		attribute x : Integer = 0;
		attribute y : Integer = x + 1;
		attribute v : Integer = y * 2;
		attribute took : Boolean = false;
		requirement tracks { require y == x + 1; }
		requirement small { require v < 12; }
		first start;
		action set { assign x := 5; }
		decide pick;
		action hi { assign took := true; }
		action lo { assign took := false; }
		done;
		succession first start then set;
		succession first set then pick;
		succession first pick if y > 3 then hi;
		succession first pick if y <= 3 then lo;
		succession first hi then done;
		succession first lo then done;
	}
	action def Free {
		attribute n : Integer = 0;
		attribute y : Integer = n + 1;
		requirement nonneg { require n >= 0; }
		constraint positive { y > 0 }
		first start;
		done;
		succession first start then done;
	}
}`

// TestDerivedFeatureInGuardAndProperty: a guard, a chained feature and a property
// read a feature valued by an expression at its current value — after the write
// of what it reads, not at its starting value — and an assumption over the
// initial state constrains the released input the expression reads.
func TestDerivedFeatureInGuardAndProperty(t *testing.T) {
	e := engine(t)
	solver := requireSolver(t)
	d := indexed(t, "derived.sysml", derivedSrc)
	budget := analysis.Budget{Depth: 8}

	action := lookup(t, d.idx, "test::Gate")
	encoding, refusal := encodeDocument(t, d, action, budget)
	if refusal != nil {
		t.Fatalf("refused: %v", refusal)
	}
	exploration := explore(t, runtime.DefaultExploreBudget, d.fresh(budget), action)
	outcomes := compareOutcomes(t, solver, encoding, d, action, budget, exploration)
	if want := []string{"took = true; v = 12; x = 5; y = 6"}; !slices.Equal(outcomes.found, want) || !outcomes.agreeing || outcomes.replayed != 1 {
		t.Errorf("the solver reaches %q (agreeing %v, %d replayed), want %q", outcomes.found, outcomes.agreeing, outcomes.replayed, want)
	}
	expect(t, answer(t, e, d, d.holds(t, "test::Gate", "test::Gate::tracks"), budget), analysis.ClaimHolds, analysis.Proved)
	expect(t, answer(t, e, d, d.holds(t, "test::Gate", "test::Gate::small"), budget), analysis.ClaimViolated, analysis.Witnessed)

	q := d.holds(t, "test::Free", "test::Free::nonneg")
	q.Holds.Inputs = []string{"n"}
	expect(t, answer(t, e, d, q, budget), analysis.ClaimViolated, analysis.Witnessed)
	q.Holds.Assume = []*symbols.Symbol{lookup(t, d.idx, "test::Free::positive")}
	expect(t, answer(t, e, d, q, budget), analysis.ClaimHolds, analysis.Proved)
}

// TestDerivedFeatureRefusals: what the encoding of a feature valued by an
// expression does not cover is refused naming the feature and the reason, and a
// cycle among such features is the interpreter's refusal at the start, before
// any encoding.
func TestDerivedFeatureRefusals(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
		e := engine(t)
		d := indexed(t, "cycle.sysml", `package test {
	private import ScalarValues::*;
	action m {
		attribute y : Integer = w + 1;
		attribute w : Integer = y * 2;
		first start;
		done;
		succession first start then done;
	}
}`)
		_, err := e.Run(context.Background(), d.model, d.holds(t, "test::m", ""), analysis.Budget{Depth: 4})
		if want := "cyclic feature value dependency: y"; err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("run: %v, want the interpreter's refusal %q", err, want)
		}
	})
	const flow = `
		first start;
		action a { %s }
		done;
		succession first start then a;
		succession first a then done;
	}
}`
	model := func(features, body string) string {
		return "package test {\n\tprivate import ScalarValues::*;\n\taction m {\n" + features + fmt.Sprintf(flow, body)
	}
	cases := []struct {
		name, src, release, want string
	}{
		{"assignment", model("attribute x : Integer = 0; attribute y : Integer = x + 1;", "assign y := 7;"), "",
			"assignment to y: a feature valued by an expression over other features holds the value written until written again"},
		{"release", model("attribute x : Integer = 0; attribute y : Integer = x + 1;", "assign x := 7;"), "y",
			"action m values it by an expression over other features; free a feature it reads instead"},
		{"narrowed type", model("attribute x : Integer = 0; attribute y : Natural = x + 1;", "assign x := 7;"), "",
			"attribute y: a feature valued by an expression over other features, with a declared type narrowing its sort, is not encoded"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, action, graph, held := loweredAction(t, c.src, "test::m")
			var releases []string
			if c.release != "" {
				releases = []string{c.release}
			}
			_, err := Encode(ctx, action, graph, held, releases, 4, DefaultUnroll)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("encode: %v, want a refusal containing %q", err, c.want)
			}
		})
	}
}
