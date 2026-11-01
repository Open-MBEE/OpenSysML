package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// deriveUsageMayTimeVary (SysML v2.1 §8.3.6.4): a composite usage that
// specializes Actions::Action does not vary in time; a performed action and an
// exhibited state are referential by the notation, so they do; a required or
// assumed constraint is the ConstraintUsage its member declares; a flow's
// payload is a PayloadFeature, no usage, and never varies in time; a variant's
// VariantMembership is no FeatureMembership, so it has no owningType.
func TestMayTimeVaryReadsTheEffectiveComposite(t *testing.T) {
	src := `package Vary {
	state def S;
	part def Engine { perform action generateTorque; exhibit state s : S; action own; part sub : Engine; }
	requirement def R;
	requirement r : R { require constraint c2 { true } assume constraint c4 { true } constraint c3 { true } }
	item def Fuel;
	part def Car { part tank; part eng; flow fuelFlow of fuel : Fuel from tank to eng; }
	variation part def EngineChoices :> Engine { variant part e4 : Engine; variant part e6 : Engine; part plain : Engine; }
	attribute values : Real[*];
	attribute positives = values->select { in v : Real; v > 0 };
}
`
	for i, fixture := range reflectiveFixtures(t, "vary.sysml", source.KindSysML, src) {
		vary, ok := fixture.root.LookupLocal("Vary")
		if !ok || vary.Scope == nil {
			t.Fatal("Vary not found")
		}
		engine, ok := vary.Scope.LookupLocal("Engine")
		if !ok || engine.Scope == nil {
			t.Fatal("Vary::Engine not found")
		}
		r, ok := vary.Scope.LookupLocal("r")
		if !ok || r.Scope == nil {
			t.Fatal("Vary::r not found")
		}
		car, ok := vary.Scope.LookupLocal("Car")
		if !ok || car.Scope == nil {
			t.Fatal("Vary::Car not found")
		}
		choices, ok := vary.Scope.LookupLocal("EngineChoices")
		if !ok || choices.Scope == nil {
			t.Fatal("Vary::EngineChoices not found")
		}
		flow, ok := car.Scope.LookupLocal("fuelFlow")
		if !ok || flow.Scope == nil {
			t.Fatal("Vary::Car::fuelFlow not found")
		}
		// An expression body's parameter is owned by the Expression, an Occurrence;
		// an interface record keeps no expression body, so only the resolved model reads it.
		if i == 0 {
			item := bodyParameter(t, fixture.root, "v")
			for name, got := range map[string]bool{"mayTimeVary": fixture.model.UsageMayTimeVary(item), "isVariable": fixture.model.FeatureIsVariable(item)} {
				if !got {
					t.Errorf("fixture %d: body parameter v %s = false, want true", i, name)
				}
			}
		}
		if fuel, ok := flow.Scope.LookupLocal("fuel"); !ok {
			t.Fatal("Vary::Car::fuelFlow::fuel not found")
		} else if meta := fixture.model.MetaclassOf(fuel); meta == nil || meta.Name != "PayloadFeature" {
			t.Errorf("fixture %d: fuelFlow::fuel metaclass = %v, want PayloadFeature", i, meta)
		}
		for _, c := range []struct {
			owner *symbols.Symbol
			name  string
			want  bool
		}{
			{engine, "generateTorque", true}, {engine, "s", true}, {engine, "own", false},
			{r, "c2", true}, {r, "c4", true}, {r, "c3", true},
			{car, "fuelFlow", false}, {flow, "fuel", false},
			{choices, "e4", false}, {choices, "e6", false}, {choices, "plain", true},
		} {
			sym, ok := c.owner.Scope.LookupLocal(c.name)
			if !ok {
				t.Fatalf("%s::%s not found", c.owner.Name, c.name)
			}
			if got := fixture.model.UsageMayTimeVary(sym); got != c.want {
				t.Errorf("fixture %d: %s::%s mayTimeVary = %v, want %v", i, c.owner.Name, c.name, got, c.want)
			}
			if got := fixture.model.UsageMayTimeVaryForOwningType(sym, c.owner); got != c.want {
				t.Errorf("fixture %d: %s::%s mayTimeVary for owning type = %v, want %v", i, c.owner.Name, c.name, got, c.want)
			}
			if got := fixture.model.FeatureIsVariable(sym); got != c.want {
				t.Errorf("fixture %d: %s::%s isVariable = %v, want %v", i, c.owner.Name, c.name, got, c.want)
			}
		}
	}
}

// A KerML body parameter is a KerML Feature: Usage::mayTimeVary is not its to derive.
func TestKerMLBodyParameterDoesNotVaryInTime(t *testing.T) {
	const src = `package P {
		attribute values : Integer[*];
		attribute positives = values->select { in item : Integer; item > 0 };
	}`
	m, root := buildModelWithStdlibNamedKind(t, "body.kerml", source.KindKerML, src)
	param := bodyParameter(t, root, "item")
	if m.UsageMayTimeVary(param) {
		t.Error("KerML body parameter item: mayTimeVary = true, want false")
	}
	if m.FeatureIsVariable(param) {
		t.Error("KerML body parameter item: isVariable = true, want false")
	}
}

