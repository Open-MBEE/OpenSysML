package runtime

import (
	"errors"
	"fmt"
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

// TestMetadataAccessNestedBindings reads an annotation whose body binds a value
// through a nested declaration: the object the outer feature holds carries it.
func TestMetadataAccessNestedBindings(t *testing.T) {
	src := `
package test {
	private import ScalarValues::*;

	metadata def Cause {
		attribute code : Integer = 0;
	}

	metadata def Risk {
		attribute level : Integer = 0;
		attribute note : String = "none";
		attribute cause : Cause;
	}

	metadata def Safety {
		attribute risk : Risk;
	}

	part def Vehicle;

	part seatBelt : Vehicle {
		@Safety {
			risk {
				level = 7;
				cause {
					code = 42;
				}
			}
		}
	}
}
`
	ctx, got, err := evalDeclaredExpr(t, src, "test::seatBelt.metadata")
	if err != nil {
		t.Fatalf("seatBelt.metadata failed: %v", err)
	}
	safety, ok := ctx.getInstance(elementsOf(got)[0].Instance)
	if !ok {
		t.Fatal("the metadata value is no object")
	}
	id, isObject := featureValue(t, ctx, safety, "risk").Object()
	if !isObject {
		t.Fatal("risk holds no object")
	}
	risk, ok := ctx.getInstance(id)
	if !ok {
		t.Fatal("the object risk holds is not live")
	}
	if v := featureValue(t, ctx, risk, "level"); FormatValue(v) != "7" {
		t.Errorf("risk.level = %s, want the nested binding 7", FormatValue(v))
	}
	if v := featureValue(t, ctx, risk, "note"); FormatValue(v) != `"none"` {
		t.Errorf("risk.note = %s, want the declared default", FormatValue(v))
	}
	causeID, isObject := featureValue(t, ctx, risk, "cause").Object()
	if !isObject {
		t.Fatal("cause holds no object")
	}
	cause, ok := ctx.getInstance(causeID)
	if !ok {
		t.Fatal("the object cause holds is not live")
	}
	if v := featureValue(t, ctx, cause, "code"); FormatValue(v) != "42" {
		t.Errorf("risk.cause.code = %s, want the nested binding 42", FormatValue(v))
	}
}

// TestMetadataAccessBindingScope reads a body value naming a feature of the
// metadata type that an element around the annotation also names: the body sees
// the metadata type's own member.
func TestMetadataAccessBindingScope(t *testing.T) {
	src := `
package test {
	private import ScalarValues::*;

	attribute limit = 1;

	metadata def Safety {
		attribute limit : Integer = 9;
		attribute level : Integer = 0;
	}

	part def Vehicle;

	part seatBelt : Vehicle {
		@Safety {
			level = limit;
		}
	}
}
`
	ctx, got, err := evalDeclaredExpr(t, src, "test::seatBelt.metadata")
	if err != nil {
		t.Fatalf("seatBelt.metadata failed: %v", err)
	}
	safety, ok := ctx.getInstance(elementsOf(got)[0].Instance)
	if !ok {
		t.Fatal("the metadata value is no object")
	}
	if v := featureValue(t, ctx, safety, "level"); FormatValue(v) != "9" {
		t.Errorf("level = %s, want 9, the limit the metadata type declares", FormatValue(v))
	}
}

// TestMetadataAccessRenamedRedefinition binds a metadata feature through a
// redefinition that gives it a name of its own, at both body levels.
func TestMetadataAccessRenamedRedefinition(t *testing.T) {
	src := `
package test {
	private import ScalarValues::*;

	metadata def Cause {
		attribute code : Integer = 0;
	}

	metadata def Safety {
		attribute level : Integer = 0;
		attribute cause : Cause;
	}

	part def Vehicle;

	part seatBelt : Vehicle {
		@Safety {
			attribute severity :>> level = 3;
			cause {
				attribute reason :>> code = 42;
			}
		}
	}
}
`
	ctx, got, err := evalDeclaredExpr(t, src, "test::seatBelt.metadata")
	if err != nil {
		t.Fatalf("seatBelt.metadata failed: %v", err)
	}
	safety, ok := ctx.getInstance(elementsOf(got)[0].Instance)
	if !ok {
		t.Fatal("the metadata value is no object")
	}
	if v := featureValue(t, ctx, safety, "level"); FormatValue(v) != "3" {
		t.Errorf("level = %s, want the redefining binding 3", FormatValue(v))
	}
	id, isObject := featureValue(t, ctx, safety, "cause").Object()
	if !isObject {
		t.Fatal("cause holds no object")
	}
	cause, ok := ctx.getInstance(id)
	if !ok {
		t.Fatal("the object cause holds is not live")
	}
	if v := featureValue(t, ctx, cause, "code"); FormatValue(v) != "42" {
		t.Errorf("cause.code = %s, want the redefining binding 42", FormatValue(v))
	}
}

// TestMetadataAccessDependentBindings reads a body value naming a feature an
// earlier binding of the same body bound: it reads what was bound, not the
// default the metadata type declares.
func TestMetadataAccessDependentBindings(t *testing.T) {
	src := `
package test {
	private import ScalarValues::*;

	metadata def Safety {
		attribute level : Integer = 0;
		attribute margin : Integer = 0;
	}

	part def Vehicle;

	part seatBelt : Vehicle {
		@Safety {
			level = 2;
			margin = level + 1;
		}
	}
}
`
	ctx, got, err := evalDeclaredExpr(t, src, "test::seatBelt.metadata")
	if err != nil {
		t.Fatalf("seatBelt.metadata failed: %v", err)
	}
	safety, ok := ctx.getInstance(elementsOf(got)[0].Instance)
	if !ok {
		t.Fatal("the metadata value is no object")
	}
	if v := featureValue(t, ctx, safety, "margin"); FormatValue(v) != "3" {
		t.Errorf("margin = %s, want 3, one more than the level bound before it", FormatValue(v))
	}
}

// TestMetadataAccessSameObjects reads the metadata of one element twice: an
// annotation denotes one object, so both reads answer it and no second object
// of the metadata type is made.
func TestMetadataAccessSameObjects(t *testing.T) {
	src := `
package test {
	private import ScalarValues::*;

	metadata def Safety {
		attribute level : Integer = 0;
	}

	part def Vehicle;

	part seatBelt : Vehicle {
		@Safety {
			level = 3;
		}
	}
}
`
	ctx, scope, decl := declaredExpr(t, src, "test::seatBelt.metadata")
	first, err := NewEvalContext(ctx, scope).Eval(decl)
	if err != nil {
		t.Fatalf("seatBelt.metadata failed: %v", err)
	}
	made := len(ctx.instances)
	second, err := NewEvalContext(ctx, scope).Eval(decl)
	if err != nil {
		t.Fatalf("the second seatBelt.metadata failed: %v", err)
	}
	one, two := elementsOf(first), elementsOf(second)
	if len(one) != 1 || len(two) != 1 {
		t.Fatalf("read %d then %d metadata values, want one each", len(one), len(two))
	}
	if one[0].Instance != two[0].Instance {
		t.Errorf("the reads answered objects %d and %d, want one object", one[0].Instance, two[0].Instance)
	}
	if len(ctx.instances) != made {
		t.Errorf("the second read left %d objects, want the %d the first did", len(ctx.instances), made)
	}
}

const adoptMetadataSrc = `package Demo {
	metadata def Safety { attribute level = 3; }
	part def Vehicle;
	part seatBelt : Vehicle {
		@Safety {
			level = 5;
		}
	}
	part def Holder { attribute mark; }
	part def Reader { attribute seen = seatBelt.metadata; }
}`

// TestAdoptKeepsTheObjectAnAnnotationDenotes carries an object holding what an
// annotation denotes into a re-analysis: the annotation still denotes that
// object there, so reading it again answers it rather than making a second one.
func TestAdoptKeepsTheObjectAnAnnotationDenotes(t *testing.T) {
	prev := contextOver(t, adoptMetadataSrc)
	reader, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Reader"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := reader.GetFeatureValue(prev, "seen")
	if err != nil {
		t.Fatalf("GetFeatureValue(seen): %v", err)
	}
	holder, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Holder"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := holder.SetFeatureValue(prev, "mark", fv.Value); err != nil {
		t.Fatalf("SetFeatureValue(mark): %v", err)
	}
	carried := elementsOf(fv.Value)
	if len(carried) != 1 {
		t.Fatalf("mark holds %d metadata values, want one", len(carried))
	}
	shapes := prev.ShapesOf(holder)

	ctx := contextOver(t, adoptMetadataSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, holder); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if _, found := ctx.Instance(carried[0].Instance); !found {
		t.Fatalf("the metadata object %d was not carried over, so the test proves nothing",
			carried[0].Instance)
	}
	made := len(ctx.instances)
	again, err := ctx.Instantiate(lookupOne(t, ctx.resolver.Index(), "Demo::Reader"))
	if err != nil {
		t.Fatalf("Instantiate after adoption: %v", err)
	}
	seen, err := again.GetFeatureValue(ctx, "seen")
	if err != nil {
		t.Fatalf("GetFeatureValue(seen) after adoption: %v", err)
	}
	read := elementsOf(seen.Value)
	if len(read) != 1 {
		t.Fatalf("the annotation reads as %d metadata values after adoption, want one", len(read))
	}
	if read[0].Instance != carried[0].Instance {
		t.Errorf("the annotation denotes object %d after adoption, want the carried %d",
			read[0].Instance, carried[0].Instance)
	}
	if len(ctx.instances) != made+1 {
		t.Errorf("reading the annotation again left %d objects, want the %d carried plus the reader",
			len(ctx.instances), made)
	}
}

