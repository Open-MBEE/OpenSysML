package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestImplicitBaseNeedsTheLibrary covers a model resolved without the standard
// library: no base is in the index, so an untyped usage stays untyped rather
// than reporting a phantom supertype.
func TestImplicitBaseWithoutLibrary(t *testing.T) {
	m, root := buildModel(t, "part p;")
	if supers := m.DirectSupertypes(sym(t, root, "p")); len(supers) != 0 {
		t.Fatalf("DirectSupertypes(p) = %v, want none without the library", supers)
	}
}

// TestImplicitBaseResolvesThroughIndex covers the lookup itself against a
// stand-in library declared in the same document.
func TestImplicitBaseResolvesThroughIndex(t *testing.T) {
	m, root := buildModel(t, "package Parts { part def Part; } part p; part q : Parts::Part;")
	parts := sym(t, root, "Parts")
	part, _ := parts.Scope.LookupLocal("Part")

	supers := m.DirectSupertypes(sym(t, root, "p"))
	if len(supers) != 1 || supers[0] != part {
		t.Fatalf("DirectSupertypes(p) = %v, want [Parts::Part]", supers)
	}
	if supers := m.DirectSupertypes(sym(t, root, "q")); len(supers) != 1 || supers[0] != part {
		t.Fatalf("DirectSupertypes(q) = %v, want the declared [Parts::Part] only", supers)
	}
}

// TestUsageKindsWithoutImplicitBase pins the kinds deliberately left out of the
// mapping: connector, succession, flow, binding, satisfy, subject and objective
// take their type from the element they relate to, and the KerML structural
// kinds are definitions in usage position.
func TestUsageKindsWithoutImplicitBase(t *testing.T) {
	for _, k := range []ast.UsageKind{
		ast.UsageConnector, ast.UsageSuccession, ast.UsageFlow, ast.UsageBinding,
		ast.UsageSatisfy, ast.UsageSubject, ast.UsageObjective, ast.UsageInteraction,
		ast.UsageBehavior, ast.UsageAssoc, ast.UsageStruct, ast.UsageClass,
		ast.UsagePredicate, ast.UsageBool,
	} {
		if fqn, ok := implicitUsageBases[k]; ok {
			t.Errorf("usage kind %v unexpectedly has implicit base %q", k, fqn)
		}
	}
}

// TestImplicitBaseUsageContributesThat covers SysML v2 §7.6: every usage element
// subsets the most general base usage, which is what makes `that` visible in a
// usage body. The base usage contributes members only — it is not a supertype.
func TestImplicitBaseUsageContributesThat(t *testing.T) {
	m, root := buildModel(t, `package Base { abstract feature things { feature that; } }
		package Parts { part def Part; }
		part p : Parts::Part;`)
	p := sym(t, root, "p")

	if _, ok := m.LookupMember(p, "that"); !ok {
		t.Errorf("`that` is not a member of a usage")
	}
	parts := sym(t, root, "Parts")
	part, _ := parts.Scope.LookupLocal("Part")
	if supers := m.DirectSupertypes(p); len(supers) != 1 || supers[0] != part {
		t.Errorf("DirectSupertypes(p) = %v, want the declared [Parts::Part] only", supers)
	}
	base := sym(t, root, "Base")
	things, _ := base.Scope.LookupLocal("things")
	if m.Conforms(p, things) {
		t.Errorf("a usage reported conforming to the base usage")
	}
	// A definition is not a usage element and takes nothing from the base usage.
	if _, ok := m.LookupMember(part, "that"); ok {
		t.Errorf("`that` reported as a member of a definition")
	}
}

