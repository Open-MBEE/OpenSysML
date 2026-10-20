package model

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// baseFeatureModel opens src under uri with the standard library and returns the
// semantic model with the symbol at path.
func baseFeatureModel(t *testing.T, uri, src string, path ...string) (*semantics.Model, *Workspace, *symbols.Symbol) {
	t.Helper()
	ws := NewWorkspace()
	ws.Open(uri, []byte(src), 1)
	t.Cleanup(func() { ws.Close(uri) })
	scope := ws.index.DocumentRoot(uri)
	var sym *symbols.Symbol
	for _, part := range path {
		if scope == nil {
			t.Fatalf("no scope while looking up %q", part)
		}
		s, ok := scope.LookupLocal(part)
		if !ok {
			t.Fatalf("symbol %q not found", part)
		}
		sym, scope = s, s.Scope
	}
	r := resolve.New(ws.index)
	m := semantics.NewModel(r)
	r.SetModel(m)
	return m, ws, sym
}

func libSymbol(t *testing.T, ws *Workspace, fqn string) *symbols.Symbol {
	t.Helper()
	for _, s := range ws.index.LookupQualified(fqn) {
		if s != nil {
			return s
		}
	}
	t.Fatalf("library symbol %s not found", fqn)
	return nil
}

func fqns(syms []*symbols.Symbol) []string {
	var out []string
	for _, s := range syms {
		out = append(out, symbols.FQNOf(s))
	}
	return out
}

// TestUsageSubsetsTheBaseUsageOfItsKind covers the specialization clause of each
// usage kind (SysML v2 §8.3.9.4 checkOccurrenceUsageSpecialization and kin, KerML
// §8.3.4.5.3 checkConnectorSpecialization): the usage directly or indirectly
// specializes the base library usage of its kind, an implied Subsetting beside
// the typing by the kind's base definition, so it conforms to the base usage
// while its types stay those of the definition.
func TestUsageSubsetsTheBaseUsageOfItsKind(t *testing.T) {
	const sysml = `package P {
		part def D;
		allocation u_allocation; case u_case; concern u_concern; connection u_connection;
		interface u_interface; item u_item; occurrence u_occurrence; part u_part;
		port u_port; rendering u_rendering; use case u_usecase; view u_view; viewpoint u_viewpoint;
		part typed : D;
		individual occurrence life;
		ref plain;
	}`
	const kerml = `package K {
		class Cl; assoc As { end a : Cl; end b : Cl; } assoc Tri { end a : Cl; end b : Cl; end c : Cl; }
		feature fc : Cl; connector cx : As from fc to fc; connector three : Tri (fc, fc, fc);
		feature f;
	}`
	for _, tc := range []struct {
		uri, src, pkg, name, base, typ string
	}{
		{"file:///base.sysml", sysml, "P", "u_allocation", "Allocations::allocations", "Allocations::Allocation"},
		{"file:///base.sysml", sysml, "P", "u_case", "Cases::cases", "Cases::Case"},
		{"file:///base.sysml", sysml, "P", "u_concern", "Requirements::concernChecks", "Requirements::ConcernCheck"},
		{"file:///base.sysml", sysml, "P", "u_connection", "Connections::connections", "Connections::Connection"},
		{"file:///base.sysml", sysml, "P", "u_interface", "Interfaces::interfaces", "Interfaces::Interface"},
		{"file:///base.sysml", sysml, "P", "u_item", "Items::items", "Items::Item"},
		{"file:///base.sysml", sysml, "P", "u_occurrence", "Occurrences::occurrences", "Occurrences::Occurrence"},
		{"file:///base.sysml", sysml, "P", "u_part", "Parts::parts", "Parts::Part"},
		{"file:///base.sysml", sysml, "P", "u_port", "Ports::ports", "Ports::Port"},
		{"file:///base.sysml", sysml, "P", "u_rendering", "Views::renderings", "Views::Rendering"},
		{"file:///base.sysml", sysml, "P", "u_usecase", "UseCases::useCases", "UseCases::UseCase"},
		{"file:///base.sysml", sysml, "P", "u_view", "Views::views", "Views::View"},
		{"file:///base.sysml", sysml, "P", "u_viewpoint", "Views::viewpointChecks", "Views::ViewpointCheck"},
		// A declared type displaces the base definition, not the base usage.
		{"file:///base.sysml", sysml, "P", "typed", "Parts::parts", "P::D"},
		// An individual occurrence is a Life and still an occurrence usage.
		{"file:///base.sysml", sysml, "P", "life", "Occurrences::occurrences", "Occurrences::Life"},
		// A usage of no kind subsets Base::things, the most general base usage.
		{"file:///base.sysml", sysml, "P", "plain", "Base::things", "Base::Anything"},
		{"file:///base.kerml", kerml, "K", "cx", "Links::binaryLinks", "K::As"},
		{"file:///base.kerml", kerml, "K", "three", "Links::links", "K::Tri"},
		{"file:///base.kerml", kerml, "K", "fc", "Occurrences::occurrences", "K::Cl"},
		{"file:///base.kerml", kerml, "K", "f", "Base::things", "Base::Anything"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, ws, sym := baseFeatureModel(t, tc.uri, tc.src, tc.pkg, tc.name)
			base := libSymbol(t, ws, tc.base)
			supers := m.DirectSupertypes(sym)
			if !slices.Contains(supers, base) {
				t.Errorf("DirectSupertypes(%s) = %v, want %s among them", tc.name, fqns(supers), tc.base)
			}
			if !m.Conforms(sym, base) {
				t.Errorf("%s does not conform to %s", tc.name, tc.base)
			}
			if !slices.Contains(m.ImplicitGenerals(sym), base) {
				t.Errorf("ImplicitGenerals(%s) = %v, want %s among them: the subsetting is implied", tc.name, fqns(m.ImplicitGenerals(sym)), tc.base)
			}
			if types := fqns(m.FeatureTypeSet(sym)); !slices.Equal(types, []string{tc.typ}) {
				t.Errorf("FeatureTypeSet(%s) = %v, want [%s]: the base usage is subsetted, not a type", tc.name, types, tc.typ)
			}
		})
	}
}

