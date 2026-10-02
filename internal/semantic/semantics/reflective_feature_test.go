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
		if sym.Recorded() && !m.isKerMLDoc(sym) &&
			m.metaclassConforms(sym, sysmlMetaclassPrefix+"Usage") {
			unsupported := feature == "isVariable" ||
				(feature == "isConstant" && sym.Facts.Modifiers.Has(symbols.ModEnd) &&
					!sym.Facts.Modifiers.Has(symbols.ModConstant))
			if unsupported {
				if ok {
					t.Errorf("%s.%s is supported for a recorded SysML usage without mayTimeVary facts", symbols.FQNOf(sym), feature)
				}
				continue
			}
		}
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
	src := symbols.NewIndex()
	addTestDoc(t, src, "lib.sysml", `part def Owner {
		part child;
		ref part borrowed;
		constant attribute frozen;
	}
	connection def C { end part endpoint; }
	action def Flow { fork f; }`)
	addTestDoc(t, src, "lib.kerml", `package K { class C { const feature f; } }`)
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
	fork := nestedSym(t, root, "Flow::f")
	kermlConst := nestedSym(t, idx.DocumentRoot("lib.kerml"), "K::C::f")
	if !child.Recorded() || !borrowed.Recorded() || !frozen.Recorded() || !fork.Recorded() {
		t.Fatal("recorded symbols lost their recorded status")
	}
	assertReflectiveFeatureFlags(t, m, child)
	assertReflectiveFeatureFlags(t, m, borrowed)
	assertReflectiveFeatureFlags(t, m, frozen)
	assertReflectiveFeatureFlags(t, m, endpoint)
	assertReflectiveFeatureFlags(t, m, fork)
	if got, ok := m.ReflectiveFeatureValue(endpoint, "isEnd"); !ok || !got.Bool {
		t.Errorf("recorded endpoint.isEnd = %v (present %t), want true", got, ok)
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
			in argument : SubjectType;
			subject subject : SubjectType;
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
			"P::CaseSubjectSecond::argument", "P::CaseSubjectSecond::subject")
		analysis := nestedSym(t, fixture.root, "P::Analysis")
		assertReflectiveElements(t, fixture.model, analysis, "objectiveRequirement", "P::Analysis::objective")
		requirement := nestedSym(t, fixture.root, "P::Requirement")
		assertReflectiveElements(t, fixture.model, requirement, "subjectParameter", "P::Requirement::subject")
		assertReflectiveElements(t, fixture.model, nestedSym(t, fixture.root, "P::Calculation"),
			"result", "P::Calculation::result")
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
