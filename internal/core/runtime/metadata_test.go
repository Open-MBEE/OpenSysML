package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const metadataSrc = `
package test {
	private import ScalarValues::*;

	metadata def Safety {
		attribute isMandatory : Boolean;
		attribute level : Integer = 2;
	}

	metadata def Legacy;

	part def Vehicle;

	part seatBelt : Vehicle {
		@Safety {
			isMandatory = true;
		}
		@Legacy;
	}

	part plain : Vehicle;

	attribute mass = 3;
}

package about {
	metadata test::Legacy about test::plain;
}
`

// metadataTypeNames is the metadata type of every object in a `.metadata` sequence.
func metadataTypeNames(t *testing.T, ctx *Context, value Value) []string {
	t.Helper()
	if value.Kind != ValSequence {
		t.Fatalf("metadata read as %v, want a sequence", value.Kind)
	}
	var names []string
	for _, elem := range elementsOf(value) {
		inst, ok := ctx.getInstance(elem.Instance)
		if !ok {
			t.Fatalf("element %v is no object", elem.Kind)
		}
		names = append(names, ctx.qualifiedSymbolName(inst.Type))
	}
	return names
}

// featureValue is what an object holds for the named feature.
func featureValue(t *testing.T, ctx *Context, inst *Instance, name string) Value {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("feature %s: %v", name, err)
	}
	return fv.HeldValue()
}

// TestMetadataAccessBoundValues reads the metadata of an annotated element: one
// object per annotation, in the order stated, carrying the values its body binds
// and the defaults its type declares.
func TestMetadataAccessBoundValues(t *testing.T) {
	ctx, got, err := evalDeclaredExpr(t, metadataSrc, "test::seatBelt.metadata")
	if err != nil {
		t.Fatalf("seatBelt.metadata failed: %v", err)
	}
	if want := []string{"test::Safety", "test::Legacy"}; !slices.Equal(metadataTypeNames(t, ctx, got), want) {
		t.Errorf("metadata types = %v, want %v", metadataTypeNames(t, ctx, got), want)
	}
	safety, ok := ctx.getInstance(elementsOf(got)[0].Instance)
	if !ok {
		t.Fatal("the first metadata value is no object")
	}
	if v := featureValue(t, ctx, safety, "isMandatory"); FormatValue(v) != "true" {
		t.Errorf("isMandatory = %s, want true", FormatValue(v))
	}
	if v := featureValue(t, ctx, safety, "level"); FormatValue(v) != "2" {
		t.Errorf("level = %s, want the declared default 2", FormatValue(v))
	}
}

// TestMetadataAccessAboutForm reads metadata an `about` annotation states elsewhere.
func TestMetadataAccessAboutForm(t *testing.T) {
	ctx, got, err := evalDeclaredExpr(t, metadataSrc, "test::plain.metadata")
	if err != nil {
		t.Fatalf("plain.metadata failed: %v", err)
	}
	if want := []string{"test::Legacy"}; !slices.Equal(metadataTypeNames(t, ctx, got), want) {
		t.Errorf("metadata types = %v, want %v", metadataTypeNames(t, ctx, got), want)
	}
}

// TestMetadataAccessEmpty reads the metadata of an element nothing annotates.
func TestMetadataAccessEmpty(t *testing.T) {
	for _, expr := range []string{"test::mass.metadata", "test::Vehicle.metadata"} {
		ctx, got, err := evalDeclaredExpr(t, metadataSrc, expr)
		if err != nil {
			t.Fatalf("%s failed: %v", expr, err)
		}
		if names := metadataTypeNames(t, ctx, got); len(names) != 0 {
			t.Errorf("%s = %v, want the empty sequence", expr, names)
		}
	}
}

// TestMetadataAccessNotAnElement refuses `.metadata` read from a value.
func TestMetadataAccessNotAnElement(t *testing.T) {
	for _, expr := range []string{"1.metadata", `"abc".metadata`, "(1 + 2).metadata"} {
		_, _, err := evalDeclaredExpr(t, metadataSrc, expr)
		if err == nil {
			t.Fatalf("%s succeeded, want a typed error", expr)
		}
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s: error %v, want a type mismatch", expr, err)
		}
		if !strings.Contains(err.Error(), "metadata access requires an element") {
			t.Errorf("%s: error %q, want it to require an element", expr, err)
		}
	}
}

// TestMetadataAccessUnresolved refuses `.metadata` over a name that names nothing.
func TestMetadataAccessUnresolved(t *testing.T) {
	_, _, err := evalDeclaredExpr(t, metadataSrc, "test::nope.metadata")
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Errorf("error %v, want an unresolved reference", err)
	}
}

// TestMetadataAccessUnknownFeature refuses a body binding the metadata type declares
// no feature for, rather than dropping the value.
func TestMetadataAccessUnknownFeature(t *testing.T) {
	src := `
package test {
	metadata def Safety {
		attribute level : ScalarValues::Integer = 2;
	}

	part def Vehicle;

	part seatBelt : Vehicle {
		@Safety {
			severity = 3;
		}
	}
}
`
	_, _, err := evalDeclaredExpr(t, src, "test::seatBelt.metadata")
	if err == nil {
		t.Fatal("an unknown metadata feature succeeded, want a typed error")
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("error %v, want a type mismatch", err)
	}
	if !strings.Contains(err.Error(), "severity") {
		t.Errorf("error %q, want it to name severity", err)
	}
}

// TestMetadataAccessFailureAbandonsInstances requires one access to materialize
// every annotation or none: an annotation that fails leaves behind no object,
// and no behavior of one, of the annotations read before it.
func TestMetadataAccessFailureAbandonsInstances(t *testing.T) {
	src := `
package test {
	part def Controller {
		attribute count : ScalarValues::Integer = 0;
		exhibit state modes {
			entry; then running;
			state running {
				entry action bump { assign count := count + 1; }
			}
		}
	}

	part ctrl : Controller;

	metadata def Safety {
		attribute level : ScalarValues::Integer = 2;
	}

	part def Vehicle;

	part reader : Vehicle {
		@Safety {
			level = ctrl.count;
		}
	}

	part seatBelt : Vehicle {
		@Safety {
			level = ctrl.count;
		}
		@Safety {
			severity = 3;
		}
	}
}
`
	// The first annotation reads an object whose machine runs, so the failing
	// access has behaviors as well as objects to abandon.
	ok, _, err := evalDeclaredExpr(t, src, "test::reader.metadata")
	if err != nil {
		t.Fatalf("test::reader.metadata: %v", err)
	}
	if len(ok.objectBehaviors) == 0 {
		t.Fatal("reading the annotation started no behavior, so the test proves nothing")
	}

	ctx, _, err := evalDeclaredExpr(t, src, "test::seatBelt.metadata")
	if err == nil {
		t.Fatal("the failing annotation succeeded, want a typed error")
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("error %v, want a type mismatch", err)
	}
	if live := len(ctx.instances); live != 0 {
		t.Errorf("%d object(s) outlived the failed access, want none", live)
	}
	if created := len(ctx.created); created != 0 {
		t.Errorf("%d object(s) stay registered as created, want none", created)
	}
	if attached, pending := len(ctx.objectBehaviors), len(ctx.pendingBehaviors); attached != 0 || pending != 0 {
		t.Errorf("%d behavior(s) (%d pending) outlived the failed access, want none", attached, pending)
	}
}