// TestKerMLTypeDeclarationsSubsetNoBaseFeature covers the KerML type
// declarations the parser records as usage nodes and the builder files under a
// definition kind — `datatype`, `function` — beside `class`: each specializes
// the base type of its kind and subsets no base feature (KerML 1.1 §8.3.2,
// §9.2.9), while a feature typed by a datatype subsets `Base::dataValues`
// (KerML 1.1 §8.4.2).
func TestKerMLTypeDeclarationsSubsetNoBaseFeature(t *testing.T) {
	const kerml = `package K {
		datatype D; function F; class C;
		feature fd : D; feature fc : C;
	}`
	for _, tc := range []struct {
		name, base, typ string
	}{
		{"D", "Base::DataValue", ""},
		{"F", "Performances::Evaluation", ""},
		{"C", "Occurrences::Occurrence", ""},
		{"fd", "Base::dataValues", "K::D"},
		{"fc", "Occurrences::occurrences", "K::C"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, ws, sym := baseFeatureModel(t, "file:///types.kerml", kerml, "K", tc.name)
			base := libSymbol(t, ws, tc.base)
			supers := m.DirectSupertypes(sym)
			if !slices.Contains(supers, base) {
				t.Errorf("DirectSupertypes(%s) = %v, want %s among them", tc.name, fqns(supers), tc.base)
			}
			if tc.typ == "" {
				for _, super := range supers {
					if super.IsFeature() {
						t.Errorf("DirectSupertypes(%s) = %v: a type subsets no feature", tc.name, fqns(supers))
					}
				}
				return
			}
			if types := fqns(m.FeatureTypeSet(sym)); !slices.Equal(types, []string{tc.typ}) {
				t.Errorf("FeatureTypeSet(%s) = %v, want [%s]", tc.name, types, tc.typ)
			}
		})
	}
}
