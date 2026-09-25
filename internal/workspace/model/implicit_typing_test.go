package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// implicitBaseOf opens src in a stdlib-loaded workspace and returns the
// qualified names of the direct supertypes the semantic model reports for the
// symbol reached by walking path from the document root.
func implicitBaseOf(t *testing.T, src string, path ...string) []string {
	t.Helper()
	const uri = "file:///implicit.sysml"
	ws := NewWorkspace()
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)

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
	var names []string
	for _, sup := range m.DirectSupertypes(sym) {
		names = append(names, symbols.FQNOf(sup))
	}
	return names
}

func implicitGeneralNamesOf(t *testing.T, src string, path ...string) []string {
	t.Helper()
	const uri = "file:///implicit-generals.sysml"
	ws := NewWorkspace()
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)

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
	var names []string
	for _, general := range m.ImplicitGenerals(sym) {
		names = append(names, symbols.FQNOf(general))
	}
	return names
}

func featureBaseNameOf(t *testing.T, src string, path ...string) string {
	t.Helper()
	const uri = "file:///feature-base.sysml"
	ws := NewWorkspace()
	ws.Open(uri, []byte(src), 1)
	defer ws.Close(uri)

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
	fqn, ok := m.FeatureBaseFQN(sym)
	if !ok {
		return ""
	}
	return fqn
}

// TestImplicitUsageBaseTypes covers the standard library definition each kind of
// untyped usage is implicitly typed by, so members inherited from it resolve.
func TestImplicitUsageBaseTypes(t *testing.T) {
	cases := []struct {
		decl string
		want string
		base string
	}{
		{"part x;", "Parts::Part", "Parts::parts"},
		{"individual part x;", "Parts::Part", "Parts::parts"},
		{"individual item x;", "Items::Item", "Items::items"},
		{"attribute x;", "Base::DataValue", "Base::dataValues"},
		{"item x;", "Items::Item", "Items::items"},
		{"occurrence x;", "Occurrences::Occurrence", "Occurrences::occurrences"},
		{"individual occurrence x;", "Occurrences::Life", "Occurrences::Life"},
		{"port x;", "Ports::Port", "Ports::ports"},
		{"connection x;", "Connections::Connection", "Connections::connections"},
		{"interface x;", "Interfaces::Interface", "Interfaces::interfaces"},
		{"allocation x;", "Allocations::Allocation", "Allocations::allocations"},
		{"action x;", "Actions::Action", "Actions::actions"},
		{"state x;", "States::StateAction", "States::stateActions"},
		{"calc x;", "Calculations::Calculation", "Calculations::calculations"},
		{"constraint x;", "Constraints::ConstraintCheck", "Constraints::constraintChecks"},
		{"requirement x;", "Requirements::RequirementCheck", "Requirements::requirementChecks"},
		{"concern x;", "Requirements::ConcernCheck", "Requirements::concernChecks"},
		{"case x;", "Cases::Case", "Cases::cases"},
		{"analysis x;", "AnalysisCases::AnalysisCase", "AnalysisCases::analysisCases"},
		{"verification x;", "VerificationCases::VerificationCase", "VerificationCases::verificationCases"},
		{"use case x;", "UseCases::UseCase", "UseCases::useCases"},
		{"view x;", "Views::View", "Views::views"},
		{"viewpoint x;", "Views::ViewpointCheck", "Views::viewpointChecks"},
		{"rendering x;", "Views::Rendering", "Views::renderings"},
		// No `metadata x;` row: the grammar reads x as the usage's typing, not
		// its name (SysML.xtext MetadataUsageDeclaration).
	}

	for _, tc := range cases {
		t.Run(tc.decl, func(t *testing.T) {
			got := implicitBaseOf(t, "package P { "+tc.decl+" }", "P", "x")
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("supertypes of %q = %v, want [%s]", tc.decl, got, tc.want)
			}
			generals := implicitGeneralNamesOf(t, "package P { "+tc.decl+" }", "P", "x")
			found := false
			for _, general := range generals {
				if general == tc.base {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("implicit generals of %q = %v, want %s", tc.decl, generals, tc.base)
			}
		})
	}
}