func TestKerMLImplicitDefinitionBases(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class C;
		behavior B;
		datatype D;
		feature F;
	}
	package Base { classifier Anything; datatype DataValue; }
	package Occurrences { classifier Occurrence; }
	package Objects { classifier Object; }
	package Links { classifier Link; }
	package Performances {
		classifier Performance;
		classifier Evaluation;
		classifier BooleanEvaluation;
	}
	package Transfers { classifier Transfer; }
	package Metaobjects { classifier Metaobject; }`)
	p := sym(t, root, "P")
	for _, tc := range []struct {
		name string
		want string
	}{
		{"C", "Occurrences::Occurrence"},
		{"B", "Performances::Performance"},
		{"D", "Base::DataValue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := sym(t, p.Scope, tc.name)
			got := m.DirectSupertypes(target)
			if len(got) != 1 || m.resolver.Index().GetFQN(got[0]) != tc.want {
				t.Fatalf("supertypes of %s = %v, want [%s]", tc.name, got, tc.want)
			}
		})
	}
	if got := m.DirectSupertypes(sym(t, p.Scope, "F")); len(got) != 0 {
		t.Fatalf("KerML feature received an implicit datatype base: %v", got)
	}
}

func TestKerMLImplicitDefinitionBasesUseRecordedDocumentKind(t *testing.T) {
	content := `package P {
		class C;
		struct S;
	}
	package Occurrences { classifier Occurrence; }
	package Objects { classifier Object; }`
	want := map[string]string{
		"C": "Occurrences::Occurrence",
		"S": "Objects::Object",
	}
	for _, tc := range []struct {
		name string
		kind source.Kind
	}{
		{"inline-content", source.KindKerML},
		{"model.kerml", source.KindKerML},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, root := buildModelNamedWithKind(t, tc.name, tc.kind, content)
			p := sym(t, root, "P")
			for name, wantFQN := range want {
				got := m.DirectSupertypes(sym(t, p.Scope, name))
				if len(got) != 1 || m.resolver.Index().GetFQN(got[0]) != wantFQN {
					t.Fatalf("supertypes of %s = %v, want [%s]", name, got, wantFQN)
				}
			}
		})
	}
}

func TestSysMLImplicitDefinitionBasesRemainKindBased(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package P {
		part def C;
		action def A;
		attribute def D;
	}
	package Parts { part def Part; }
	package Actions { action def Action; }
	package Base { attribute def DataValue; }`)
	p := sym(t, root, "P")
	for _, tc := range []struct {
		name string
		want string
	}{
		{"C", "Parts::Part"},
		{"A", "Actions::Action"},
		{"D", "Base::DataValue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := sym(t, p.Scope, tc.name)
			got := m.DirectSupertypes(target)
			if len(got) != 1 || m.resolver.Index().GetFQN(got[0]) != tc.want {
				t.Fatalf("supertypes of %s = %v, want [%s]", tc.name, got, tc.want)
			}
		})
	}
}

// An interaction is an association and a behavior, so it implicitly specializes
// Performance as well as Link — BinaryLink when it has two ends (KerML 7.4.10.2).
func TestKerMLInteractionImplicitBases(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class T;
		interaction Zero;
		interaction Two { end a : T; end b : T; }
		interaction Three { end a : T; end b : T; end c : T; }
		interaction DeclaredBehavior specializes Performances::Performance { end a : T; end b : T; }
		interaction DeclaredLink specializes Links::Link;
		interaction Derived specializes Two { end a : T; end b : T; }
	}
	package Links { classifier Link; classifier BinaryLink specializes Link; }
	package Performances { classifier Performance; }`)
	p := sym(t, root, "P")
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"Zero", []string{"Links::Link", "Performances::Performance"}},
		{"Two", []string{"Links::BinaryLink", "Performances::Performance"}},
		{"Three", []string{"Links::Link", "Performances::Performance"}},
		{"DeclaredBehavior", []string{"Performances::Performance", "Links::BinaryLink"}},
		{"DeclaredLink", []string{"Links::Link", "Performances::Performance"}},
		{"Derived", []string{"P::Two"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, super := range m.DirectSupertypes(sym(t, p.Scope, tc.name)) {
				got = append(got, m.resolver.Index().GetFQN(super))
			}
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("supertypes of %s = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestImplicitBaseOfExplicitSupertypeIsTransitive(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package Occurrences {
		class Occurrence { feature endShot; }
	}
	package Objects {
		struct Object specializes Occurrences::Occurrence;
	}
	struct Wheel;
	struct MyWheel specializes Wheel {
		feature redefines endShot : EndShot;
	}
	struct EndShot;`)
	myWheel := sym(t, root, "MyWheel")
	supers := m.AllSupertypes(myWheel)
	if len(supers) < 3 {
		var fqns []string
		for _, super := range supers {
			fqns = append(fqns, m.resolver.Index().GetFQN(super))
		}
		t.Fatalf("AllSupertypes(MyWheel) = %v, want Wheel, Objects::Object, and Occurrences::Occurrence", fqns)
	}
	if inherited, ok := m.LookupContributedMember(myWheel, "endShot"); !ok {
		t.Fatal("MyWheel does not inherit endShot through Wheel's implicit base")
	} else if got := m.resolver.Index().GetFQN(inherited); got != "Occurrences::Occurrence::endShot" {
		t.Fatalf("inherited endShot = %q, want Occurrences::Occurrence::endShot", got)
	}
}

