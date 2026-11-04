package semantics

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestRecordedSubjectDoesNotTakeDefaultMultiplicity(t *testing.T) {
	const src = `package P {
		part def T;
		requirement def R {
			subject s : T;
			actor a : T;
			stakeholder k : T;
		}
		use case def C { subject s : T; }
	}`
	parsedModel, parsedRoot := buildModel(t, src)
	record, err := symbols.RecordScope(parsedRoot, func(*symbols.Symbol) bool { return true }, func(sym *symbols.Symbol) symbols.LibraryFacts {
		facts := symbols.LibraryFacts{
			Node:         symbols.NodeKindOf(sym.Decl),
			Keyword:      sym.Keyword(),
			Multiplicity: parsedModel.MultiplicityFactsOf(sym),
		}
		facts.UsageKind, _ = sym.UsageKind()
		return facts
	})
	if err != nil {
		t.Fatalf("RecordScope: %v", err)
	}
	recordedScope, err := symbols.BuildRecorded(record, "recorded.sysml")
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	idx := symbols.NewIndex()
	idx.AddRecordedDocument("recorded.sysml", source.KindSysML, recordedScope, nil)
	resolver := resolve.New(idx)
	recordedModel := NewModel(resolver)
	resolver.SetModel(recordedModel)

	recordedRoot := idx.DocumentRoot("recorded.sysml")
	for _, path := range []string{"P::R::s", "P::C::s"} {
		t.Run(path, func(t *testing.T) {
			parsed := nestedSym(t, parsedRoot, path)
			recorded := nestedSym(t, recordedRoot, path)
			if !recorded.Recorded() || recorded.Decl != nil {
				t.Fatalf("recorded subject has Recorded()=%v, Decl=%T", recorded.Recorded(), recorded.Decl)
			}
			if path == "P::R::s" && (recorded.Facts.Node != symbols.NodeSubject || recorded.Facts.Keyword != "") {
				t.Fatalf("recorded requirement subject facts = %+v, want NodeSubject with no keyword", recorded.Facts)
			}
			if path == "P::C::s" && (recorded.Facts.Node != symbols.NodeUsage || recorded.Facts.UsageKind != ast.UsageSubject) {
				t.Fatalf("recorded case subject facts = %+v, want a subject Usage", recorded.Facts)
			}
			if parsedModel.ImplicitMultiplicityApplies(parsed) {
				t.Error("parsed subject takes the default multiplicity")
			}
			if recordedModel.ImplicitMultiplicityApplies(recorded) {
				t.Error("recorded subject takes the default multiplicity")
			}
			parsedRange := parsedModel.EffectiveParameterRange(parsed)
			recordedRange := recordedModel.EffectiveParameterRange(recorded)
			if recordedRange != parsedRange {
				t.Errorf("recorded subject range = %+v, parsed range = %+v", recordedRange, parsedRange)
			}
		})
	}

	for _, path := range []string{"P::R::a", "P::R::k"} {
		recorded := nestedSym(t, recordedRoot, path)
		if !recorded.Recorded() {
			t.Errorf("%s is not recorded", path)
		}
		if !recordedModel.ImplicitMultiplicityApplies(recorded) {
			t.Errorf("recorded usage %s does not take the default multiplicity", path)
		}
	}
}

