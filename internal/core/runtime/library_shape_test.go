package runtime

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// libraryShapeContext builds a runtime over the bundled standard library and the
// given model.
func libraryShapeContext(t *testing.T, src string) (*Context, *symbols.Index) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	return ctx, idx
}

// sortedFeatureNames is the shape's feature names, sorted for set comparison.
func sortedFeatureNames(features []EffectiveFeature) []string {
	names := featureNames(features)
	sort.Strings(names)
	return names
}

// A part def gets the Systems Library's description of a part — its ports,
// actions, states, sub-items and the Item geometry — while the Kernel frame it
// restates (`self`, `start :>> startShot`, `done :>> endShot`) stays out.
func TestShapeKeepsSystemsLibraryFeaturesAndLeavesOutTheKernelFrame(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		part def P;
		item def I;
		port def Po;
		action def A;
		occurrence def O;
		attribute def At;
		enum def E { a; b; }
	}`)

	want := map[string][]string{
		"test::P": {
			"boundingShapes", "checkedConstraints", "envelopingShapes", "exhibitedStates",
			"isSolid", "ownedActions", "ownedPorts", "ownedStates", "performedActions",
			"shape", "subitems", "subparts", "voids",
		},
		"test::I": {
			"boundingShapes", "checkedConstraints", "envelopingShapes", "isSolid", "shape",
			"subitems", "subparts", "voids",
		},
		"test::Po": {"interfacingPorts", "subports"},
		// An occurrence def specializes the Kernel alone; a value type holds a value.
		"test::O":  nil,
		"test::At": nil,
		"test::E":  nil,
	}
	for fqn, names := range want {
		got := sortedFeatureNames(ctx.FeaturesOf(oneSymbol(t, idx, fqn)))
		if len(got) == 0 && len(names) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, names) {
			t.Errorf("features of %s = %v, want %v", fqn, got, names)
		}
	}

	action := sortedFeatureNames(ctx.FeaturesOf(oneSymbol(t, idx, "test::A")))
	for _, kept := range []string{"subactions", "decisions", "forks", "joins", "merges"} {
		if !containsString(action, kept) {
			t.Errorf("action def lacks %s: %v", kept, action)
		}
	}
	for _, frame := range []string{"self", "start", "done", "incomingTransfers", "startShot", "endShot", "portions", "timeSlices", "snapshots", "localClock"} {
		if containsString(action, frame) {
			t.Errorf("action def materializes the frame feature %s: %v", frame, action)
		}
	}
}

// The tier decides: a Kernel member frames the object, a Systems member that only
// restates the frame (`self`, `start`, `done`) or is a behavior's parameter
// (`RequirementConstraintCheck::result`) is frame too, and a value type's members
// are its value's, not an object's.
func TestFrameFeatureClassifiesLibraryMembers(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test { part def P; }`)
	for fqn, want := range map[string]bool{
		"Occurrences::Occurrence::self":                    true,
		"Occurrences::Occurrence::portions":                true,
		"Occurrences::Occurrence::spaceBoundary":           true,
		"Items::Item::self":                                true,
		"Items::Item::start":                               true,
		"Items::Item::done":                                true,
		"Ports::Port::outgoingTransfersFromSelf":           true,
		"Actions::Action::incomingTransfers":               true,
		"Requirements::RequirementConstraintCheck::result": true,
		"Requirements::RequirementCheck::self":             true,
		"Cases::Case::result":                              true,
		"ISQBase::LengthValue::num":                        true,
		"Items::Item::isSolid":                             false,
		"Items::Item::voids":                               false,
		"Items::Item::shape":                               false,
		"Items::Item::subitems":                            false,
		"Parts::Part::ownedPorts":                          false,
		"Requirements::RequirementCheck::subj":             false,
		"Requirements::RequirementCheck::actors":           false,
		"Requirements::RequirementCheck::assumptions":      false,
		"ShapeItems::RectangularCuboid::length":            false,
		"test::P":                                          false,
	} {
		if got := ctx.model.semantics.FrameFeature(oneSymbol(t, idx, fqn)); got != want {
			t.Errorf("FrameFeature(%s) = %v, want %v", fqn, got, want)
		}
	}
}

