package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessExhibitStateSelf exercises the failure modes of an
// `exhibit` declaration whose binding names an element holding no body: only a
// declaration naming no element at all is its own body. A name that resolves
// to nothing is a nameres diagnostic upstream, so each case resolves to an
// element that still states no body — or to none through an unresolvable
// clause, which the model builder tolerates while the analyzer reports it.
func TestRuntimeRobustnessExhibitStateSelf(t *testing.T) {
	t.Run("reference_form_names_no_state", testExhibitReferenceNamesNoState)
	t.Run("reference_subsetting_names_nothing", testExhibitSubsettingNamesNothing)
	t.Run("typing_names_nothing", testExhibitTypingNamesNothing)
}

// testExhibitReferenceNamesNoState: `exhibit modes;` binds the element
// `modes`, which here is an attribute stating no behavior body, so the chain
// between the two never ends at a state. (An unresolvable `modes` derives no
// member at all and is reported by the analyzer instead, so it cannot reach
// the runtime.)
func testExhibitReferenceNamesNoState(t *testing.T) {
	src := `
		part def P {
			attribute modes;
			exhibit modes;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(typedModel(model, resolver), 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "P"))
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
	if !strings.Contains(err.Error(), "modes") {
		t.Errorf("error %q does not name the behavior", err)
	}
}

// testExhibitSubsettingNamesNothing: `exhibit state s ::> missing` states a
// reference subsetting whose target resolves to no element, so no body is
// reached and the usage is not its own body — it named one.
func testExhibitSubsettingNamesNothing(t *testing.T) {
	src := `
		part def P {
			state declared;
			exhibit state s ::> missing;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(typedModel(model, resolver), 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "P"))
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
}

// testExhibitTypingNamesNothing: `exhibit state s : Nothing` states a typing
// that resolves to no element, so no body is reached and the usage is not its
// own body — it named one.
func testExhibitTypingNamesNothing(t *testing.T) {
	src := `
		part def P {
			exhibit state s : Nothing;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(typedModel(model, resolver), 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "P"))
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
}