func TestBinaryUsageBaseFeatures(t *testing.T) {
	const src = `package P {
		part def A { port p; }
		part def B { port q; }
		part a : A;
		part b : B;
		connection c connect a.p to b.q;
		interface i connect a.p to b.q;
	}`
	for _, tc := range []struct {
		name string
		path string
		base string
	}{
		{"connection", "c", "Connections::binaryConnections"},
		{"interface", "i", "Interfaces::binaryInterfaces"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			generals := implicitGeneralNamesOf(t, src, "P", tc.path)
			found := false
			for _, general := range generals {
				if general == tc.base {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("implicit generals = %v, want %s", generals, tc.base)
			}
			if got := featureBaseNameOf(t, src, "P", tc.path); got != tc.base {
				t.Fatalf("FeatureBaseFQN = %q, want %q", got, tc.base)
			}
		})
	}
}

func TestConjugatedUsageHasNoImplicitBaseFeature(t *testing.T) {
	const src = `package P {
		port def PortDef;
		part def A { port p : ~PortDef; }
	}`
	generals := implicitGeneralNamesOf(t, src, "P", "A", "p")
	for _, general := range generals {
		if general == "Ports::ports" {
			t.Fatalf("implicit generals = %v, must not contain Ports::ports", generals)
		}
	}
}

// TestImplicitBaseNotAppliedToTypedUsage covers the negative cases: a usage that
// declares its own type or specialization keeps exactly that supertype, and a
// definition never gets an implicit usage base.
func TestImplicitBaseNotAppliedToTypedUsage(t *testing.T) {
	cases := []struct {
		name string
		src  string
		path []string
		want string
	}{
		{"typed usage", "package P { part def Engine; part x : Engine; }", []string{"P", "x"}, "P::Engine"},
		{"subsetting usage", "package P { part y; part x subsets y; }", []string{"P", "x"}, "P::y"},
		{"specializing usage", "package P { part y; part x :> y; }", []string{"P", "x"}, "P::y"},
		{"definition", "package P { part def Engine :> Parts::Part; }", []string{"P", "Engine"}, "Parts::Part"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := implicitBaseOf(t, tc.src, tc.path...)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("supertypes = %v, want [%s]", got, tc.want)
			}
		})
	}
}

func TestUsageBaseFeatureSuppressionFollowsDeclaredChain(t *testing.T) {
	typed := implicitGeneralNamesOf(t, `package P {
		part def Vehicle;
		part p : Vehicle;
	}`, "P", "p")
	found := false
	for _, name := range typed {
		if name == "Parts::parts" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("implicit generals of typed usage = %v, want Parts::parts", typed)
	}

	subsetted := implicitGeneralNamesOf(t, `package P {
		part q;
		part p :> q;
	}`, "P", "p")
	for _, name := range subsetted {
		if name == "Parts::parts" {
			t.Fatalf("implicit generals of usage subsetting a part = %v, want no Parts::parts", subsetted)
		}
	}
}

// TestParameterRedefinitionAccompaniesTheImplicitBase covers a parameter of a
// step: it implicitly redefines the parameter at its position in the behavior
// that types the step (KerML 7.4.7.3), and that parameter supplies the type.
// The kind's standard library base is an independent rule, so it still applies
// — the redefined parameter may itself be untyped.
func TestParameterRedefinitionAccompaniesTheImplicitBase(t *testing.T) {
	src := `package P {
		part def Image;
		action def Focus { in scene; out image : Image; }
		action focus : Focus { in item scene; out item image; }
	}`
	got := implicitBaseOf(t, src, "P", "focus", "image")
	if len(got) != 2 || got[0] != "P::Focus::image" || got[1] != "Items::Item" {
		t.Fatalf("supertypes = %v, want [P::Focus::image Items::Item] (the redefined parameter of Focus, then the kind's base)", got)
	}
}