// A usage that declares nothing of its own varies in time under an Occurrence
// owner: a connection or action, not a succession (a Link), nor a KerML owner.
func TestUsageMayTimeVaryUnderOwner(t *testing.T) {
	const src = `package P {
		part def D {
			part a;
			part b;
			connection c connect a to b;
			succession s first a then b;
			attribute v : ScalarValues::Real;
		}
	}`
	m, root := buildModelWithLibraryNamedKind(t, "under.sysml", source.KindSysML, src)
	p, ok := root.LookupLocal("P")
	if !ok {
		t.Fatal("P not found")
	}
	d, ok := p.Scope.LookupLocal("D")
	if !ok {
		t.Fatal("P::D not found")
	}
	for name, want := range map[string]bool{"c": true, "s": false, "v": false} {
		sym, ok := d.Scope.LookupLocal(name)
		if !ok {
			t.Fatalf("P::D::%s not found", name)
		}
		if got := m.UsageMayTimeVaryUnder(sym); got != want {
			t.Errorf("under %s: mayTimeVary = %v, want %v", name, got, want)
		}
	}
	if !m.UsageMayTimeVaryUnderKind(ast.UsageAction) {
		t.Error("under an action usage: mayTimeVary = false, want true")
	}
	if m.UsageMayTimeVaryUnderKind(ast.UsageAttribute) {
		t.Error("under an attribute usage: mayTimeVary = true, want false")
	}
	if !m.UsageMayTimeVaryUnderLibraryFeature("Flows::Flow::source") {
		t.Error("under a flow end (Flows::Flow::source : Occurrence): mayTimeVary = false, want true")
	}
	if m.UsageMayTimeVaryUnder(nil) {
		t.Error("under no owner: mayTimeVary = true, want false")
	}
}

// buildModelWithLibraryNamedKind is buildModelWithStdlibNamedKind with the
// standard library marked as the library it is, so library-only lookups resolve.
func buildModelWithLibraryNamedKind(t *testing.T, name string, kind source.Kind, src string) (*Model, *symbols.Scope) {
	t.Helper()
	idx := stdlibIndex(t)
	for _, doc := range idx.Documents() {
		if doc != name {
			idx.MarkLibrary(doc)
		}
	}
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx.AddDocumentWithKind(name, root, kind)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	return m, idx.DocumentRoot(name)
}

func TestConnectorEndMayTimeVaryFromItsConnector(t *testing.T) {
	const src = `package P {
		part def D {
			part a;
			part b;
			connection c connect a to b;
			succession s first a then b;
		}
	}`
	m, root := buildModelWithLibraryNamedKind(t, "ends.sysml", source.KindSysML, src)
	p, ok := root.LookupLocal("P")
	if !ok {
		t.Fatal("P not found")
	}
	d, ok := p.Scope.LookupLocal("D")
	if !ok {
		t.Fatal("P::D not found")
	}
	// A connection is an Occurrence, so its ends vary in time; a succession is
	// no Occurrence, so its ends do not.
	for name, want := range map[string]bool{"c": true, "s": false} {
		connector, ok := d.Scope.LookupLocal(name)
		if !ok || connector.Scope == nil {
			t.Fatalf("P::D::%s not found", name)
		}
		ends := 0
		connector.Scope.ForEachMember(func(end *symbols.Symbol) bool {
			if end.Kind != symbols.SymbolConnectorEnd {
				return true
			}
			ends++
			if got := m.UsageMayTimeVary(end); got != want {
				t.Errorf("%s end %d: mayTimeVary = %v, want %v", name, ends, got, want)
			}
			if got := m.UsageMayTimeVaryForOwningType(end, connector); got != want {
				t.Errorf("%s end %d: mayTimeVary for owning type = %v, want %v", name, ends, got, want)
			}
			if got := m.FeatureIsVariable(end); got != want {
				t.Errorf("%s end %d: isVariable = %v, want %v", name, ends, got, want)
			}
			// An end that varies in time is constant (KerML 1.0 §8.3.3.3.4).
			if got := m.FeatureIsConstant(end); got != want {
				t.Errorf("%s end %d: isConstant = %v, want %v", name, ends, got, want)
			}
			if got, ok := m.ReflectiveFeatureValues(end, "isConstant"); !ok || len(got) != 1 || got[0].Bool != want {
				t.Errorf("%s end %d: reflective isConstant = %v, %v; want [%v]", name, ends, got, ok, want)
			}
			ro := m.FeatureReadOnly(end)
			if want && (ro.Kind != ReadOnlyConstant || !ro.ImpliedByEnd || ro.Declared != end) {
				t.Errorf("%s end %d: read-only = %+v, want constant implied by the end itself", name, ends, ro)
			}
			if !want && ro.Kind != 0 {
				t.Errorf("%s end %d: read-only = %+v, want writable", name, ends, ro)
			}
			return true
		})
		if ends != 2 {
			t.Errorf("%s: %d ends, want 2", name, ends)
		}
	}
}