// TestAdoptReadsAChangedAnnotationAgain edits the annotation body between the
// two analyses: the object made for what it said before does not stand for what
// it says now, so the annotation is read again and answers the new value.
func TestAdoptReadsAChangedAnnotationAgain(t *testing.T) {
	prev := contextOver(t, adoptMetadataSrc)
	reader, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Reader"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := reader.GetFeatureValue(prev, "seen")
	if err != nil {
		t.Fatalf("GetFeatureValue(seen): %v", err)
	}
	holder, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Holder"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	if err := holder.SetFeatureValue(prev, "mark", fv.Value); err != nil {
		t.Fatalf("SetFeatureValue(mark): %v", err)
	}
	carried := elementsOf(fv.Value)
	if len(carried) != 1 {
		t.Fatalf("the annotation reads as %d metadata values, want one", len(carried))
	}
	shapes := prev.ShapesOf(holder)

	ctx := contextOver(t, strings.Replace(adoptMetadataSrc, "level = 5", "level = 9", 1))
	if _, err := ctx.Adopt(prev, shapes, holder); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	again, err := ctx.Instantiate(lookupOne(t, ctx.resolver.Index(), "Demo::Reader"))
	if err != nil {
		t.Fatalf("Instantiate after adoption: %v", err)
	}
	seen, err := again.GetFeatureValue(ctx, "seen")
	if err != nil {
		t.Fatalf("GetFeatureValue(seen) after adoption: %v", err)
	}
	read := elementsOf(seen.Value)
	if len(read) != 1 {
		t.Fatalf("the annotation reads as %d metadata values after adoption, want one", len(read))
	}
	if read[0].Instance == carried[0].Instance {
		t.Fatalf("the edited annotation reused object %d, made for what it said before",
			carried[0].Instance)
	}
	obj, found := ctx.Instance(read[0].Instance)
	if !found {
		t.Fatalf("the annotation reads as object %d, which the context does not hold", read[0].Instance)
	}
	level, err := obj.GetFeatureValue(ctx, "level")
	if err != nil {
		t.Fatalf("GetFeatureValue(level): %v", err)
	}
	if got := fmt.Sprint(level.Value.Const); !strings.Contains(got, "9") {
		t.Errorf("level = %s, want the 9 the annotation states now", got)
	}
}