func TestImplicitMultiplicityAppliesByUsageMetaclass(t *testing.T) {
	const src = `package P {
		enum def E { enum value; }
		part def T;
		view def V;
		rendering def R;
		metadata def M;
		part packagePart;

		part def D {
			attribute attributeUsage;
			enum enumerationUsage : E;
			item itemUsage;
			ref item referenceItem;
			part partUsage;
			ref part referencePart;
			individual part individualPart;
			snapshot part snapshotPart;
			timeslice part timeslicePart;
			port portUsage;
			view viewUsage : V;
			rendering renderingUsage : R;
			viewpoint viewpointUsage;
			metadata metadataUsage : M;
			item itemSubsetMetadata :> metadataUsage;
			connection connectionUsage;
			interface interfaceUsage;
			allocation allocationUsage;
			flow flowUsage;
			occurrence occurrenceUsage;
			event occurrence eventOccurrence;
			individual occurrence individualOccurrence;
			ref referenceUsage : T;
			action actionUsage;
			in inputUsage : T;
			feature genericFeature;
		}
		requirement def Req {
			subject requirementSubject : T;
			actor requirementActor : T;
			stakeholder requirementStakeholder : T;
		}
		use case def Case {
			subject caseSubject : T;
			actor caseActor : T;
			objective caseObjective;
		}
		action def Act { in attribute inputAttribute; }
	}`
	m, root := buildModelWithStdlib(t, src)
	cases := []struct {
		path  string
		apply bool
	}{
		{"P::D::attributeUsage", true},
		{"P::D::enumerationUsage", true},
		{"P::D::itemUsage", true},
		{"P::D::referenceItem", true},
		{"P::D::partUsage", true},
		{"P::D::referencePart", true},
		{"P::D::individualPart", true},
		{"P::D::snapshotPart", true},
		{"P::D::timeslicePart", true},
		{"P::D::portUsage", true},
		{"P::D::viewUsage", true},
		{"P::D::renderingUsage", true},
		{"P::Req::requirementActor", true},
		{"P::Req::requirementStakeholder", true},
		{"P::Case::caseActor", true},
		{"P::Act::inputAttribute", true},
		{"P::D::viewpointUsage", false},
		{"P::D::metadataUsage", false},
		{"P::D::connectionUsage", false},
		{"P::D::interfaceUsage", false},
		{"P::D::allocationUsage", false},
		{"P::D::flowUsage", false},
		{"P::D::occurrenceUsage", false},
		{"P::D::eventOccurrence", false},
		{"P::D::individualOccurrence", false},
		{"P::D::referenceUsage", false},
		{"P::D::actionUsage", false},
		{"P::D::inputUsage", false},
		{"P::D::genericFeature", false},
		{"P::Req::requirementSubject", false},
		{"P::Case::caseSubject", false},
		{"P::Case::caseObjective", false},
		{"P::packagePart", false},
		{"P::D::itemSubsetMetadata", true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := m.ImplicitMultiplicityApplies(nestedSym(t, root, tc.path)); got != tc.apply {
				t.Errorf("ImplicitMultiplicityApplies(%s) = %v, want %v", tc.path, got, tc.apply)
			}
		})
	}
}

func TestImplicitMultiplicityAppliesRejectsKerMLFeatures(t *testing.T) {
	m, root := buildModelWithStdlibNamedKind(t, "test.kerml", source.KindKerML, `class P { feature f; }`)
	if m.ImplicitMultiplicityApplies(nestedSym(t, root, "P::f")) {
		t.Fatal("a KerML feature should not take the SysML default multiplicity")
	}
}