// TestParameterOfAnUntypedParameterKeepsItsImplicitBase covers the case where
// the redefined parameter carries no type of its own: the redefining parameter
// still gets the standard library base of its kind, so the redefinition never
// costs it a type.
func TestParameterOfAnUntypedParameterKeepsItsImplicitBase(t *testing.T) {
	src := `package P {
		action def Focus { in scene; }
		action focus : Focus { in item lighting; }
	}`
	got := implicitBaseOf(t, src, "P", "focus", "lighting")
	if len(got) != 2 || got[0] != "P::Focus::scene" || got[1] != "Items::Item" {
		t.Fatalf("supertypes = %v, want [P::Focus::scene Items::Item]", got)
	}
}

// TestLikeNamedUsageIsNotAnImplicitRedefinition covers the case the parameter
// rule does not extend to: an undirected nested usage that happens to have the
// same name as a feature its owner inherits. SysML v2 7.6.1 makes that a name
// conflict to be resolved by an explicit redefinition, not an implicit
// redefinition, so the usage keeps the standard library base of its kind.
func TestLikeNamedUsageIsNotAnImplicitRedefinition(t *testing.T) {
	src := `package P {
		part def Engine;
		part def Vehicle { part engine : Engine; }
		part v : Vehicle { part engine; }
	}`
	if got := implicitBaseOf(t, src, "P", "v", "engine"); len(got) != 1 || got[0] != "Parts::Part" {
		t.Fatalf("supertypes = %v, want [Parts::Part]", got)
	}
}

// TestImplicitRedefinitionSuppliesInheritedMembers covers the user-visible
// effect on the OMG training model "Conditional Succession Example-1": the
// output parameter of a subaction is typed by the parameter it redefines, so
// members of that type resolve.
func TestImplicitRedefinitionSuppliesInheritedMembers(t *testing.T) {
	good := `package P {
		part def Scene;
		part def Image { isWellFocused : ScalarValues::Boolean; }
		action def Focus { in scene : Scene; out image : Image; }
		action takePicture {
			action focus : Focus {
				in item scene;
				out item image;
			}
			constraint { focus.image.isWellFocused }
		}
	}`
	if found := diagnose(t, "implicit_redef_ok", good); len(found) != 0 {
		t.Fatalf("expected no findings, got %v", found)
	}

	bad := `package P {
		part def Scene;
		part def Image { isWellFocused : ScalarValues::Boolean; }
		action def Focus { in scene : Scene; out image : Image; }
		action takePicture {
			action focus : Focus {
				in item scene;
				out item image;
			}
			constraint { focus.image.notAMember }
		}
	}`
	if found := diagnose(t, "implicit_redef_bad", bad); len(found) != 1 {
		t.Fatalf("expected one finding for the undeclared member, got %d: %v", len(found), found)
	}
}

// TestInheritedMembersResolveThroughUntypedUsage covers the user-visible effect:
// members inherited from the implicit base resolve through an untyped usage,
// while a name no base declares still reports.
func TestInheritedMembersResolveThroughUntypedUsage(t *testing.T) {
	good := []struct {
		name string
		src  string
	}{
		{"state done", `package P {
			state machine {
				state normal;
				constraint { Time::TimeOf(normal.done) > 0 }
			}
		}`},
		{"state with body", `package P {
			state machine {
				state normal { entry; }
				constraint { Time::TimeOf(normal.start) > 0 }
			}
		}`},
		{"action done", `package P {
			action a;
			action b { constraint { Time::TimeOf(a.done) > 0 } }
		}`},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			if found := diagnose(t, "implicit_ok", tc.src); len(found) != 0 {
				t.Fatalf("expected no findings, got %v", found)
			}
		})
	}

	bad := `package P {
		state machine {
			state normal;
			constraint { Time::TimeOf(normal.notAMember) > 0 }
		}
	}`
	if found := diagnose(t, "implicit_bad", bad); len(found) != 1 {
		t.Fatalf("expected one finding for the undeclared member, got %d: %v", len(found), found)
	}
}
