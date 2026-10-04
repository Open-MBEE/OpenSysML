package semantics

import (
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
	if got := m.ImplicitMultiplicityApplies(nestedSym(t, root, "P::f")); got {
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
