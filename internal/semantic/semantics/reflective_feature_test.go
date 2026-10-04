package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

var reflectiveUsageFeatureFlags = []string{
	"isEnd", "isPortion", "isConstant", "isVariable", "isComposite",
	"isDerived", "isAbstract", "isOrdered", "isUnique", "isReference",
}

func assertReflectiveFeatureFlags(t *testing.T, m *Model, sym *symbols.Symbol) {
	t.Helper()
	for _, feature := range reflectiveUsageFeatureFlags {
		_, ok := m.ReflectiveFeatureValue(sym, feature)
		if !ok {
			t.Errorf("%s.%s is underived", symbols.FQNOf(sym), feature)
		}
	}
}

func TestReflectiveVariableEndUsagesAreImplicitlyConstant(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		connection def C { end part a; }
		attribute def A { end part nonOccurrence; }
	}`)
	variableEnd := nestedSym(t, root, "P::C::a")
	for name, want := range map[string]bool{
		"isEnd": true, "isVariable": true, "isConstant": true,
	} {
		got, ok := m.ReflectiveFeatureValue(variableEnd, name)
		if !ok || got.Bool != want {
			t.Errorf("P::C::a.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}

	nonOccurrenceEnd := nestedSym(t, root, "P::A::nonOccurrence")
	for name, want := range map[string]bool{
		"isEnd": true, "isVariable": false, "isConstant": false,
	} {
		got, ok := m.ReflectiveFeatureValue(nonOccurrenceEnd, name)
		if !ok || got.Bool != want {
			t.Errorf("P::A::nonOccurrence.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
}

func TestReflectiveAttributeFeaturesAreReferentialAndNonComposite(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		attribute def A { part definitionFeature; }
		attribute a : A { part usageFeature; }
	}`)
	for _, path := range []string{"P::A::definitionFeature", "P::a::usageFeature"} {
		feature := nestedSym(t, root, path)
		for name, want := range map[string]bool{"isComposite": false, "isReference": true} {
			got, ok := m.ReflectiveFeatureValue(feature, name)
			if !ok || got.Bool != want {
				t.Errorf("%s.%s = %v (present %t), want %t", path, name, got, ok, want)
			}
		}
	}
}

func TestReflectiveVariationsAreAbstract(t *testing.T) {
	const src = `package P {
		part def Base;
		variation part def Choices :> Base { variant part small : Base; }
		part def Owner {
			variation part choice : Base { variant part small : Base; }
		}
		enum def Color { red; green; }
	}`
	for _, fixture := range reflectiveFixtures(t, "variations.sysml", source.KindSysML, src) {
		for _, path := range []string{"P::Choices", "P::Owner::choice", "P::Color"} {
			sym := nestedSym(t, fixture.root, path)
			got, ok := fixture.model.ReflectiveFeatureValue(sym, "isAbstract")
			if !ok || got.Kind != symbols.FilterValueBool || !got.Bool {
				t.Errorf("%s.isAbstract = %v (present %t), want true", path, got, ok)
			}
		}
	}
}

func TestReflectiveConnectorRelatedFeaturesCoverEndForms(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		part def A {
			attribute x : Integer;
			attribute y : Integer;
		}
		part def Host {
			part a : A;
			part b : A;
			connection byPair connect (a, b);
			flow byFlow of Integer from a.x to b.y;
			message byMessage of Integer from a.x to b.y;
			binding byReferences {
				end feature references a;
				end feature references b;
			}
		}
		action def Sequence {
			action firstNode;
			action secondNode;
			succession explicit first firstNode then secondNode;
			first firstNode then secondNode {
				action nested;
			}
			first firstNode;
			then secondNode;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "connector-related.sysml", source.KindSysML, src) {
		for path, want := range map[string][]string{
			"P::Host::byPair":       {"P::Host::a", "P::Host::b"},
			"P::Host::byFlow":       {"P::A::x", "P::A::y"},
			"P::Host::byMessage":    {"P::A::x", "P::A::y"},
			"P::Host::byReferences": {"P::Host::a", "P::Host::b"},
			"P::Sequence::explicit": {"P::Sequence::firstNode", "P::Sequence::secondNode"},
		} {
			connector := nestedSym(t, fixture.root, path)
			assertReflectiveElements(t, fixture.model, connector, "relatedFeature", want...)
			assertReflectiveElements(t, fixture.model, connector, "sourceFeature", want[0])
			assertReflectiveElements(t, fixture.model, connector, "targetFeature", want[1:]...)
		}
		sequence := nestedSym(t, fixture.root, "P::Sequence")
		if sequence.Recorded() {
			continue
		}
		firstNode := nestedSym(t, fixture.root, "P::Sequence::firstNode")
		secondNode := nestedSym(t, fixture.root, "P::Sequence::secondNode")
		foundShorthand := false
		for _, succession := range fixture.model.ActionSuccessions(sequence) {
			switch succession.Decl.(type) {
			case *ast.InitialNode, *ast.SuccessionEdge:
			default:
				continue
			}
			source := succession.Source.Symbol
			if source == nil && succession.Source.Node != nil {
				source = memberSymbol(sequence.Scope, succession.Source.Node)
			}
			target := succession.Target.Symbol
			if target == nil && succession.Target.Node != nil {
				target = memberSymbol(sequence.Scope, succession.Target.Node)
			}
			if source == firstNode && target == secondNode {
				foundShorthand = true
				if edge := memberSymbol(sequence.Scope, succession.Decl); edge != nil {
					assertReflectiveElements(t, fixture.model, edge, "relatedFeature",
						"P::Sequence::firstNode", "P::Sequence::secondNode")
				}
			}
		}
		if !foundShorthand {
			t.Fatal("shorthand then succession does not resolve both connector ends")
		}
	}
}

