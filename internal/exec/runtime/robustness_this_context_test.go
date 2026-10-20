package runtime

import "testing"

// TestRuntimeRobustnessThisContext covers `this` where no object gives it a
// context: a performance nothing performs has no `this` to answer.
func TestRuntimeRobustnessThisContext(t *testing.T) {
	t.Run("unperformed_package_level_action_this_stays_undetermined", testUnperformedPackageLevelActionThisStaysUndetermined)
}

// testUnperformedPackageLevelActionThisStaysUndetermined reads `this`, a feature
// valued by `this` and one in a subaction of a package-level action usage no
// object performs: each is undetermined, not the action itself.
func testUnperformedPackageLevelActionThisStaysUndetermined(t *testing.T) {
	src := `
	package Probe {
		action def A { attribute who = this; action sub { attribute w = this; } }
		action a : A;
	}`
	ctx, idx := libraryShapeContext(t, src)
	root := idx.DocumentRoot("<test>")
	for _, path := range []string{"Probe::a", "Probe::a.this", "Probe::a.who", "Probe::a.sub.w"} {
		val, err := evalIn(t, ctx, root, path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if val.Kind != ValUndetermined {
			t.Errorf("%s = %s, want <undetermined>", path, FormatValue(val))
		}
	}
}
