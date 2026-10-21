package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const holdingFeatureTypesModel = `package test {
	item def Panel;
	item def Rig :> Objects::StructuredSpaceObject {
		item front : Panel :> faces;
		item spare : Panel;
	}
	item rig : Rig;
}`

// An object made as a feature has every type the feature inherits as a direct type
// (KerML 1.0 §7.3.4.3–§7.3.4.4), and none of what those specialize.
func TestDirectTypesOfAnObjectMadeAsAFeatureAreEveryTypeItInherits(t *testing.T) {
	ctx, idx := libraryShapeContext(t, holdingFeatureTypesModel)
	rig := instantiateQualified(t, ctx, idx, "test::rig")
	panel := lookupOne(t, idx, "test::Panel")
	structured := lookupOne(t, idx, "Objects::StructuredSpaceObject::StructuredSurface")
	surface := lookupOne(t, idx, "Objects::Surface")

	front := readInstance(t, ctx, rig, "front")
	fv, err := rig.GetFeatureValue(ctx, "front")
	if err != nil {
		t.Fatalf("GetFeatureValue(front): %v", err)
	}
	types, err := ctx.directValueTypes(rig.Type.Scope, fv.HeldValue())
	if err != nil {
		t.Fatalf("directValueTypes(front): %v", err)
	}
	if len(types) != 2 || types[0] != panel || types[1] != structured {
		t.Fatalf("direct types of front = %v, want Panel then StructuredSurface", types)
	}
	for sym, want := range map[*symbols.Symbol]bool{panel: true, structured: true, surface: false, front.Type: true} {
		if got := ctx.isDirectTypeOf(front, sym); got != want {
			t.Errorf("isDirectTypeOf(front, %s) = %t, want %t", sym.Name, got, want)
		}
	}

	spare := readInstance(t, ctx, rig, "spare")
	if got := ctx.directTypes(spare.Type); len(got) != 1 || got[0] != panel {
		t.Errorf("direct types of spare = %v, want Panel alone", got)
	}
	if ctx.isDirectTypeOf(spare, structured) {
		t.Error("spare, held by nothing that subsets faces, is a direct StructuredSurface")
	}
}

// directTypes of a definition, or of a feature stating no type, is the symbol itself.
func TestDirectTypesOfAnUntypedSymbolIsItself(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Panel;
		item def Rack { item raw [1]; }
	}`)
	for _, name := range []string{"test::Panel", "test::Rack::raw"} {
		sym := lookupOne(t, idx, name)
		if got := ctx.directTypes(sym); len(got) != 1 || got[0] != sym {
			t.Errorf("directTypes(%s) = %v, want the symbol itself", name, got)
		}
	}
}

// Every classification surface consults the types an object inherits through the feature
// holding it: `istype` and `@` answer true for the subsetted feature's type and what it
// specializes, `hastype` for the direct types only, and `as` keeps the object.
func TestClassificationConsultsTheTypesOfTheFeatureHoldingAnObject(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		item def Panel;
		item def Rig :> Objects::StructuredSpaceObject {
			item front : Panel :> faces;
			item spare : Panel;
			attribute isStructured = front istype Objects::StructuredSpaceObject::StructuredSurface;
			attribute isSurface = front istype Objects::Surface;
			attribute atStructured = front @ Objects::StructuredSpaceObject::StructuredSurface;
			attribute isPanel = front istype Panel;
			attribute hasPanel = front hastype Panel;
			attribute hasStructured = front hastype Objects::StructuredSpaceObject::StructuredSurface;
			attribute hasSurface = front hastype Objects::Surface;
			attribute isCurve = front istype Objects::StructuredSpaceObject::StructuredCurve;
			attribute spareIsStructured = spare istype Objects::StructuredSpaceObject::StructuredSurface;
			item asSurface [1] = front as Objects::Surface;
		}
		item rig : Rig;
	}`)
	rig := instantiateQualified(t, ctx, idx, "test::rig")
	for name, want := range map[string]bool{
		"isStructured": true, "isSurface": true, "atStructured": true, "isPanel": true,
		"hasPanel": true, "hasStructured": true, "hasSurface": false,
		"isCurve": false, "spareIsStructured": false,
	} {
		if got := readBool(t, ctx, rig, name); got != want {
			t.Errorf("%s = %t, want %t", name, got, want)
		}
	}
	if readInstance(t, ctx, rig, "asSurface") != readInstance(t, ctx, rig, "front") {
		t.Error("front as Objects::Surface is not the object front holds")
	}
}