func TestReflectiveRepeatedBindingRelatedFeaturesRemainBinaryWhenRecorded(t *testing.T) {
	const src = `package P {
		class C {
			feature a;
			binding repeated of a = a;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "repeated-binding.kerml", source.KindKerML, src) {
		connector := nestedSym(t, fixture.root, "P::C::repeated")
		feature := nestedSym(t, fixture.root, "P::C::a")
		related, ok := fixture.model.ReflectiveElements(connector, "relatedFeature")
		if !ok || len(related) != 2 || related[0] != feature || related[1] != feature {
			t.Errorf("%s.relatedFeature = %v (supported %t), want two references to %s",
				symbols.FQNOf(connector), related, ok, symbols.FQNOf(feature))
		}
		assertReflectiveElements(t, fixture.model, connector, "relatedFeature",
			"P::C::a", "P::C::a")
		assertReflectiveElements(t, fixture.model, connector, "sourceFeature", "P::C::a")
		assertReflectiveElements(t, fixture.model, connector, "targetFeature", "P::C::a")
	}
}

func TestReflectiveKerMLSuccessionRelatedFeatures(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		feature transitionLinkSource[0..1];
		feature trigger[1..*];
		feature triggerNum : Natural[1] = 1;
		succession triggerAfter [triggerNum]
			first [0..1] transitionLinkSource
			then [*] trigger;
	}`
	for _, fixture := range reflectiveFixtures(t, "succession.kerml", source.KindKerML, src) {
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::triggerAfter"),
			"relatedFeature", "P::transitionLinkSource", "P::trigger")
	}
}

func TestReflectiveMessageFlowAbstractWhenRelatedFeaturesAreMissing(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		private import Flows::Message;
		part def A {
			attribute x : Integer;
			attribute y : Integer;
		}
		part def Host {
			part a : A;
			part b : A;
			message incoming : Message[*];
			message connected of Integer from a.x to b.y;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "message-abstract.sysml", source.KindSysML, src) {
		for path, want := range map[string]bool{
			"P::Host::incoming":  true,
			"P::Host::connected": false,
		} {
			got, ok := fixture.model.ReflectiveFeatureValue(nestedSym(t, fixture.root, path), "isAbstract")
			if !ok || got.Kind != symbols.FilterValueBool || got.Bool != want {
				t.Errorf("%s.isAbstract = %v (present %t), want %t", path, got, ok, want)
			}
		}
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::Host::connected"),
			"relatedFeature", "P::A::x", "P::A::y")
	}
}

func TestReflectiveReferentialProjectionUsesExpectedFeaturingType(t *testing.T) {
	const src = `package P {
		part def Base;
		port def PD;
		action sequential;
		part def Owner {
			in part directed;
			end part endpoint;
			attribute attr;
			port p : PD;
			variation part options : Base {
				variant part option : Base;
			}
		}
		variation part orphan : Base {
			variant part orphanOption : Base;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "referential.sysml", source.KindSysML, src) {
		for path, want := range map[string]bool{
			"P::sequential":             false,
			"P::Owner::directed":        false,
			"P::Owner::endpoint":        false,
			"P::Owner::attr":            false,
			"P::Owner::p":               false,
			"P::orphan":                 false,
			"P::orphan::orphanOption":   false,
			"P::Owner::options":         true,
			"P::Owner::options::option": true,
		} {
			sym := nestedSym(t, fixture.root, path)
			for feature, wantValue := range map[string]bool{
				"isComposite": want,
				"isReference": !want,
			} {
				got, ok := fixture.model.ReflectiveFeatureValue(sym, feature)
				if !ok || got.Bool != wantValue {
					t.Errorf("%s.%s = %v (present %t), want %t", path, feature, got, ok, wantValue)
				}
			}
		}
	}
}

func TestReflectiveConnectionDefinitionsAreSufficient(t *testing.T) {
	const src = `package P {
		connection def C;
		interface def I;
		allocation def A;
		part def Ordinary;
	}`
	for _, fixture := range reflectiveFixtures(t, "sufficient.sysml", source.KindSysML, src) {
		for path, want := range map[string]bool{
			"P::C":        true,
			"P::I":        true,
			"P::A":        true,
			"P::Ordinary": false,
		} {
			sym := nestedSym(t, fixture.root, path)
			got, ok := fixture.model.ReflectiveFeatureValue(sym, "isSufficient")
			if !ok || got.Kind != symbols.FilterValueBool || got.Bool != want {
				t.Errorf("%s.isSufficient = %v (present %t), want %t", path, got, ok, want)
			}
		}
	}
	const kerml = `package P {
		class all Sufficient;
		class Ordinary;
	}`
	for _, fixture := range reflectiveFixtures(t, "sufficient.kerml", source.KindKerML, kerml) {
		for path, want := range map[string]bool{
			"P::Sufficient": true,
			"P::Ordinary":   false,
		} {
			sym := nestedSym(t, fixture.root, path)
			got, ok := fixture.model.ReflectiveFeatureValue(sym, "isSufficient")
			if !ok || got.Kind != symbols.FilterValueBool || got.Bool != want {
				t.Errorf("%s.isSufficient = %v (present %t), want %t", path, got, ok, want)
			}
		}
	}
}

func TestReflectiveFeatureFlagsForWrapperDeclarations(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		part def Owner { part child; ref part borrowed; }
		action def Flow {
			fork f; join j; merge m; decide d;
			first start;
		}
		requirement def Req {
			subject subject : Owner;
			assume constraint { true; }
			require constraint { true; }
		}
		metadata def Tag;
		action def Marked { @ annotation : Tag; }
		state machine {
			state running { state idle; }
			transition t first running accept after 1 [SI::s] then running;
		}
	}`)
	paths := []string{
		"P::Owner::child", "P::Owner::borrowed",
		"P::Flow::f", "P::Flow::j", "P::Flow::m", "P::Flow::d",
		"P::Flow::start",
		"P::Req::subject",
		"P::Marked::annotation",
		"P::machine::running", "P::machine::running::idle", "P::machine::t",
	}
	for _, path := range paths {
		feature := nestedSym(t, root, path)
		assertReflectiveFeatureFlags(t, m, feature)
	}
	for _, path := range []string{"P::Flow::f", "P::Flow::j", "P::Flow::m", "P::Flow::d"} {
		feature := nestedSym(t, root, path)
		got, ok := m.ReflectiveFeatureValue(feature, "isComposite")
		if !ok || !got.Bool {
			t.Errorf("%s.isComposite = %v (present %t), want true", path, got, ok)
		}
		if variation, ok := m.ReflectiveFeatureValue(feature, "isVariation"); !ok || variation.Bool {
			t.Errorf("%s.isVariation = %v (present %t), want present false", path, variation, ok)
		}
		if individual, ok := m.ReflectiveFeatureValue(feature, "isIndividual"); !ok || individual.Bool {
			t.Errorf("%s.isIndividual = %v (present %t), want present false", path, individual, ok)
		}
		if portion, ok := m.ReflectiveFeatureValue(feature, "portionKind"); !ok || portion.Kind != symbols.FilterValueEmpty {
			t.Errorf("%s.portionKind = %v (present %t), want present empty", path, portion, ok)
		}
	}
}