func TestImplicitMultiplicitySubsettingKindsAndChains(t *testing.T) {
	const src = `package P {
		part g;
		metadata def M;
		part def W { part w [0..*]; }
		part def D {
			part a [0..*];
			part b :> a;
			part d ::> a;
			part e references a;
			part cross => a;
		}
		part def RedefinitionBase { part a [0..*]; }
		part def RedefinitionD :> RedefinitionBase { part c :>> a; }
		part def D2 {
			part a : W;
			part subsetChain :> a.w;
			part referenceChain ::> a.w;
		}
		part def D3 {
			part h :> g;
			part i ::> g;
		}
		part def D4 {
			metadata m : M;
			item x :> m;
		}
	}`
	m, root := buildModelWithStdlib(t, src)
	cases := []struct {
		path string
		want bool
	}{
		{"P::D::a", true},
		{"P::D::b", false},
		{"P::D::d", false},
		{"P::D::e", false},
		{"P::D::cross", false},
		{"P::RedefinitionD::c", false},
		{"P::D2::subsetChain", false},
		{"P::D2::referenceChain", false},
		{"P::D3::h", true},
		{"P::D3::i", true},
		{"P::D4::m", false},
		{"P::D4::x", true},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := m.ImplicitMultiplicityApplies(nestedSym(t, root, tc.path)); got != tc.want {
				t.Errorf("ImplicitMultiplicityApplies(%s) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}

	redefinition := nestedSym(t, root, "P::RedefinitionD::c")
	var redefined *symbols.Symbol
	for _, rel := range RelationshipsOf(redefinition) {
		if rel.Kind == ast.RelRedefines {
			redefined = m.RelationshipTarget(redefinition, rel)
			break
		}
	}
	if want := nestedSym(t, root, "P::RedefinitionBase::a"); redefined != want {
		t.Errorf("RelationshipTarget(P::RedefinitionD::c) = %v, want inherited feature %v", redefined, want)
	}

	want := nestedSym(t, root, "P::W::w")
	for _, tc := range []struct {
		path string
		kind ast.RelationshipKind
	}{
		{"P::D2::subsetChain", ast.RelSubsets},
		{"P::D2::referenceChain", ast.RelReferences},
	} {
		sym := nestedSym(t, root, tc.path)
		var got *symbols.Symbol
		for _, rel := range RelationshipsOf(sym) {
			if rel.Kind == tc.kind {
				got = m.RelationshipTarget(sym, rel)
				break
			}
		}
		if got != want {
			t.Errorf("RelationshipTarget(%s) = %v, want final chain feature %v", tc.path, got, want)
		}
	}
}

func TestImplicitMultiplicityMetaclassSetMatchesSysMLHierarchy(t *testing.T) {
	m, _ := buildModelWithStdlib(t, `package P {}`)
	names := make(map[string]struct{})
	for _, name := range metaclassNames {
		if strings.HasSuffix(name, "Usage") {
			names[name] = struct{}{}
		}
	}
	for _, name := range usageMetaclassNames {
		if strings.HasSuffix(name, "Usage") {
			names[name] = struct{}{}
		}
	}
	for _, name := range []string{"EventOccurrenceUsage", "TransitionUsage"} {
		names[name] = struct{}{}
	}

	attribute := m.Metaclass("AttributeUsage")
	item := m.Metaclass("ItemUsage")
	port := m.Metaclass("PortUsage")
	connection := m.Metaclass("ConnectionUsage")
	if attribute == nil || item == nil || port == nil || connection == nil {
		t.Fatal("SysML usage metaclasses are missing from the bundled library")
	}
	for name := range names {
		meta := m.Metaclass(name)
		if meta == nil {
			t.Errorf("SysML metaclass %q is missing from the bundled library", name)
			continue
		}
		want := name != "MetadataUsage" &&
			(m.Conforms(meta, attribute) || m.Conforms(meta, item) || m.Conforms(meta, port)) &&
			!m.Conforms(meta, connection)
		_, got := implicitMultiplicityMetaclasses[name]
		if got != want {
			t.Errorf("implicit multiplicity membership for %s = %v, want %v", name, got, want)
		}
	}
}

// TestDefaultMultiplicityMember checks that a usage whose multiplicity is the
// implicit [1..1] owns a Multiplicity stating it — its Type::multiplicity, the
// member of the last of its owned memberships, an OwningMembership — while a
// declared range, a subsetting usage, an action, a variant, an enumerated value,
// a redefinition of a package-level feature and a KerML feature own none.
func TestDefaultMultiplicityMember(t *testing.T) {
	src := `package DM {
    part def Vehicle {
        attribute mass : ScalarValues::Real;
        attribute tags : ScalarValues::String [2];
        part wheels [4] { part hub; }
        part wheel : Wheel;
        part spare :> wheel;
        action go;
        part trim : Trim { variant part leather : Trim; }
        part cap redefines lid;
    }
    part lid : Wheel;
    part def Wheel;
    part def Trim;
    enum def Color { red; }
}`
	for _, fixture := range reflectiveFixtures(t, "default-multiplicity.sysml", source.KindSysML, src) {
		model := fixture.model
		mass := relFixtureSymbol(t, fixture, "mass")
		member := model.DefaultMultiplicityMember(mass)
		if member == nil {
			t.Fatal("DefaultMultiplicityMember(mass) = nil, want the Multiplicity stating its implicit [1..1]")
		}
		if again := model.DefaultMultiplicityMember(mass); again != member {
			t.Error("DefaultMultiplicityMember(mass) changed between reads")
		}
		if !IsDefaultMultiplicity(member) || !member.Implied() {
			t.Error("the member is not recognised as an implied default multiplicity")
		}
		if got := model.fqnOf(model.MetaclassOf(member)); got != "KerML::Core::Multiplicity" {
			t.Errorf("metaclass = %s, want KerML::Core::Multiplicity", got)
		}
		assertRelFeature(t, model, mass, "multiplicity", member)
		assertRelFeature(t, model, member, "owner", mass)
		if members, _ := model.ReflectiveElements(mass, "ownedMember"); len(members) != 1 || members[0] != member {
			t.Errorf("mass ownedMember = %d elements, want the default multiplicity alone", len(members))
		}
		if members, _ := model.ReflectiveElements(mass, "member"); len(members) == 0 || members[len(members)-1] != member {
			t.Errorf("mass member = %d elements, want the default multiplicity last", len(members))
		}
		if features, _ := model.ReflectiveElements(mass, "feature"); slices.Contains(features, member) {
			t.Error("mass feature holds the default multiplicity: an OwningMembership is no FeatureMembership")
		}
		if elements, _ := model.ReflectiveElements(mass, "ownedElement"); len(elements) == 0 || elements[len(elements)-1] != member {
			t.Errorf("mass ownedElement = %d elements, want the default multiplicity last", len(elements))
		}
		rels := model.OwnedRelationships(mass)
		if len(rels) == 0 {
			t.Fatal("mass owns no relationship")
		}
		membership := rels[len(rels)-1]
		if membership.Implicit == nil || membership.Implicit.Member != member {
			t.Fatalf("the last owned relationship of mass is %s, want the default multiplicity's membership", relMetaName(model, membership))
		}
		if got := relMetaName(model, membership); got != "OwningMembership" {
			t.Errorf("membership metaclass = %s, want OwningMembership", got)
		}
		if got := model.OwningMembership(member); got != membership {
			t.Error("OwningMembership(member) is not the membership mass owns it through")
		}
		for sym, feature := range map[*symbols.Symbol]string{membership: "memberName", member: "qualifiedName"} {
			if value, ok := model.ReflectiveFeatureValues(sym, feature); !ok || len(value) > 1 || (len(value) == 1 && value[0].Kind != symbols.FilterValueEmpty) {
				t.Errorf("%s = %v, %t; want none: the member is unnamed", feature, value, ok)
			}
		}
		if symbols.KeyOf(member) == symbols.KeyOf(mass) || symbols.SameElement(member, mass) {
			t.Error("KeyOf(default multiplicity) collides with its usage")
		}
		for _, name := range []string{"tags", "spare", "go", "leather", "red", "cap"} {
			sym := relFixtureSymbol(t, fixture, name)
			if model.DefaultMultiplicityMember(sym) != nil {
				t.Errorf("%s owns a default multiplicity, want none", name)
			}
		}
		tags := relFixtureSymbol(t, fixture, "tags")
		if got, _ := model.ReflectiveElements(tags, "multiplicity"); len(got) != 1 || !IsImpliedMultiplicity(got[0]) || IsDefaultMultiplicity(got[0]) {
			t.Errorf("tags multiplicity = %d elements, want its declared range alone", len(got))
		} else if name := MultiplicityMetaclassName(got[0]); name != "MultiplicityRange" {
			t.Errorf("tags range metaclass = %s, want MultiplicityRange", name)
		} else if owned, _ := model.ReflectiveElements(tags, "ownedMember"); len(owned) == 0 || owned[0] != got[0] {
			t.Errorf("tags ownedMember = %d elements, want its declared range first", len(owned))
		}
		wheels := relFixtureSymbol(t, fixture, "wheels")
		rangeOf := model.DeclaredMultiplicityMember(wheels)
		for _, feature := range []string{"ownedMember", "member", "ownedElement"} {
			if got, _ := model.ReflectiveElements(wheels, feature); len(got) < 2 || got[0] != rangeOf {
				t.Errorf("wheels %s = %d elements, want its declared range before hub", feature, len(got))
			}
		}
		if got, _ := model.ReflectiveElements(wheels, "ownedMembership"); len(got) < 2 || got[0].Implicit == nil || got[0].Implicit.Member != rangeOf {
			t.Errorf("wheels ownedMembership = %d elements, want the range's membership before hub's", len(got))
		}
		if got, _ := model.ReflectiveElements(relFixtureSymbol(t, fixture, "tags"), "ownedRelationship"); len(got) == 0 {
			t.Errorf("tags ownedRelationship = %d elements, want the range's OwningMembership among them", len(got))
		}
	}
	for _, fixture := range reflectiveFixtures(t, "default-multiplicity.kerml", source.KindKerML, "package DK {\n    classifier C { feature f : C; }\n}") {
		if fixture.model.DefaultMultiplicityMember(relFixtureSymbol(t, fixture, "f")) != nil {
			t.Error("a KerML feature owns a default multiplicity; only a SysML usage takes the implicit [1..1]")
		}
	}
}