func TestSysMLImplicitBaseOfExplicitSupertypeIsTransitive(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package Parts {
		part def Part { feature endShot; }
	}
	part def Wheel;
	part def MyWheel specializes Wheel {
		feature redefines endShot;
	}`)
	myWheel := sym(t, root, "MyWheel")
	if inherited, ok := m.LookupContributedMember(myWheel, "endShot"); !ok {
		t.Fatal("MyWheel does not inherit endShot through Wheel's implicit base")
	} else if got := m.resolver.Index().GetFQN(inherited); got != "Parts::Part::endShot" {
		t.Fatalf("inherited endShot = %q, want Parts::Part::endShot", got)
	}
}

// TestW7ATransitionMemberImplicitBase covers SysML v2 §7.19.2: a transition
// written in a state body is a TransitionUsage, so it inherits TransitionAction's
// accepter, effect, acceptedMessage and receiver.
func TestW7ATransitionMemberImplicitBase(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package Actions {
		action def Action;
		action def TransitionAction :> Action {
			ref acceptedMessage;
			ref receiver;
			action accepter;
			action effect;
		}
	}
	package States { action def StateAction; }
	package P {
		state machine {
			state wait;
			transition subscribing first wait then wait;
		}
	}`)
	p := sym(t, root, "P")
	machine := sym(t, p.Scope, "machine")
	trans, ok := machine.Scope.LookupLocal("subscribing")
	if !ok {
		t.Fatal("the state body declares no subscribing transition")
	}
	for _, name := range []string{"accepter", "effect", "acceptedMessage", "receiver"} {
		if _, ok := m.LookupMember(trans, name); !ok {
			t.Errorf("transition does not inherit %s from Actions::TransitionAction", name)
		}
	}
}

// TestW7AKerMLFeatureBaseContributesMembers covers KerML §8.4.2: a feature of a
// kind subsets that kind's base feature, which contributes members only.
func TestW7AKerMLFeatureBaseContributesMembers(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package Base { abstract feature things; }
	package Performances {
		abstract step performances { feature enclosingPerformance; }
		classifier Performance;
	}
	package P { step s; }`)
	s := sym(t, sym(t, root, "P").Scope, "s")
	if _, ok := m.LookupMember(s, "enclosingPerformance"); !ok {
		t.Errorf("a KerML step does not inherit members of Performances::performances")
	}
	performances, _ := sym(t, root, "Performances").Scope.LookupLocal("performances")
	if m.Conforms(s, performances) {
		t.Errorf("a feature reported conforming to its base feature")
	}
}

// TestW7AKerMLFeatureBaseSuppressedWhenDeclared covers the §8.4.2 suppression
// rule: a declared subsetting that already reaches the base suppresses it.
func TestW7AKerMLFeatureBaseSuppressedWhenDeclared(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package Performances {
		abstract step performances;
		step nested subsets performances;
	}
	package P {
		step direct subsets Performances::performances;
		step indirect subsets Performances::nested;
		step plain;
	}`)
	p := sym(t, root, "P").Scope
	for _, name := range []string{"direct", "indirect"} {
		if m.implicitKerMLFeatureBase(sym(t, p, name)) != nil {
			t.Errorf("%s: declared subsetting did not suppress the implicit feature base", name)
		}
	}
	if m.implicitKerMLFeatureBase(sym(t, p, "plain")) == nil {
		t.Errorf("plain step lost its implicit feature base")
	}
}

