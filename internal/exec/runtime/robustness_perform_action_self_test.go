package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessPerformActionSelf exercises the failure modes of a
// `perform` declaration whose binding names an element holding no body: only a
// declaration naming no element at all is its own body. A name that resolves
// to nothing is a nameres diagnostic upstream, so each case resolves to an
// element that still states no body — or to none through an unresolvable
// clause, which the model builder tolerates while the analyzer reports it.
func TestRuntimeRobustnessPerformActionSelf(t *testing.T) {
	t.Run("reference_form_names_no_action", testPerformReferenceNamesNoAction)
	t.Run("reference_subsetting_names_nothing", testPerformSubsettingNamesNothing)
	t.Run("typing_names_nothing", testPerformTypingNamesNothing)
}

// testPerformReferenceNamesNoAction: `perform go;` binds the element `go`,
// which here is an attribute stating no behavior body, so the chain between the
// two never ends at an action. (An unresolvable `go` derives no member at all
// and is reported by the analyzer instead, so it cannot reach the runtime.)
func testPerformReferenceNamesNoAction(t *testing.T) {
	src := `
		part def P {
			attribute go;
			perform go;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(typedModel(model, resolver), 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "P"))
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
	if !strings.Contains(err.Error(), "go") {
		t.Errorf("error %q does not name the behavior", err)
	}
}

// testPerformSubsettingNamesNothing: `perform action go ::> missing` states a
// reference subsetting whose target resolves to no element, so no body is
// reached and the usage is not its own body — it named one.
func testPerformSubsettingNamesNothing(t *testing.T) {
	src := `
		part def P {
			action declared;
			perform action go ::> missing;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(typedModel(model, resolver), 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "P"))
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
}

// testPerformTypingNamesNothing: `perform action go : Nothing` states a typing
// that resolves to no element, so no body is reached and the usage is not its
// own body — it named one.
func testPerformTypingNamesNothing(t *testing.T) {
	src := `
		part def P {
			perform action go : Nothing;
		}
	`
	model, resolver, root := parseAndBuildModel(t, src)
	ctx := NewContext(typedModel(model, resolver), 10000)

	_, err := ctx.Instantiate(resolveSymbol(t, root, "P"))
	if !errors.Is(err, ErrUnresolvedClassifierBehavior) {
		t.Fatalf("error = %v, want ErrUnresolvedClassifierBehavior", err)
	}
}