func TestReflectiveFeatureFlagsOnRecordedUsagesAndControlNodes(t *testing.T) {
	src := stdlibIndex(t)
	addTestDoc(t, src, "lib.sysml", `part def Owner {
		part child;
		ref part borrowed;
		constant attribute frozen;
	}
	connection def C { end part endpoint; }
	attribute def A { end part x; }
	action def Flow { fork f; }`)
	addTestDoc(t, src, "lib.kerml", `package K { class C { const feature f; } }`)
	srcRes := resolve.New(src)
	srcModel := NewModel(srcRes)
	srcRes.SetModel(srcModel)
	factsFor := func(sym *symbols.Symbol) symbols.LibraryFacts {
		facts := symbols.LibraryFacts{
			Node:    symbols.NodeKindOf(sym.Decl),
			Keyword: sym.Keyword(),
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok {
			facts.UsageKind = usage.Kind
			facts.Direction = usage.Direction
			if usage.IsEnd {
				facts.Modifiers |= symbols.ModEnd
			}
			if usage.IsPortion || usage.Portion != ast.PortionNone {
				facts.Modifiers |= symbols.ModPortion
			}
			if usage.IsConstant {
				facts.Modifiers |= symbols.ModConstant
			}
			if usage.IsVariable {
				facts.Modifiers |= symbols.ModVariable
			}
			if usage.IsReference {
				facts.Modifiers |= symbols.ModReference
			}
			if usage.IsComposite {
				facts.Modifiers |= symbols.ModComposite
			}
			if usage.IsDerived {
				facts.Modifiers |= symbols.ModDerived
			}
			if usage.IsOrdered {
				facts.Modifiers |= symbols.ModOrdered
			}
			if usage.IsNonunique {
				facts.Modifiers |= symbols.ModNonunique
			}
		}
		if srcModel.UsageMayTimeVary(sym) {
			facts.Modifiers |= symbols.ModMayTimeVary
		}
		return facts
	}
	record, err := symbols.RecordScope(src.DocumentRoot("lib.sysml"),
		func(*symbols.Symbol) bool { return true }, factsFor)
	if err != nil {
		t.Fatalf("RecordScope: %v", err)
	}
	recorded, err := symbols.BuildRecorded(record, "lib.sysml")
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	idx := stdlibIndex(t)
	idx.AddRecordedDocument("lib.sysml", source.KindSysML, recorded, nil)
	kermlRecord, err := symbols.RecordScope(src.DocumentRoot("lib.kerml"),
		func(*symbols.Symbol) bool { return true }, factsFor)
	if err != nil {
		t.Fatalf("RecordScope KerML: %v", err)
	}
	recordedKerML, err := symbols.BuildRecorded(kermlRecord, "lib.kerml")
	if err != nil {
		t.Fatalf("BuildRecorded KerML: %v", err)
	}
	idx.AddRecordedDocument("lib.kerml", source.KindKerML, recordedKerML, nil)
	res := resolve.New(idx)
	m := NewModel(res)
	res.SetModel(m)
	root := idx.DocumentRoot("lib.sysml")
	child := nestedSym(t, root, "Owner::child")
	borrowed := nestedSym(t, root, "Owner::borrowed")
	frozen := nestedSym(t, root, "Owner::frozen")
	endpoint := nestedSym(t, root, "C::endpoint")
	x := nestedSym(t, root, "A::x")
	fork := nestedSym(t, root, "Flow::f")
	kermlConst := nestedSym(t, idx.DocumentRoot("lib.kerml"), "K::C::f")
	if !child.Recorded() || !borrowed.Recorded() || !frozen.Recorded() || !fork.Recorded() {
		t.Fatal("recorded symbols lost their recorded status")
	}
	assertReflectiveFeatureFlags(t, m, child)
	assertReflectiveFeatureFlags(t, m, borrowed)
	assertReflectiveFeatureFlags(t, m, frozen)
	assertReflectiveFeatureFlags(t, m, endpoint)
	assertReflectiveFeatureFlags(t, m, x)
	assertReflectiveFeatureFlags(t, m, fork)
	for name, want := range map[string]bool{"isEnd": true, "isVariable": true, "isConstant": true} {
		got, ok := m.ReflectiveFeatureValue(endpoint, name)
		if !ok || got.Bool != want {
			t.Errorf("recorded endpoint.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	for name, want := range map[string]bool{"isVariable": true, "isConstant": false} {
		got, ok := m.ReflectiveFeatureValue(child, name)
		if !ok || got.Bool != want {
			t.Errorf("recorded child.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	for name, want := range map[string]bool{"isVariable": false, "isConstant": false} {
		got, ok := m.ReflectiveFeatureValue(x, name)
		if !ok || got.Bool != want {
			t.Errorf("recorded x.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	if got, ok := m.ReflectiveFeatureValue(kermlConst, "isVariable"); !ok || !got.Bool {
		t.Errorf("recorded KerML const isVariable = %v (present %t), want true", got, ok)
	}
	for name, want := range map[string]bool{"isComposite": false, "isReference": true} {
		got, ok := m.ReflectiveFeatureValue(borrowed, name)
		if !ok || got.Bool != want {
			t.Errorf("recorded borrowed.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	for name, want := range map[string]bool{"isConstant": true} {
		got, ok := m.ReflectiveFeatureValue(frozen, name)
		if !ok || got.Bool != want {
			t.Errorf("recorded frozen.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	got, ok := m.ReflectiveFeatureValue(fork, "isComposite")
	if !ok || !got.Bool {
		t.Errorf("recorded fork.isComposite = %v (present %t), want true", got, ok)
	}
	for name := range map[string]bool{"isVariation": false, "isIndividual": false} {
		value, ok := m.ReflectiveFeatureValue(fork, name)
		if !ok || value.Bool {
			t.Errorf("recorded fork.%s = %v (present %t), want present false", name, value, ok)
		}
	}
	if portion, ok := m.ReflectiveFeatureValue(fork, "portionKind"); !ok || portion.Kind != symbols.FilterValueEmpty {
		t.Errorf("recorded fork.portionKind = %v (present %t), want present empty", portion, ok)
	}
	if direction, ok := m.ReflectiveFeatureValue(fork, "direction"); !ok || direction.Kind != symbols.FilterValueEmpty {
		t.Errorf("recorded fork.direction = %v (present %t), want present empty", direction, ok)
	}
}

func TestReflectivePortUsageNestedFeaturesAndConnectorEnds(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		port def PD;
		port def OuterPD { port subport : PD; }
		part host {
			port p : PD { part nested; }
			interface i connect (e ::> p);
		}
	}`)
	port := nestedSym(t, root, "P::host::p")
	for name, want := range map[string]bool{"isComposite": false, "isReference": true} {
		got, ok := m.ReflectiveFeatureValue(port, name)
		if !ok || got.Bool != want {
			t.Errorf("P::host::p.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	subport := nestedSym(t, root, "P::OuterPD::subport")
	for name, want := range map[string]bool{"isComposite": true, "isReference": false} {
		got, ok := m.ReflectiveFeatureValue(subport, name)
		if !ok || got.Bool != want {
			t.Errorf("P::OuterPD::subport.%s = %v (present %t), want %t", name, got, ok, want)
		}
	}
	nested := nestedSym(t, root, "P::host::p::nested")
	assertReflectiveFeatureFlags(t, m, port)
	for name, want := range map[string]bool{"isComposite": true, "isReference": false} {
		got, ok := m.ReflectiveFeatureValue(nested, name)
		if !ok || got.Bool != want {
			t.Errorf("nested %s = %v (present %t), want %t", name, got, ok, want)
		}
	}

	connectorEnd := nestedSym(t, root, "P::host::i::e")
	if _, ok := connectorEnd.Decl.(*ast.ConnectorEnd); !ok {
		t.Fatalf("connector end declaration = %T, want *ast.ConnectorEnd", connectorEnd.Decl)
	}
	assertReflectiveFeatureFlags(t, m, connectorEnd)
	if direction, ok := m.ReflectiveFeatureValue(connectorEnd, "direction"); !ok || direction.Kind != symbols.FilterValueEmpty {
		t.Errorf("connector end direction = %v (present %t), want present empty", direction, ok)
	}
	if got := m.MetaclassOf(connectorEnd); got == nil || got.Name != "PortUsage" {
		t.Errorf("connector end metaclass = %v, want SysML::PortUsage", got)
	}
	nestedUsages, ok := m.reflectiveOwnedUsages(connectorEnd, "nestedUsage")
	if !ok || len(nestedUsages) != 0 {
		t.Errorf("connector-end nestedUsage = %v, %v; want a derived empty sequence", fqns(nestedUsages), ok)
	}
}

func TestReflectiveOwnershipAndFeaturingTypes(t *testing.T) {
	const src = `package P {
		part def A;
		part def B;
		part owner {
			part child : A;
			feature featured : A featured by B;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "reflective.sysml", source.KindSysML, src) {
		child := nestedSym(t, fixture.root, "P::owner::child")
		assertReflectiveElements(t, fixture.model, child, "owningType", "P::owner")
		assertReflectiveElements(t, fixture.model, child, "owningNamespace", "P::owner")
		assertReflectiveElements(t, fixture.model, child, "featuringType", "P::owner")
		featured := nestedSym(t, fixture.root, "P::owner::featured")
		assertReflectiveElements(t, fixture.model, featured, "featuringType", "P::owner", "P::B")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::A"), "owningNamespace", "P")
	}
}

func TestReflectiveParameterAndRoleFeatures(t *testing.T) {
	const src = `package P {
		part def SubjectType;
		action def Action {
			in inputValue : SubjectType;
			out outputValue : SubjectType;
		}
		requirement def Requirement {
			subject subject : SubjectType;
		}
		case def Case {
			subject subject : SubjectType;
			in argument : SubjectType;
		}
		case def CaseSubjectSecond {
			subject subject : SubjectType;
			in argument : SubjectType;
		}
		analysis def Analysis {
			objective objective : Requirement;
		}
		calc def Calculation { return result : SubjectType; }
	}`
	for _, fixture := range reflectiveFixtures(t, "reflective.sysml", source.KindSysML, src) {
		action := nestedSym(t, fixture.root, "P::Action")
		assertReflectiveElements(t, fixture.model, action, "input",
			"P::Action::inputValue")
		assertReflectiveElements(t, fixture.model, action, "output", "P::Action::outputValue")
		assertReflectiveElements(t, fixture.model, action, "parameter",
			"P::Action::inputValue", "P::Action::outputValue")
		assertReflectiveElements(t, fixture.model, action, "directedFeature",
			"P::Action::inputValue", "P::Action::outputValue")
		caseDef := nestedSym(t, fixture.root, "P::Case")
		assertReflectiveElements(t, fixture.model, caseDef, "subjectParameter", "P::Case::subject")
		assertReflectiveElements(t, fixture.model, caseDef, "input",
			"P::Case::subject", "P::Case::argument")
		caseSubjectSecond := nestedSym(t, fixture.root, "P::CaseSubjectSecond")
		assertReflectiveElements(t, fixture.model, caseSubjectSecond, "input",
			"P::CaseSubjectSecond::subject", "P::CaseSubjectSecond::argument")
		reordered := includeSubjectParameterInOrder(fixture.model, caseSubjectSecond, []*symbols.Symbol{
			nestedSym(t, fixture.root, "P::CaseSubjectSecond::argument"),
		})
		if len(reordered) < 2 ||
			reordered[0] != nestedSym(t, fixture.root, "P::CaseSubjectSecond::subject") ||
			reordered[1] != nestedSym(t, fixture.root, "P::CaseSubjectSecond::argument") {
			t.Errorf("subject insertion order = %s, want subject then argument", strings.Join(fqns(reordered), ", "))
		}
		analysis := nestedSym(t, fixture.root, "P::Analysis")
		assertReflectiveElements(t, fixture.model, analysis, "objectiveRequirement", "P::Analysis::objective")
		requirement := nestedSym(t, fixture.root, "P::Requirement")
		assertReflectiveElements(t, fixture.model, requirement, "subjectParameter", "P::Requirement::subject")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::Calculation"),
			"result", "P::Calculation::result")
	}
}

func TestReflectiveInputOrderAndSubjectIdentity(t *testing.T) {
	const src = `package P {
		part def Subject;
		part owner : Subject;
		concern def MassBudget {
			subject robot : Subject;
			in budget : Subject;
		}
		viewpoint def Perspective {
			subject system : Subject;
			in viewpointInput : Subject;
			frame concern framed : MassBudget;
		}
		requirement def Base {
			subject s : Subject;
			in inherited : Subject;
			in inheritedTail : Subject;
		}
		requirement def Derived :> Base {
			subject replacement : Subject :>> s;
			in own : Subject :>> inherited;
		}
		requirement req : Base {
			subject = owner;
			in ownInput : Subject;
		}
		verification def Verifier {
			subject system : Subject;
			objective { verify req; }
		}
		concern concernUse : MassBudget {
			subject = owner;
			in ownInput : Subject;
		}
		viewpoint viewpointUse : Perspective {
			subject = owner;
			in ownInput : Subject;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "subject-input-order.sysml", source.KindSysML, src) {
		verifier := nestedSym(t, fixture.root, "P::Verifier")
		var verifiedReq *symbols.Symbol
		verifier.Scope.ForEachAnonymousMember(func(member *symbols.Symbol) bool {
			if member.Scope == nil {
				return true
			}
			member.Scope.ForEachMember(func(child *symbols.Symbol) bool {
				if fixture.model.fqnOf(child) == "P::Verifier::req" {
					verifiedReq = child
					return false
				}
				return true
			})
			return verifiedReq == nil
		})
		if verifiedReq == nil {
			t.Fatalf("verification usage not found under P::Verifier")
		}
		for path, want := range map[string][]string{
			"P::MassBudget": {
				"P::MassBudget::robot", "P::MassBudget::budget",
			},
			"P::Perspective": {
				"P::Perspective::system", "P::Perspective::viewpointInput",
			},
			"P::Base": {
				"P::Base::s", "P::Base::inherited", "P::Base::inheritedTail",
			},
			"P::Derived": {
				"P::Derived::replacement", "P::Derived::own", "P::Base::inheritedTail",
			},
		} {
			assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, path), "input", want...)
		}
		for _, path := range []string{
			"P::MassBudget", "P::Perspective", "P::Base", "P::Derived",
			"P::req", "P::concernUse", "P::viewpointUse",
			"P::Perspective::framed",
		} {
			owner := nestedSym(t, fixture.root, path)
			input, inputOK := fixture.model.ReflectiveElements(owner, "input")
			subject, subjectOK := fixture.model.ReflectiveElements(owner, "subjectParameter")
			if !inputOK || !subjectOK || len(input) == 0 || len(subject) != 1 {
				t.Errorf("%s input=%v (present %t), subjectParameter=%v (present %t)",
					path, input, inputOK, subject, subjectOK)
				continue
			}
			foundSubject := false
			for _, parameter := range input {
				if parameter == subject[0] {
					foundSubject = true
					break
				}
			}
			if !foundSubject {
				t.Errorf("%s input %v does not contain subjectParameter %s",
					path, fqns(input), fixture.model.fqnOf(subject[0]))
			}
			if input[0] != subject[0] {
				t.Errorf("%s input[0] %s is not the owned subjectParameter %s",
					path, fixture.model.fqnOf(input[0]), fixture.model.fqnOf(subject[0]))
			}
		}
		input, inputOK := fixture.model.ReflectiveElements(verifiedReq, "input")
		subject, subjectOK := fixture.model.ReflectiveElements(verifiedReq, "subjectParameter")
		if !inputOK || !subjectOK || len(input) == 0 || len(subject) != 1 || input[0] != subject[0] {
			t.Errorf("%s input=%v (present %t), subjectParameter=%v (present %t)",
				fixture.model.fqnOf(verifiedReq), input, inputOK, subject, subjectOK)
		}
	}
}

func TestReflectiveDefinitionTypeAndIndividualFeatures(t *testing.T) {
	const src = `package P {
		part def PartType;
		individual occurrence def IndividualType;
		metadata def Marker;
		part partUsage : PartType;
		occurrence occurrenceUsage : IndividualType;
		part def Annotated { metadata marker : Marker; }
	}`
	for _, fixture := range reflectiveFixtures(t, "reflective.sysml", source.KindSysML, src) {
		partUsage := nestedSym(t, fixture.root, "P::partUsage")
		assertReflectiveElements(t, fixture.model, partUsage, "partDefinition", "P::PartType")
		assertReflectiveElements(t, fixture.model, partUsage, "itemDefinition", "P::PartType")
		occurrence := nestedSym(t, fixture.root, "P::occurrenceUsage")
		assertReflectiveElements(t, fixture.model, occurrence, "occurrenceDefinition", "P::IndividualType")
		assertReflectiveElements(t, fixture.model, occurrence, "individualDefinition", "P::IndividualType")
		if got, ok := fixture.model.ReflectiveElements(partUsage, "individualDefinition"); !ok || len(got) != 0 {
			t.Errorf("PartUsage.individualDefinition = %v, %t; want a present empty value", got, ok)
		}
		marker := nestedSym(t, fixture.root, "P::Annotated::marker")
		assertReflectiveElements(t, fixture.model, marker, "metadataDefinition", "P::Marker")
		assertReflectiveElements(t, fixture.model, marker, "metaclass", "P::Marker")
	}
}

func TestReflectiveObjectiveIsRequirementUsage(t *testing.T) {
	const src = `package P {
		requirement def Requirement;
		part def Base;
		part selected : Base;
		variation part def Choices :> Base {
			variant selected;
		}
		case def Case {
			objective objective : Requirement;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "objective-usage.sysml", source.KindSysML, src) {
		objective := nestedSym(t, fixture.root, "P::Case::objective")
		if objective.Kind != symbols.SymbolRequirementUsage ||
			!fixture.model.reflectiveMetaclassConforms(objective, "RequirementUsage") {
			t.Errorf("%s has kind %s; want the RequirementUsage kind and metaclass",
				fixture.model.fqnOf(objective), objective.Kind)
		}
		if fixture.model.reflectiveMetaclassConforms(objective, "PartUsage") {
			t.Errorf("%s conforms to PartUsage; objective requirement usages must not", fixture.model.fqnOf(objective))
		}
		if _, ok := fixture.model.ReflectiveElements(objective, "partDefinition"); ok {
			t.Errorf("%s.partDefinition is supported; objective is not a PartUsage", fixture.model.fqnOf(objective))
		}
		variant := nestedSym(t, fixture.root, "P::Choices::selected")
		if !fixture.model.reflectiveMetaclassConforms(variant, "ReferenceUsage") {
			t.Errorf("%s does not conform to ReferenceUsage", fixture.model.fqnOf(variant))
		}
		if fixture.model.reflectiveMetaclassConforms(variant, "PartUsage") {
			t.Errorf("%s conforms to PartUsage; a variant reference is not a declaration", fixture.model.fqnOf(variant))
		}
	}
}

func TestReflectivePartUsageIncludesImplicitPartDefinition(t *testing.T) {
	const src = `package P {
		item def ItemType;
		part actual : ItemType;
		case def Case {
			subject subject : ItemType;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "part-usage-types.sysml", source.KindSysML, src) {
		for _, path := range []string{"P::actual", "P::Case::subject"} {
			usage := nestedSym(t, fixture.root, path)
			assertReflectiveElements(t, fixture.model, usage, "type", "P::ItemType", "Parts::Part")
			assertReflectiveElements(t, fixture.model, usage, "partDefinition", "Parts::Part")
		}
	}
}

func TestReflectiveMetadataFeatureHasOneMetaclassType(t *testing.T) {
	const src = `package P {
		private import AnalysisRecords::*;
		part def Run;
		part run : Run {
			@AnalysisRecords::RecordedRun {
				runAt = "2025-01-01T00:00:00Z";
				tool = "sysml";
				command = "sysml";
				kind = "run";
			}
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "metadata-feature.sysml", source.KindSysML, src) {
		run := nestedSym(t, fixture.root, "P::run")
		var metadata *symbols.Symbol
		for _, member := range run.Scope.AllMembers() {
			if member.Kind == symbols.SymbolMetadataUsage {
				metadata = member
				break
			}
		}
		if metadata == nil {
			t.Fatalf("%s has no metadata usage", fixture.model.fqnOf(run))
		}
		metaclass := fixture.model.Metaclass("Metaclass")
		metadataDefinition := fixture.model.symbolByFQN("SysML::Systems::MetadataDefinition")
		recordedType := fixture.model.symbolByFQN("AnalysisRecords::RecordedRun")
		if metadataDefinition == nil || !fixture.model.Conforms(metadataDefinition, metaclass) {
			t.Errorf("SysML::Systems::MetadataDefinition does not conform to %s", fixture.model.fqnOf(metaclass))
		}
		if recordedType == nil || !symbols.SameElement(fixture.model.MetaclassOf(recordedType), metadataDefinition) {
			t.Errorf("AnalysisRecords::RecordedRun metaclass = %s, want SysML::Systems::MetadataDefinition", fixture.model.fqnOf(fixture.model.MetaclassOf(recordedType)))
		} else if !fixture.model.MetaclassConforms(recordedType, fixture.model.fqnOf(metaclass)) {
			t.Errorf("AnalysisRecords::RecordedRun's metaclass does not conform to %s", fixture.model.fqnOf(metaclass))
		}
		types, ok := fixture.model.ReflectiveElements(metadata, "type")
		if !ok || len(types) != 1 {
			t.Errorf("%s.type = %v, present %t; want one metaclass type", fixture.model.fqnOf(metadata), fqns(types), ok)
			continue
		}
		if got := fixture.model.fqnOf(types[0]); got != "AnalysisRecords::RecordedRun" {
			t.Errorf("%s.type = %s, want AnalysisRecords::RecordedRun", fixture.model.fqnOf(metadata), got)
		}
		if !fixture.model.MetaclassConforms(types[0], fixture.model.fqnOf(metaclass)) {
			t.Errorf("%s's metaclass does not conform to %s", fixture.model.fqnOf(types[0]), fixture.model.fqnOf(metaclass))
		}
		assertReflectiveElements(t, fixture.model, metadata, "metaclass", "AnalysisRecords::RecordedRun")
	}
}

func TestReflectiveTypeCompositionEndsAndPortionKind(t *testing.T) {
	const sysmlSrc = `package P {
		occurrence def Timeline {
			snapshot occurrence point;
			timeslice occurrence interval;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "reflective.sysml", source.KindSysML, sysmlSrc) {
		for path, want := range map[string]string{
			"P::Timeline::point":    "snapshot",
			"P::Timeline::interval": "timeslice",
		} {
			sym := nestedSym(t, fixture.root, path)
			value, ok := fixture.model.ReflectiveFeatureValue(sym, "portionKind")
			if !ok || value.Kind != symbols.FilterValueString || value.Str != want {
				t.Errorf("%s.portionKind = %v (present %t), want %q", path, value, ok, want)
			}
		}
	}

	const kermlSrc = `package K {
		class A; class B;
		class U unions A, B;
		class I intersects A, B;
		class D differences A, B;
		assoc Link { end left : A; end right : B; }
	}`
	for _, fixture := range reflectiveFixtures(t, "reflective.kerml", source.KindKerML, kermlSrc) {
		for feature, typeName := range map[string]string{
			"unioningType": "K::U", "intersectingType": "K::I", "differencingType": "K::D",
		} {
			assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, typeName),
				feature, "K::A", "K::B")
		}
		link := nestedSym(t, fixture.root, "K::Link")
		assertReflectiveElements(t, fixture.model, link, "endFeature", "K::Link::left", "K::Link::right")
		assertReflectiveElements(t, fixture.model, link, "ownedEndFeature", "K::Link::left", "K::Link::right")
	}
}

type reflectiveFixture struct {
	model *Model
	root  *symbols.Scope
}

func reflectiveFixtures(t *testing.T, name string, kind source.Kind, src string) []reflectiveFixture {
	t.Helper()
	idx := stdlibIndex(t)
	addTestDoc(t, idx, name, src)
	resolver := resolve.New(idx)
	model := NewModel(resolver)
	resolver.SetModel(model)
	root := idx.DocumentRoot(name)
	resolver.ResolveDocument(name, root.Node().(*ast.RootNamespace))

	record, err := symbols.RecordScope(root, func(*symbols.Symbol) bool { return true },
		func(sym *symbols.Symbol) symbols.LibraryFacts {
			return reflectiveRecordedFacts(model, idx, sym)
		})
	if err != nil {
		t.Fatalf("RecordScope: %v", err)
	}
	recorded, err := symbols.BuildRecorded(record, name)
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	recordedIndex := stdlibIndex(t)
	recordedIndex.AddRecordedDocument(name, kind, recorded, nil)
	recordedIndex.MarkLibrary(name)
	recordedResolver := resolve.New(recordedIndex)
	recordedModel := NewModel(recordedResolver)
	recordedResolver.SetModel(recordedModel)
	return []reflectiveFixture{
		{model: model, root: root},
		{model: recordedModel, root: recordedIndex.DocumentRoot(name)},
	}
}

func reflectiveRecordedFacts(model *Model, idx *symbols.Index, sym *symbols.Symbol) symbols.LibraryFacts {
	facts := symbols.LibraryFacts{
		Node:     symbols.NodeKindOf(sym.Decl),
		Keyword:  sym.Keyword(),
		Notation: sym.Notation(),
		Abstract: symbols.IsAbstract(sym),
		Supers:   []symbols.ElementRef{},
	}
	for _, super := range model.DirectSupertypes(sym) {
		if ref, ok := idx.RefTo(super); ok {
			facts.Supers = append(facts.Supers, ref)
		} else {
			facts.Supers = append(facts.Supers, symbols.ElementRef{FQN: model.fqnOf(super)})
		}
	}
	if ends, ok := model.OwnedConnectorEnds(sym); ok {
		facts.Ends = make([]symbols.ElementRef, len(ends))
		for i, end := range ends {
			if end == nil {
				continue
			}
			if ref, found := idx.RefTo(end); found {
				facts.Ends[i] = ref
			} else {
				facts.Ends[i] = symbols.ElementRef{FQN: model.fqnOf(end)}
			}
		}
	}
	if related, ok := model.ReflectiveElements(sym, "relatedFeature"); ok {
		facts.RelatedFeatures = make([]symbols.ElementRef, len(related))
		for i, feature := range related {
			if ref, found := idx.RefTo(feature); found {
				facts.RelatedFeatures[i] = ref
			} else {
				facts.RelatedFeatures[i] = symbols.ElementRef{FQN: model.fqnOf(feature)}
			}
		}
	}
	if _, ok := sym.Decl.(*ast.PrefixMetadata); ok {
		if types, supported := model.ReflectiveElements(sym, "type"); supported && len(types) == 1 {
			if ref, found := idx.RefTo(types[0]); found {
				facts.MetadataType = ref
			} else {
				facts.MetadataType = symbols.ElementRef{FQN: model.fqnOf(types[0])}
			}
		}
	}
	facts.UsageKind, _ = sym.UsageKind()
	facts.DefKind, _ = sym.DefinitionKind()
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		if d.IsIndividual {
			facts.Modifiers |= symbols.ModIndividual
		}
		if d.IsVariation {
			facts.Modifiers |= symbols.ModVariation
		}
		if d.IsAll {
			facts.Modifiers |= symbols.ModAll
		}
		if d.IsConstant {
			facts.Modifiers |= symbols.ModConstant
		}
	case *ast.Usage:
		facts.Direction = d.Direction
		facts.Portion = d.Portion
		mods := []struct {
			on  bool
			mod symbols.Modifiers
		}{
			{d.IsVariation, symbols.ModVariation},
			{d.IsVariant, symbols.ModVariant},
			{d.IsAll, symbols.ModAll},
			{d.IsEnd, symbols.ModEnd},
			{d.IsDerived, symbols.ModDerived},
			{d.IsConstant, symbols.ModConstant},
			{d.IsVariable, symbols.ModVariable},
			{d.IsComposite, symbols.ModComposite},
			{d.IsReference, symbols.ModReference},
			{d.IsPortion || d.Portion != ast.PortionNone, symbols.ModPortion},
			{d.IsIndividual, symbols.ModIndividual},
			{d.IsOrdered, symbols.ModOrdered},
			{d.IsNonunique, symbols.ModNonunique},
			{d.IsResult, symbols.ModResult},
		}
		for _, modifier := range mods {
			if modifier.on {
				facts.Modifiers |= modifier.mod
			}
		}
	}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil {
			continue
		}
		rf := symbols.RelationshipFacts{Kind: rel.Kind, Conjugated: rel.Conjugated}
		if target := model.RelationshipTarget(sym, rel); target != nil {
			if ref, ok := idx.RefTo(target); ok {
				rf.Target = ref
			} else {
				rf.Target = symbols.ElementRef{FQN: model.fqnOf(target)}
			}
		}
		facts.Relationships = append(facts.Relationships, rf)
	}
	return facts
}

func assertReflectiveElements(t *testing.T, model *Model, sym *symbols.Symbol, feature string, want ...string) {
	t.Helper()
	got, ok := model.ReflectiveElements(sym, feature)
	if !ok {
		t.Errorf("%s.%s is unsupported", symbols.FQNOf(sym), feature)
		return
	}
	gotNames := make([]string, 0, len(got))
	for _, element := range got {
		gotNames = append(gotNames, model.fqnOf(element))
	}
	if strings.Join(gotNames, "\x00") != strings.Join(want, "\x00") {
		typeNames := make([]string, 0)
		for _, typ := range model.FeatureTypeSet(sym) {
			typeNames = append(typeNames, model.fqnOf(typ))
		}
		t.Errorf("%s.%s = %v, want %v (feature types %v, recorded %t, kind %s, decl %T, direct supertypes %v, relationships %v)",
			symbols.FQNOf(sym), feature, gotNames, want, typeNames, sym.Recorded(), sym.Kind, sym.Decl,
			fqns(model.DirectSupertypes(sym)), RelationshipsOf(sym))
	}
}