// Inherited library features carry the multiplicity, default and type the
// library declares, and the model's own redefinition masks them in place, so a
// materialized library feature behaves as one of the model's own.
func TestLibraryFeaturesInheritDeclarationsAndMask(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		part def Plain;
		part def Hollow {
			attribute :>> isSolid = false;
			part :>> subparts : Plain[2];
		}
	}`)

	plain := ctx.FeaturesOf(oneSymbol(t, idx, "test::Plain"))
	isSolid := plain[indexOfFeature(t, plain, "isSolid")]
	if isSolid.DefaultValue == nil || symbols.FQNOf(isSolid.Symbol) != "Items::Item::isSolid" {
		t.Fatalf("Plain.isSolid = %+v, want the library's derivation", isSolid)
	}
	if !isSolid.Scalar() {
		t.Errorf("Plain.isSolid multiplicity = %v, want scalar", isSolid.Multiplicity)
	}
	voids := plain[indexOfFeature(t, plain, "voids")]
	if voids.Multiplicity.Lower.Value != 0 || !voids.Multiplicity.Upper.Infinite {
		t.Errorf("Plain.voids multiplicity = %v, want 0..*", voids.Multiplicity)
	}
	if voids.Type == nil || symbols.FQNOf(voids.Type) != "Occurrences::Occurrence" {
		t.Errorf("Plain.voids type = %v, want the library's Occurrence", voids.Type)
	}

	hollow := ctx.FeaturesOf(oneSymbol(t, idx, "test::Hollow"))
	if got, want := sortedFeatureNames(hollow), sortedFeatureNames(plain); !reflect.DeepEqual(got, want) {
		t.Errorf("Hollow features = %v, want the same names as Plain %v", got, want)
	}
	masked := hollow[indexOfFeature(t, hollow, "isSolid")]
	if symbols.FQNOf(masked.Symbol) != "test::Hollow::isSolid" || masked.DefaultValue == nil {
		t.Errorf("Hollow.isSolid = %s, want the model's redefinition", symbols.FQNOf(masked.Symbol))
	}
	subparts := hollow[indexOfFeature(t, hollow, "subparts")]
	if subparts.Multiplicity.Lower.Value != 2 || subparts.Multiplicity.Upper.Value != 2 {
		t.Errorf("Hollow.subparts multiplicity = %v, want the model's 2..2", subparts.Multiplicity)
	}
	if subparts.Type == nil || symbols.FQNOf(subparts.Type) != "test::Plain" {
		t.Errorf("Hollow.subparts type = %v, want test::Plain", subparts.Type)
	}
}

// The shape is stable: two resolutions of one model list the features in the
// same order, with the model's own declarations before the inherited ones.
func TestLibraryFeatureOrderIsStable(t *testing.T) {
	const src = `package test {
		private import ScalarValues::*;
		part def Truck { attribute mass : Real = 1.0; part cab : Truck[0..1]; }
	}`
	first, idx := libraryShapeContext(t, src)
	second, idx2 := libraryShapeContext(t, src)
	a := featureNames(first.FeaturesOf(oneSymbol(t, idx, "test::Truck")))
	b := featureNames(second.FeaturesOf(oneSymbol(t, idx2, "test::Truck")))
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("feature order differs between resolutions: %v vs %v", a, b)
	}
	if len(a) < 2 || a[0] != "mass" || a[1] != "cab" {
		t.Errorf("features = %v, want the model's own first", a)
	}
}

// An object reads its inherited library features as the model's: isSolid derives
// true from the empty voids, an optional empty collection is the empty sequence,
// and a required feature holding nothing is the typed uninitialized error.
func TestInstanceReadsInheritedLibraryFeatures(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ShapeItems::*;
		private import ISQ::*;
		private import SI::*;
		part def Crate {
			part box : Box {
				:>> length = 2 [m];
				:>> width = 1 [m];
				:>> height = 1 [m];
			}
		}
		requirement def MassLimit;
	}`)
	crate, err := ctx.Instantiate(oneSymbol(t, idx, "test::Crate"))
	if err != nil {
		t.Fatalf("instantiate Crate: %v", err)
	}
	box := objectAt(t, ctx, crate, "box")

	for name, want := range map[string]string{"isSolid": "true", "voids": "[]", "shape": "[]"} {
		fv, err := box.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("box.%s: %v", name, err)
		}
		val, err := fv.ReadValue(name)
		if err != nil {
			t.Fatalf("read box.%s: %v", name, err)
		}
		if got := FormatValue(val); got != want {
			t.Errorf("box.%s = %s, want %s", name, got, want)
		}
	}
	if _, err := box.GetFeatureValue(ctx, "self"); err == nil || !strings.Contains(err.Error(), `feature "self" not found`) {
		t.Errorf("box.self error = %v, want the frame feature not found", err)
	}

	req, err := ctx.Instantiate(oneSymbol(t, idx, "test::MassLimit"))
	if err != nil {
		t.Fatalf("instantiate MassLimit: %v", err)
	}
	subj, err := req.GetFeatureValue(ctx, "subj")
	if err != nil {
		t.Fatalf("MassLimit.subj: %v", err)
	}
	if _, err := subj.ReadValue("subj"); !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Errorf("reading the required subj = %v, want ErrUninitializedFeatureValue", err)
	}
	actors, err := req.GetFeatureValue(ctx, "actors")
	if err != nil {
		t.Fatalf("MassLimit.actors: %v", err)
	}
	if val, err := actors.ReadValue("actors"); err != nil || FormatValue(val) != "[]" {
		t.Errorf("MassLimit.actors = %v, %v; want the empty sequence", val, err)
	}
}

