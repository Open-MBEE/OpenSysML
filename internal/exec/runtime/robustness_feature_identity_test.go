package runtime

import (
	"testing"
)

// TestRuntimeRobustnessFeatureIdentity covers feature values of features that
// share a name on one type: a redefined feature and its redefiner read one value,
// a namesake inherited along another path keeps its own, and a chain reads the
// feature its segment resolves to rather than whatever the object has under the name.
func TestRuntimeRobustnessFeatureIdentity(t *testing.T) {
	t.Run("namesake_inherited_along_another_path_keeps_its_value", testNamesakeInheritedAlongAnotherPathKeepsItsValue)
	t.Run("two_redefined_namesakes_keep_two_values", testTwoRedefinedNamesakesKeepTwoValues)
	t.Run("diamond_inherited_feature_reads_the_redefiner", testDiamondInheritedFeatureReadsTheRedefiner)
}

// readsIn evaluates each expression in the package scope and compares the rendering.
func readsIn(t *testing.T, ctx *Context, pkg string, want map[string]string) {
	t.Helper()
	scope := lookupOne(t, ctx.model.resolver.Index(), pkg).Scope
	for expr, value := range want {
		got, err := evalIn(t, ctx, scope, expr)
		if err != nil || FormatValue(got) != value {
			t.Errorf("%s = %s, %v; want %s", expr, FormatValue(got), err, value)
		}
	}
}

// `Both :> A2, A1 { b :>> A1::a }` inherits `A2::a` under `a` and holds `A1::a`
// under `b`, in either order of the generals and when a usage owns the redefinition.
func testNamesakeInheritedAlongAnotherPathKeepsItsValue(t *testing.T) {
	_, _, ctx := buildRuntimeWithLibraries(t, "<namesake>", parseAndBuild(t, `package P {
		part def A1 { attribute a : ScalarValues::Real default = 1.0; }
		part def A2 { attribute a : ScalarValues::Real default = 2.0; }
		part def Both21 :> A2, A1 { attribute b :>> A1::a = 3.0; }
		part def Both12 :> A1, A2 { attribute b :>> A1::a = 3.0; }
		part bd21 : Both21;
		part bd12 : Both12;
		part both : A2, A1 { attribute b :>> A1::a = 3.0; }
	}`))
	for _, obj := range []string{"bd21", "bd12", "both"} {
		readsIn(t, ctx, "P", map[string]string{
			obj + ".a":              "2.0",
			obj + ".b":              "3.0",
			"(" + obj + " as A1).a": "3.0",
			"(" + obj + " as A2).a": "2.0",
		})
	}
}

// Redefining both namesakes under different names keeps two values apart, each
// read under its redefiner's name and as the general that declares it.
func testTwoRedefinedNamesakesKeepTwoValues(t *testing.T) {
	_, _, ctx := buildRuntimeWithLibraries(t, "<two-namesakes>", parseAndBuild(t, `package P {
		part def A1 { attribute a : ScalarValues::Real default = 1.0; }
		part def A2 { attribute a : ScalarValues::Real default = 2.0; }
		part def Both :> A2, A1 { attribute b :>> A1::a = 3.0; attribute c :>> A2::a = 4.0; }
		part bd : Both;
	}`))
	readsIn(t, ctx, "P", map[string]string{
		"bd.b":         "3.0",
		"bd.c":         "4.0",
		"(bd as A1).a": "3.0",
		"(bd as A2).a": "4.0",
	})
}

// One feature inherited along two paths is one feature: redefining it through
// either path values it for every name and every general.
func testDiamondInheritedFeatureReadsTheRedefiner(t *testing.T) {
	_, _, ctx := buildRuntimeWithLibraries(t, "<diamond>", parseAndBuild(t, `package P {
		part def A0 { attribute a : ScalarValues::Real default = 1.0; }
		part def D1 :> A0;
		part def D2 :> A0;
		part def Diamond :> D2, D1 { attribute b :>> D1::a = 3.0; }
		part dia : Diamond;
	}`))
	readsIn(t, ctx, "P", map[string]string{
		"dia.b":         "3.0",
		"(dia as D1).a": "3.0",
		"(dia as D2).a": "3.0",
		"(dia as A0).a": "3.0",
	})
}
