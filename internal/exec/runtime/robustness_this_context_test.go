package runtime

import (
	"strings"
	"testing"
)

// TestRuntimeRobustnessThisContext covers `this` where no object gives it a
// context, and where the context it has is not what a feature typed for it holds.
func TestRuntimeRobustnessThisContext(t *testing.T) {
	t.Run("unperformed_package_level_action_this_stays_undetermined", testUnperformedPackageLevelActionThisStaysUndetermined)
	t.Run("performer_not_conforming_to_a_typed_this_redefinition_is_refused", testPerformerNotConformingToTypedThisIsRefused)
	t.Run("exhibited_machine_binding_this_to_its_own_definition_type_is_refused", testExhibitedMachineBindingThisToItsDefinitionIsRefused)
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

// testPerformerNotConformingToTypedThisIsRefused performs, from a Q, a definition
// whose `this` is redefined as `ref part :>> this : H`: the performer is the
// value of `this`, and a Q is no H, so instantiating q is refused with that reason.
func testPerformerNotConformingToTypedThisIsRefused(t *testing.T) {
	src := `
	package Cross {
		private import ScalarValues::*;
		part def H {
			attribute level : Integer = 10;
			action def Nudge { ref part :>> this : H; attribute seen : Integer = this.level; }
		}
		part def Q { perform action n : H::Nudge; }
		part q : Q;
	}`
	ctx, idx := libraryShapeContext(t, src)
	root := idx.DocumentRoot("<test>")
	_, err := evalIn(t, ctx, root, "Cross::q")
	if err == nil || !strings.Contains(err.Error(), "typed by H") {
		t.Fatalf("Cross::q: got %v, want a refusal of the performer as the value of this typed by H", err)
	}
}

// testExhibitedMachineBindingThisToItsDefinitionIsRefused exhibits a state
// definition whose entry action binds `this` to a parameter typed by that
// definition: `this` of an exhibited machine is the exhibiting part, not a Life.
func testExhibitedMachineBindingThisToItsDefinitionIsRefused(t *testing.T) {
	src := `
	package T {
		private import ScalarValues::*;
		action def Observer { in ref context : Life[1]; first o; action o { assign context.flag := true; } first o then done; }
		state def Life {
			attribute flag : Boolean = false;
			entry; then work;
			state work { entry action run { first call; action call : Observer { in ref :>> context = this; } first call then done; } }
		}
		part def Host { exhibit state life : Life; }
		part host : Host;
	}`
	ctx, idx := libraryShapeContext(t, src)
	root := idx.DocumentRoot("<test>")
	_, err := evalIn(t, ctx, root, "T::host")
	if err == nil || !strings.Contains(err.Error(), "typed by Life") {
		t.Fatalf("T::host: got %v, want the binding of this (the Host) to a feature typed by Life refused", err)
	}
}
