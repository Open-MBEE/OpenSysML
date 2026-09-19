package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestBodyScopeImportSpellings: the scope an action body is evaluated in is the
// one the document declares, so the two import spellings behave identically —
// `private import SI::*` brings the unit `m` into the package that wrote it just
// as `public import SI::*` does, the difference being what the package re-exports
// rather than what its own members see.
func TestBodyScopeImportSpellings(t *testing.T) {
	model := func(visibility string) string {
		return `
			package test {
				` + visibility + ` import ScalarValues::*;
				` + visibility + ` import SI::*;
				` + visibility + ` import ISQ::*;
				attribute pkgStep : ISQSpaceTime::TimeValue = 0.5 [s];
				action march {
					attribute t : ISQSpaceTime::TimeValue = 0.0 [s];
					first start;
					action step {
						assign t := t + pkgStep;
					}
					done;
					succession first start then step;
					succession first step then done;
				}
			}
		`
	}

	outputs := map[string]Value{}
	for _, visibility := range []string{"private", "public"} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model(visibility)))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "march", ast.DefAction)
		if sym == nil {
			t.Fatalf("%s import: action march not found", visibility)
		}
		out, err := ctx.ExecuteAction(sym)
		if err != nil {
			t.Fatalf("%s import: ExecuteAction: %v", visibility, err)
		}
		val, ok := out["t"]
		if !ok {
			t.Fatalf("%s import: no output for t: %v", visibility, out)
		}
		if val.Kind != ValQuantity || val.Quantity() == nil {
			t.Fatalf("%s import: t = %v; want a quantity", visibility, val)
		}
		if got, want := val.Quantity().Unit.String(), "s"; got != want {
			t.Errorf("%s import: unit of t = %q, want %q", visibility, got, want)
		}
		if got, want := val.Quantity().Num.Real, 0.5; got != want {
			t.Errorf("%s import: magnitude of t = %v, want %v", visibility, got, want)
		}
		outputs[visibility] = val
	}

	private, public := outputs["private"], outputs["public"]
	if private.Quantity().Num.Real != public.Quantity().Num.Real ||
		private.Quantity().Unit.String() != public.Quantity().Unit.String() {
		t.Errorf("private import gave %v %s, public import gave %v %s; want the same",
			private.Quantity().Num.Real, private.Quantity().Unit,
			public.Quantity().Num.Real, public.Quantity().Unit)
	}
}