// TestW7ASysMLSuppressionMatchesKerML covers the same rule in SysML: a declared
// generalization that does not reach the kind's base does not suppress it.
func TestW7ASysMLSuppressionMatchesKerML(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package Parts { part def Part { feature endShot; } }
	package Frames { attribute def Frame; }
	part p :> Frames::Frame;
	part q :> Parts::Part;`)
	part, _ := sym(t, root, "Parts").Scope.LookupLocal("Part")
	if _, ok := m.LookupContributedMember(sym(t, root, "p"), "endShot"); !ok {
		t.Errorf("a part usage specializing a non-part lost Parts::Part")
	}
	supers := m.DirectSupertypes(sym(t, root, "q"))
	if len(supers) != 1 || supers[0] != part {
		t.Errorf("DirectSupertypes(q) = %v, want the declared [Parts::Part] only", supers)
	}
}

// A KerML feature's implicit base follows the kind of its declared type, not its
// own keyword: `feature b : A` with A a class subsets Occurrences::occurrences,
// so it contributes nothing while Occurrences is absent, while an untyped
// feature still subsets Base::things (KerML §8.4.2).
func TestImplicitKerMLFeatureBaseFollowsTheDeclaredTypeKind(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package Base { abstract feature things { feature that; } }
		package P { class A; feature b : A; feature c; }`)
	p := sym(t, root, "P")
	things, _ := sym(t, root, "Base").Scope.LookupLocal("things")

	b, _ := p.Scope.LookupLocal("b")
	for _, src := range m.DirectMemberSources(b) {
		if src == things {
			t.Errorf("b : A (a class) subsets %v, want no base while Occurrences is absent", src.Name)
		}
	}
	c, _ := p.Scope.LookupLocal("c")
	found := false
	for _, src := range m.DirectMemberSources(c) {
		found = found || src == things
	}
	if !found {
		t.Errorf("untyped feature c does not subset Base::things; sources %v", m.DirectMemberSources(c))
	}
}