// Materializing a part costs one object: the inherited library collections hold
// nothing until the model puts something in them.
func TestInheritedLibraryFeaturesMaterializeNoObjects(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ShapeItems::*;
		part def Crate { part box : Box; }
	}`)
	crate, err := ctx.Instantiate(oneSymbol(t, idx, "test::Crate"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	box := objectAt(t, ctx, crate, "box")
	for _, name := range []string{"voids", "shape", "subparts", "subitems", "isSolid"} {
		if _, err := box.GetFeatureValue(ctx, name); err != nil {
			t.Fatalf("box.%s: %v", name, err)
		}
	}
	if n := len(ctx.instances); n != 2 {
		t.Errorf("instantiating Crate made %d objects, want the crate and its box", n)
	}
}

// A shape digest names a library type rather than expanding it — the library
// resolves the same way in every context — so it stays small however much the
// Geometry library describes, and an edit to the model still changes it.
func TestShapeDigestNamesLibraryTypes(t *testing.T) {
	const src = `package test {
		private import ShapeItems::*;
		part def Crate { part box : Box; attribute n = 1; }
	}`
	ctx, idx := libraryShapeContext(t, src)
	digest := ctx.ShapeDigest(oneSymbol(t, idx, "test::Crate"))
	if len(digest) > 2000 {
		t.Errorf("digest of Crate is %d bytes, want the library types named, not expanded", len(digest))
	}
	if !strings.Contains(digest, "box:1..1@ShapeItems::RectangularCuboid/itemDef#") {
		t.Errorf("digest does not name the box's library type: %s", digest)
	}
	if strings.Contains(digest, "…") {
		t.Errorf("digest cut a recursion the library types should not have opened: %s", digest)
	}
	other, idx2 := libraryShapeContext(t, strings.Replace(src, "part box : Box;", "part box : Box[2];", 1))
	if other.ShapeDigest(oneSymbol(t, idx2, "test::Crate")) == digest {
		t.Errorf("digest unchanged by an edit to the model's own multiplicity")
	}
}

// A Systems-tier library feature whose derivation the runtime cannot evaluate
// is the typed materialization error when read, never a panic or a silent value.
func TestUnevaluableLibraryFeatureIsATypedError(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("Lib.sysml", parseAndBuild(t, `package Lib {
		abstract part def Base {
			attribute broken = 1 / 0;
			attribute unsupported = ComplexFunctions::ToString(1);
		}
	}`))
	idx.MarkLibraryTier("Lib.sysml", symbols.TierSystems)
	idx.AddDocument("<test>", parseAndBuild(t, `package test { part def P :> Lib::Base; }`))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)

	obj, err := ctx.Instantiate(oneSymbol(t, idx, "test::P"))
	if err != nil {
		t.Fatalf("instantiate P: %v", err)
	}
	if got := sortedFeatureNames(ctx.FeaturesOf(obj.Type)); !containsString(got, "broken") || !containsString(got, "unsupported") {
		t.Fatalf("P features = %v, want the library's broken and unsupported", got)
	}
	for name, want := range map[string]error{"broken": ErrDivisionByZero, "unsupported": ErrUnevaluableLibraryFunction} {
		fv, err := obj.GetFeatureValue(ctx, name)
		if err == nil {
			_, err = fv.ReadValue(name)
		}
		if err == nil {
			t.Errorf("P.%s read a value, want a typed error", name)
			continue
		}
		if !errors.Is(err, want) {
			t.Errorf("P.%s error = %v, want %v", name, err, want)
		}
	}
}

// objectAt is the object the named feature of obj holds.
func objectAt(t *testing.T, ctx *Context, obj *Instance, name string) *Instance {
	t.Helper()
	fv, err := obj.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	val := fv.HeldValue()
	id, ok := val.Object()
	if !ok {
		t.Fatalf("%s holds %s, want an object", name, FormatValue(val))
	}
	held, found := ctx.Instance(id)
	if !found {
		t.Fatalf("%s holds object %d, which the context does not know", name, id)
	}
	return held
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// A requirement's subject redefines the `subj` it inherits from the Systems
// Library without naming it, so the two names read one feature value; the
// same holds for an analysis case's objective and `obj`.
func TestSubjectAndObjectiveShareTheInheritedRoleFeature(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		requirement def Lim { subject s : Real; }
		requirement lim : Lim { subject v : Real; }
		analysis def An { objective o { } }
	}`)
	shared := func(fqn string, names ...string) {
		obj, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
		if err != nil {
			t.Fatalf("instantiate %s: %v", fqn, err)
		}
		var first *FeatureValue
		for _, name := range names {
			fv, err := obj.GetFeatureValue(ctx, name)
			if err != nil {
				t.Fatalf("%s.%s: %v", fqn, name, err)
			}
			if first == nil {
				first = fv
			} else if fv != first {
				t.Errorf("%s.%s is a feature value of its own, want the one %s reads", fqn, name, names[0])
			}
		}
	}
	shared("test::Lim", "s", "subj")
	shared("test::lim", "v", "s", "subj")
	shared("test::An", "o", "obj")
}

