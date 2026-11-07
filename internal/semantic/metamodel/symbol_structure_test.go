package metamodel

import (
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func newSymbolStructureModel(t *testing.T, name, text string) (*SymbolStructure, *Evaluator, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(text)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	index := libs.NewModelIndex()
	index.AddDocumentWithKind(name, root, source.KindOf(name))
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	resolver.ResolveDocument(name, root)
	structure := NewSymbolStructure(resolver, model)
	return structure, New(resolver, model, Options{Structure: structure}), index.DocumentRoot(name)
}

const symbolStructureModel = `package P {
    part def Vehicle { attribute mass : ScalarValues::Real; part wheel : Vehicle; }
    part car : Vehicle { attribute :>> mass = 1200.0; }
    action a { first start; action step; succession first start then step; }
}`

func TestSymbolStructureProjectsDeclaredElements(t *testing.T) {
	structure, evaluator, root := newSymbolStructureModel(t, "structure.sysml", symbolStructureModel)
	pkg := testSymbol(t, root, "P")
	vehicle := testSymbol(t, pkg.Scope, "Vehicle")
	car := testSymbol(t, pkg.Scope, "car")

	element := structure.ElementFor(car)
	if again := structure.ElementFor(car); again.Key() != element.Key() {
		t.Fatalf("ElementFor is not stable: %v vs %v", element, again)
	}
	if sym, ok := structure.SymbolOf(element); !ok || sym != car {
		t.Fatalf("SymbolOf(ElementFor(car)) = %v, %v", sym, ok)
	}
	if class, ok := structure.Metaclass(element); !ok || class != "PartUsage" {
		t.Fatalf("Metaclass(car) = %q, %v", class, ok)
	}
	if !structure.Specializes("PartUsage", "Usage") || structure.Specializes("Usage", "PartUsage") {
		t.Fatal("Specializes does not follow the metamodel library")
	}
	owning, present, known := structure.OwningRelationship(element)
	if !known || !present || !owning.IsMembership {
		t.Fatalf("OwningRelationship(car) = %v, %v, %v", owning, present, known)
	}
	if owner, present, known := structure.OwningRelatedElement(owning); !known || !present || owner.Key() != structure.ElementFor(pkg).Key() {
		t.Fatalf("OwningRelatedElement(membership of car) = %v, %v, %v", owner, present, known)
	}
	if value, ok := structure.Attribute(element, "declaredName"); !ok || value.Kind != StringValue || value.String != "car" {
		t.Fatalf("Attribute(car, declaredName) = %v, %v", value, ok)
	}
	wheel := structure.ElementFor(testSymbol(t, vehicle.Scope, "wheel"))
	if value, ok := structure.Attribute(wheel, "isComposite"); !ok || value.Kind != BooleanValue || !value.Boolean {
		t.Fatalf("Attribute(wheel, isComposite) = %v, %v", value, ok)
	}
	typed := testProperty(t, evaluator, element, "Feature", "type")
	if elements := valueElements(t, typed); len(elements) != 1 || elements[0].Key() != structure.ElementFor(vehicle).Key() {
		t.Fatalf("Feature::type over the symbol structure = %v", typed)
	}
	members := testProperty(t, evaluator, structure.ElementFor(vehicle), "Namespace", "ownedMember")
	if elements := valueElements(t, members); len(elements) != 2 {
		t.Fatalf("Namespace::ownedMember(Vehicle) = %v", members)
	}
}

func TestSymbolStructureProjectsInitialNodeMarker(t *testing.T) {
	structure, evaluator, root := newSymbolStructureModel(t, "marker.sysml", symbolStructureModel)
	action := testSymbol(t, testSymbol(t, root, "P").Scope, "a")
	relationships, ok := structure.OwnedRelationships(structure.ElementFor(action))
	if !ok {
		t.Fatal("OwnedRelationships(a) unknown")
	}
	var marker Element
	found := false
	for _, rel := range relationships {
		if class, _ := structure.Metaclass(rel); class == "Membership" {
			marker, found = rel, true
		}
	}
	if !found {
		t.Fatalf("`first start;` is not projected as a Membership of a: %v", relationships)
	}
	if _, present, known := structure.OwningRelationship(marker); !known || present {
		t.Fatal("a Membership is owned by its namespace, not through another membership")
	}
	member := testProperty(t, evaluator, marker, "Membership", "memberElement")
	if member.Kind != ElementValue {
		t.Fatalf("Membership::memberElement of `first start;` = %v", member)
	}
	if sym, ok := structure.SymbolOf(member.Element); !ok || sym.Name != "start" || sym.Decl == nil {
		t.Fatalf("memberElement is not the library start action: %v", sym)
	}
	if name := testProperty(t, evaluator, marker, "Element", "name"); name.Kind != NullValue {
		t.Fatalf("Element::name of a Membership = %v", name)
	}
	if value, ok := structure.StatedAttribute(marker, "declaredName"); ok {
		t.Fatalf("StatedAttribute(marker, declaredName) = %v, stated; the Membership writes no name", value)
	}
}

func TestSymbolStructureRejectsForeignHandles(t *testing.T) {
	first, _, firstRoot := newSymbolStructureModel(t, "first.sysml", symbolStructureModel)
	second, _, secondRoot := newSymbolStructureModel(t, "second.sysml", `package Q { part def Person; }`)
	vehicle := first.ElementFor(testSymbol(t, testSymbol(t, firstRoot, "P").Scope, "Vehicle"))
	person := second.ElementFor(testSymbol(t, testSymbol(t, secondRoot, "Q").Scope, "Person"))
	if vehicle.Key() == person.Key() {
		t.Fatalf("two structures issued the same handle %v", vehicle)
	}
	if sym, ok := second.SymbolOf(vehicle); ok {
		t.Fatalf("second.SymbolOf(first handle) = %v, true", sym)
	}
	if _, ok := second.Metaclass(vehicle); ok {
		t.Fatal("second.Metaclass(first handle) is known")
	}
	if sym, ok := first.SymbolOf(vehicle); !ok || sym.Name != "Vehicle" {
		t.Fatalf("first.SymbolOf(own handle) = %v, %v", sym, ok)
	}
}

func TestSymbolStructureIsSafeForConcurrentReads(t *testing.T) {
	structure, evaluator, root := newSymbolStructureModel(t, "concurrent.sysml", symbolStructureModel)
	pkg := structure.ElementFor(testSymbol(t, root, "P"))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, property := range []string{"ownedMember", "member", "ownedMembership"} {
				if _, ok := evaluator.PropertyReadOnlyUncached(pkg, "Namespace", property); !ok {
					t.Errorf("Namespace::%s unsupported", property)
				}
			}
		}()
	}
	wg.Wait()
}