// An `assoc struct` is an AssociationStructure, so its implicit base is
// Objects::LinkObject (Objects::BinaryLinkObject with two ends) rather than a
// plain association's Links::Link / Links::BinaryLink (KerML §8.4.4).
func TestImplicitKerMLAssocStructBaseIsLinkObject(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package Objects { struct Object; struct LinkObject :> Object; struct BinaryLinkObject :> LinkObject; }
		package Links { assoc Link; assoc BinaryLink :> Link; }
		package P {
			assoc struct S { end feature a; end feature b; }
			assoc struct S3 { end feature a; end feature b; end feature c; }
			assoc A { end feature a; end feature b; }
			assoc A3 { end feature a; end feature b; end feature c; }
		}`)
	p := sym(t, root, "P")
	objects := sym(t, root, "Objects").Scope
	links := sym(t, root, "Links").Scope
	for name, base := range map[string]struct {
		scope *symbols.Scope
		name  string
	}{
		"S":  {objects, "BinaryLinkObject"},
		"S3": {objects, "LinkObject"},
		"A":  {links, "BinaryLink"},
		"A3": {links, "Link"},
	} {
		want, _ := base.scope.LookupLocal(base.name)
		s, _ := p.Scope.LookupLocal(name)
		if supers := m.DirectSupertypes(s); len(supers) != 1 || supers[0] != want {
			t.Errorf("DirectSupertypes(%s) = %v, want [%s]", name, supers, base.name)
		}
	}
}

// A control node is an action usage of its control action, so its members
// are the action's (SysML v2 §8.3.17) and the variability rules see an occurrence.
func TestControlNodeBaseIsItsControlAction(t *testing.T) {
	m, root := buildModelNamed(t, "t.sysml", `package Actions { action def Action; action def ForkAction :> Action; action def JoinAction :> Action; action def MergeAction :> Action; action def DecisionAction :> Action; }
		action def A { fork f; join j; merge m; decide d; }`)
	actions := sym(t, root, "Actions").Scope
	a := sym(t, root, "A").Scope
	for node, base := range map[string]string{"f": "ForkAction", "j": "JoinAction", "m": "MergeAction", "d": "DecisionAction"} {
		want, _ := actions.LookupLocal(base)
		n, _ := a.LookupLocal(node)
		if supers := m.DirectSupertypes(n); len(supers) != 1 || supers[0] != want {
			t.Errorf("DirectSupertypes(%s) = %v, want [Actions::%s]", node, supers, base)
		}
	}
}

// A binary base follows the effective ends of a KerML association or connector
// (KerML 1.1 checkAssociationBinarySpecialization, checkConnectorBinarySpecialization):
// two owned ends redefining two of a general's three leave it n-ary, while two
// owned ends over two binary generals mask both pairs by position and stay binary
// (an association then reaches BinaryLink through its generals and adds no base).
func TestKerMLBinaryBaseFollowsEffectiveEnds(t *testing.T) {
	m, root := buildModelNamed(t, "t.kerml", `package P {
		class T;
		assoc Three { end a : T; end b : T; end c : T; }
		assoc TwoOfThree specializes Three { end redefines a : T; end redefines b : T; }
		assoc ByPosition specializes Three { end x : T; end y : T; }
		assoc Two { end a : T; end b : T; }
		assoc TwoOfTwo specializes Two { end redefines a : T; end redefines b : T; }
		assoc OneOfTwo specializes Two { end redefines a : T; }
		assoc A { end source : T; end target : T; }
		assoc B { end source : T; end target : T; }
		assoc OfA specializes A, B { end redefines A::source : T; end redefines A::target : T; }
		assoc OfBoth specializes A, B { end redefines A::source : T; end redefines B::target : T; }
		class C {
			feature x : T; feature y : T; feature z : T;
			connector ofA : A, B { end redefines A::source references x; end redefines A::target references y; }
			connector three : Three (x, y, z);
			connector twoOfThree : Three { end redefines a references x; end redefines b references y; }
			connector twoOfInherited subsets three { end redefines a references x; end redefines b references y; }
			connector two (x, y);
			connector twoOfTwo subsets two { end redefines source references x; end redefines target references y; }
		}
	}
	package Links {
		abstract assoc Link;
		assoc BinaryLink specializes Link { end source : T; end target : T; }
		abstract feature links : Link;
		abstract feature binaryLinks : BinaryLink subsets links;
	}`)
	p := sym(t, root, "P")
	c := sym(t, p.Scope, "C")
	for _, tc := range []struct {
		scope *symbols.Scope
		name  string
		want  []string
	}{
		{p.Scope, "Three", []string{"Links::Link"}},
		{p.Scope, "TwoOfThree", []string{"P::Three"}},
		{p.Scope, "ByPosition", []string{"P::Three"}},
		{p.Scope, "Two", []string{"Links::BinaryLink"}},
		{p.Scope, "TwoOfTwo", []string{"P::Two"}},
		{p.Scope, "OneOfTwo", []string{"P::Two"}},
		{p.Scope, "OfA", []string{"P::A", "P::B"}},
		{p.Scope, "OfBoth", []string{"P::A", "P::B"}},
		{c.Scope, "ofA", []string{"P::A", "P::B", "Links::binaryLinks"}},
		{c.Scope, "three", []string{"P::Three", "Links::links"}},
		{c.Scope, "twoOfThree", []string{"P::Three", "Links::links"}},
		{c.Scope, "twoOfInherited", []string{"P::C::three", "Links::links"}},
		{c.Scope, "two", []string{"Links::binaryLinks"}},
		{c.Scope, "twoOfTwo", []string{"P::C::two", "Links::binaryLinks"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sym(t, tc.scope, tc.name)
			var got []string
			for _, super := range m.DirectSupertypes(s) {
				got = append(got, m.resolver.Index().GetFQN(super))
			}
			if base := m.implicitKerMLFeatureBase(s); base != nil {
				got = append(got, m.resolver.Index().GetFQN(base))
			}
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("supertypes of %s = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