// A subject's declared type governs what binds to it, declared or inherited:
// an object of an unrelated type is refused and leaves the subject untouched.
func TestSubjectDeclaredTypeGovernsItsBinding(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		part def Vehicle;
		part def Rock;
		part car : Vehicle;
		part rock : Rock;
		requirement def MassLimit { subject v : Vehicle; }
		requirement def TruckMassLimit :> MassLimit;
	}`)
	objectOf := func(fqn string) Value {
		obj, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
		if err != nil {
			t.Fatalf("instantiate %s: %v", fqn, err)
		}
		return Value{Kind: ValInstance, Instance: obj.ID}
	}
	car, rock := objectOf("test::car"), objectOf("test::rock")
	for _, fqn := range []string{"test::MassLimit", "test::TruckMassLimit"} {
		req, err := ctx.Instantiate(oneSymbol(t, idx, fqn))
		if err != nil {
			t.Fatalf("instantiate %s: %v", fqn, err)
		}
		subject, err := req.GetFeatureValue(ctx, "v")
		if err != nil {
			t.Fatalf("%s.v: %v", fqn, err)
		}
		if subject.Feature.Type == nil || subject.Feature.Type.Name != "Vehicle" {
			t.Fatalf("%s.v is typed %v, want Vehicle", fqn, subject.Feature.Type)
		}
		err = req.SetFeatureValue(ctx, "v", rock)
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("%s.v = rock: %v, want ErrTypeMismatch", fqn, err)
		}
		if held := subject.HeldValue(); held.Kind != ValInvalid || subject.Written {
			t.Errorf("%s.v holds %s after the refused binding, want it untouched", fqn, FormatValue(held))
		}
		if err := req.SetFeatureValue(ctx, "v", car); err != nil {
			t.Errorf("%s.v = car: %v", fqn, err)
		}
	}
}

// A case has one subject: the one a requirement declares redefines each general's,
// by `:>>` or implicitly (the pinned pilot accepts this without "Only one subject").
func TestSubjectRedefinesEveryInheritedSubject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::*;
		requirement def Weight { subject w : Real; }
		requirement def Volume { subject vol : Real; }
		requirement def Both :> Weight, Volume { subject s : Real :>> w; }
	}`)
	obj, err := ctx.Instantiate(oneSymbol(t, idx, "test::Both"))
	if err != nil {
		t.Fatalf("instantiate Both: %v", err)
	}
	if err := obj.SetFeatureValue(ctx, "s", realConst(2.5)); err != nil {
		t.Fatalf("Both.s = 2.5: %v", err)
	}
	for _, name := range []string{"s", "w", "vol"} {
		fv, err := obj.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("Both.%s: %v", name, err)
		}
		got, err := fv.ReadValue(name)
		if err != nil {
			t.Fatalf("Both.%s: %v", name, err)
		}
		if got.Kind != ValConst || got.Const.Real != 2.5 {
			t.Errorf("Both.%s = %s, want 2.5 through the one subject", name, FormatValue(got))
		}
	}
}

// A redefinition chain is followed through a declaration masking its target
// in the object's own type: a Box face reads `shape` as Items::Item::shape,
// which redefines the `spaceBoundary` the face carries as
// Objects::StructuredSpaceObject::faces::spaceBoundary, restated there without a
// multiplicity, so Occurrence::spaceBoundary's [0..1] governs it.
func TestRedefinitionChainFollowsMaskedTargets(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ShapeItems::*;
		item box : Box;
	}`)
	obj, err := ctx.Instantiate(oneSymbol(t, idx, "test::box"))
	if err != nil {
		t.Fatalf("instantiate box: %v", err)
	}
	fv, err := obj.GetFeatureValue(ctx, "tf")
	if err != nil {
		t.Fatalf("box.tf: %v", err)
	}
	face := ctx.instances[fv.HeldValue().Instance]
	if face == nil {
		t.Fatalf("box.tf = %s, want a face instance", FormatValue(fv.HeldValue()))
	}
	for _, feat := range ctx.FeaturesOf(face.Type) {
		if feat.Name != "shape" {
			continue
		}
		if lower := feat.Multiplicity.Lower; !lower.Known || lower.Value != 0 {
			t.Errorf("tf.shape lower bound = %v, want 0 from Occurrence::spaceBoundary", lower)
		}
		shape, err := face.GetFeatureValue(ctx, "shape")
		if err != nil {
			t.Fatalf("tf.shape: %v", err)
		}
		if got := shape.HeldValue(); got.Kind != ValInvalid {
			t.Errorf("tf.shape = %s, want an unset optional feature", FormatValue(got))
		}
		return
	}
	t.Fatal("tf has no shape feature")
}
