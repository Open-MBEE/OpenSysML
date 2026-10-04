package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

func TestSemanticSubjectMapUsesNormativeLibraryIDs(t *testing.T) {
	file := source.New("m.sysml", []byte("package M;"))
	root := parser.New(file).ParseFile()
	_, res, _, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	index := res.Index()
	var symbol *symbols.Symbol
	for _, candidate := range index.LookupQualified("Occurrences::Occurrence") {
		if candidate != nil {
			symbol = candidate
			break
		}
	}
	if symbol == nil || !index.Library(symbol) {
		t.Fatal("resolver did not provide the library occurrence definition")
	}
	libraryElement, ok := identity.LibraryCatalog(index).ElementForSymbol(symbol)
	if !ok || libraryElement.ID == "" || libraryElement.OwningMembershipID == "" {
		t.Fatal("library catalog omitted the element or owning-membership id")
	}
	subjects := newSemanticSubjectMap()
	subjects.resolver = res
	elementTerm, ok := subjects.libraryElementTerm(metamodel.ElementOf(symbol))
	if !ok || elementTerm != rdf.ElementIRIForID(libraryElement.ID) {
		t.Fatalf("library element term = %s, want id %q", elementTerm, libraryElement.ID)
	}
	membershipTerm, ok := subjects.libraryMembershipTerm(metamodel.ElementOf(symbol).Membership)
	if !ok || membershipTerm != rdf.ElementIRIForID(libraryElement.OwningMembershipID) {
		t.Fatalf("library membership term = %s, want id %q", membershipTerm, libraryElement.OwningMembershipID)
	}
}

func TestGraphStructureTreatsLibraryStubContainmentAsUnknown(t *testing.T) {
	file := source.New("library-stub.sysml", []byte("package M;"))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var symbol *symbols.Symbol
	for _, candidate := range res.Index().LookupQualified("ISQBase::MassValue") {
		if candidate != nil {
			symbol = candidate
			break
		}
	}
	if symbol == nil {
		t.Fatal("resolver did not provide ISQBase::MassValue")
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	handle := metamodel.ElementOf(symbol)
	if _, ok := subjects.byElement[handle.Key()]; !ok || !subjects.isLibraryHandle(handle) {
		t.Fatal("ISQBase::MassValue has no library stub subject")
	}
	structure := newGraphStructure(graph, subjects)
	if _, known := structure.OwnedRelationships(handle); known {
		t.Error("library stub OwnedRelationships is known")
	}
	if _, known, ok := structure.OwningRelationship(handle); known || ok {
		t.Errorf("library stub OwningRelationship = (%t, %t), want unknown", known, ok)
	}
	if _, known, ok := structure.OwningRelatedElement(handle); known || ok {
		t.Errorf("library stub OwningRelatedElement = (%t, %t), want unknown", known, ok)
	}
	if _, known := structure.OwnedRelatedElements(handle); known {
		t.Error("library stub OwnedRelatedElements is known")
	}
	seen := make(map[string]bool)
	for _, property := range ontology.Properties() {
		if seen[property.Name] || !(strings.HasPrefix(property.Name, "owned") || strings.HasPrefix(property.Name, "owning")) {
			continue
		}
		seen[property.Name] = true
		if _, known := structure.RelatedElements(handle, property.Name); known {
			t.Errorf("library stub %s is known", property.Name)
		}
	}
}

func TestFullAPIJSONLibraryStubUsesCatalogIdentityAndOmitsContainment(t *testing.T) {
	const model = `package Vehicles {
		part def Vehicle {
			attribute mass : ISQ::MassValue;
		}
	}`
	file := source.New("library-stub.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	_, resolver, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var librarySymbol *symbols.Symbol
	for _, candidate := range resolver.Index().LookupQualified("ISQBase::MassValue") {
		if candidate != nil {
			librarySymbol = candidate
			break
		}
	}
	if librarySymbol == nil {
		t.Fatal("resolver did not provide ISQBase::MassValue")
	}
	libraryElement, ok := identity.LibraryCatalog(resolver.Index()).ElementForSymbol(librarySymbol)
	if !ok {
		t.Fatal("library catalog did not provide ISQBase::MassValue")
	}
	expectedName := encoders[0].ids.model.EffectiveNameOf(librarySymbol)
	expectedShortName := encoders[0].ids.model.EffectiveShortNameOf(librarySymbol)
	output, err := ModelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		var qualifiedName string
		if err := json.Unmarshal(element["qualifiedName"], &qualifiedName); err != nil ||
			qualifiedName != libraryElement.FQN {
			continue
		}
		var name string
		if err := json.Unmarshal(element["name"], &name); err != nil || name != expectedName {
			t.Fatalf("library stub name = %s, want %s", element["name"], expectedName)
		}
		if expectedShortName != "" {
			var shortName string
			if err := json.Unmarshal(element["shortName"], &shortName); err != nil || shortName != expectedShortName {
				t.Fatalf("library stub shortName = %s, want %s", element["shortName"], expectedShortName)
			}
		} else if raw := strings.TrimSpace(string(element["shortName"])); raw != "" && raw != "null" {
			t.Fatalf("library stub shortName = %s, want unknown", raw)
		}
		var isLibrary bool
		if err := json.Unmarshal(element["isLibraryElement"], &isLibrary); err != nil || !isLibrary {
			t.Fatalf("library stub isLibraryElement = %s, want true", element["isLibraryElement"])
		}
		for property := range element {
			if property == "owner" || strings.HasPrefix(property, "owned") || strings.Contains(property, "owning") {
				t.Errorf("library stub contains unknown containment property %q", property)
			}
		}
		return
	}
	t.Fatalf("full API JSON has no ISQBase::MassValue library stub: %s", output)
}

func TestIndividualDeclarationsUseOccurrenceMetaclassesAndPreserveKeyword(t *testing.T) {
	const model = `package P {
		individual def Person;
		individual person : Person;
	}`
	file := source.New("individuals.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, _, _, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	seenDefinition, seenUsage := false, false
	for _, subject := range graph.Subjects() {
		typeTerm, ok := graph.Object(subject, rdf.RDFType)
		if !ok || typeTerm.Kind != rdf.TermIRI {
			continue
		}
		individual, ok := graph.Object(subject, rdf.SysML+"isIndividual")
		if !ok || individual.Kind != rdf.TermLiteral || individual.Value != "true" {
			continue
		}
		switch typeTerm.Value {
		case rdf.SysML + "OccurrenceDefinition":
			seenDefinition = true
		case rdf.SysML + "OccurrenceUsage":
			seenUsage = true
		default:
			t.Errorf("individual subject %s has unsupported metaclass %s", subject.Value, typeTerm.Value)
		}
		keyword, ok := graph.Object(subject, rdf.OpenSysML+xDeclaredKeyword)
		if !ok || keyword.Kind != rdf.TermLiteral || keyword.Value != "individual" {
			t.Errorf("individual subject %s has declared keyword %v, want individual", subject.Value, keyword)
		}
	}
	if !seenDefinition || !seenUsage {
		t.Fatalf("individual occurrence declarations found definition=%t usage=%t", seenDefinition, seenUsage)
	}
	for _, subject := range graph.Subjects() {
		if typeTerm, ok := graph.Object(subject, rdf.RDFType); ok && typeTerm.Value == rdf.SysML+"EmptyMultiplicityMember" {
			t.Errorf("individual definition emitted EmptyMultiplicityMember %s", subject.Value)
		}
	}
}

func TestConstructorUsesInstantiatedTypeInsteadOfFunction(t *testing.T) {
	const model = `package P {
		part def Product;
		part product = new Product();
	}`
	file := source.New("constructor.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, _, _, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var constructors []rdf.Term
	for _, subject := range graph.Subjects() {
		if metaclass, ok := graph.Object(subject, rdf.RDFType); ok && metaclass.Value == rdf.SysML+"ConstructorExpression" {
			constructors = append(constructors, subject)
		}
	}
	if len(constructors) != 1 {
		t.Fatalf("found %d ConstructorExpression subjects, want one", len(constructors))
	}
	constructor := constructors[0]
	if functions := graph.Objects(constructor, rdf.SysML+pFunction); len(functions) != 0 {
		t.Errorf("constructor function = %v, want no function property", functions)
	}
	instantiatedTypes := graph.Objects(constructor, rdf.SysML+pInstantiatedType)
	if len(instantiatedTypes) != 1 {
		t.Fatalf("constructor instantiatedType = %v, want one constructed type", instantiatedTypes)
	}
	if marker, ok := graph.Object(constructor, rdf.OpenSysML+xIsConstructor); !ok ||
		marker.Kind != rdf.TermLiteral || marker.Value != "true" || marker.Datatype != rdf.XSD+"boolean" {
		t.Errorf("constructor marker = %v, want true", marker)
	}
	if memberships := graph.Objects(constructor, rdf.SysML+pOwnedRelationship); len(memberships) == 0 {
		t.Error("constructor lost the owned callee membership")
	}
}

func TestFullAPIJSONFeatureInvocationUsesInstantiatedType(t *testing.T) {
	const model = `package P {
		calc naturalLogarithm {
			in value : ScalarValues::Real;
			return : ScalarValues::Real = value;
		}
		calc caller {
			return : ScalarValues::Real = naturalLogarithm(1.0);
		}
	}`
	file := source.New("feature-invocation.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	_, resolver, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	output, failures, err := modelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]json.RawMessage, len(elements))
	var invocation map[string]json.RawMessage
	for _, element := range elements {
		id := apiJSONElementID(t, element)
		byID[id] = element
		var metaclass string
		if err := json.Unmarshal(element["@type"], &metaclass); err == nil && metaclass == "InvocationExpression" {
			invocation = element
		}
	}
	if invocation == nil {
		t.Fatal("full API JSON has no InvocationExpression")
	}
	instantiated := apiJSONReferenceIDs(t, invocation["instantiatedType"])
	if len(instantiated) != 1 {
		t.Fatalf("InvocationExpression::instantiatedType = %v, want the callee Feature", instantiated)
	}
	callee := byID[instantiated[0]]
	var calleeType string
	if callee == nil || json.Unmarshal(callee["@type"], &calleeType) != nil ||
		calleeType != "CalculationUsage" {
		t.Fatalf("instantiatedType %s has type %q, want CalculationUsage", instantiated[0], calleeType)
	}
	types := apiJSONReferenceIDs(t, callee["type"])
	if len(types) != 1 {
		t.Fatalf("CalculationUsage::type = %v, want its Function type", types)
	}
	functions := apiJSONReferenceIDs(t, invocation["function"])
	if len(functions) != 1 {
		var qualifiedName string
		_ = json.Unmarshal(callee["qualifiedName"], &qualifiedName)
		t.Fatalf("InvocationExpression::function = %v, want a Function type; callee=%s qualifiedName=%q serialization failures=%v", functions, callee, qualifiedName, failures)
	}
	if functions[0] != types[0] {
		t.Fatalf("InvocationExpression::function = %v, want callee type %v", functions, types)
	}
	library, ok := identity.LibraryCatalog(resolver.Index()).Element(functions[0])
	if !ok {
		t.Fatalf("InvocationExpression::function target %s is neither a serialized element nor a library element", functions[0])
	}
	functionMetaclass := encoders[0].ids.model.MetaclassOf(library.Symbol)
	if functionMetaclass == nil || !ontology.IsAncestorOrSelf(functionMetaclass.Name, "Function") {
		t.Fatalf("InvocationExpression::function target %s has metaclass %v, want Function", functions[0], functionMetaclass)
	}
	behaviors := apiJSONReferenceIDs(t, invocation["behavior"])
	if len(behaviors) != 1 {
		t.Fatalf("InvocationExpression::behavior = %v, want a Behavior type", behaviors)
	}
	if behaviors[0] != types[0] {
		t.Fatalf("InvocationExpression::behavior = %v, want the callee's Function type %v", behaviors, types)
	}
	behaviorMetaclass := encoders[0].ids.model.MetaclassOf(library.Symbol)
	if behaviorMetaclass == nil || !ontology.IsAncestorOrSelf(behaviorMetaclass.Name, "Behavior") {
		t.Fatalf("InvocationExpression::behavior target %s has metaclass %v, want Behavior", behaviors[0], behaviorMetaclass)
	}
}

func TestFullAPIJSONFeatureTargetIncludesUnchainedFeatures(t *testing.T) {
	const model = `package P {
		part def Vehicle {
			attribute base;
			attribute mass;
			attribute dryMass :> base;
		}
	}`
	file := source.New("feature-target.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	output, err := ModelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]map[string]json.RawMessage, len(elements))
	for _, element := range elements {
		var qualifiedName string
		if json.Unmarshal(element["qualifiedName"], &qualifiedName) == nil {
			byName[qualifiedName] = element
		}
	}
	for _, qualifiedName := range []string{"P::Vehicle::mass", "P::Vehicle::dryMass"} {
		element := byName[qualifiedName]
		if element == nil {
			t.Fatalf("full API JSON has no %s feature", qualifiedName)
		}
		id := apiJSONElementID(t, element)
		if got := apiJSONReferenceIDs(t, element["featureTarget"]); len(got) != 1 || got[0] != id {
			t.Errorf("%s.featureTarget = %v, want self [%s]", qualifiedName, got, id)
		}
	}
}

func TestFullAPIJSONUsesEffectiveNamesForTransitionFeatures(t *testing.T) {
	const model = `package P {
		item def IgnitionCmd;
		state def Vehicle {
			state machine {
				state off;
				state starting;
				first off then starting;
				transition
					first off
					accept ignitionCmd : IgnitionCmd
					then starting;
			}
		}
	}`
	file := source.New("effective-transition-names.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	output, err := ModelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	names := make(map[string]int)
	ends := make(map[string]bool)
	triggerPayloadNamed := false
	for _, element := range elements {
		var name string
		if err := json.Unmarshal(element["name"], &name); err == nil && name != "" {
			names[name]++
		}
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			continue
		}
		if name == "earlierOccurrence" && strings.Contains(id, "_pend0") {
			ends["earlierOccurrence"] = true
		}
		if name == "laterOccurrence" && strings.Contains(id, "_pend1") {
			ends["laterOccurrence"] = true
		}
		if name == "ignitionCmd" && strings.Contains(id, "trigger__ignitionCmd") {
			triggerPayloadNamed = true
		}
	}
	for _, want := range []string{"earlierOccurrence", "laterOccurrence", "ignitionCmd"} {
		if names[want] == 0 {
			t.Errorf("full API JSON has no element with effective name %q; elements: %s", want, output)
		}
	}
	for _, want := range []string{"earlierOccurrence", "laterOccurrence"} {
		if !ends[want] {
			t.Errorf("full API JSON has no succession end with effective name %q", want)
		}
	}
	if !triggerPayloadNamed {
		t.Error("full API JSON has no transition trigger payload named ignitionCmd")
	}
}

func TestFullWriterOmitsUnserializableComputedValues(t *testing.T) {
	property := ontology.Property{DefiningClass: "Element", Name: "owner", Many: true}
	value := metamodel.Value{
		Kind: metamodel.SequenceValue,
		Values: []metamodel.Value{
			{Kind: metamodel.ElementValue, Element: metamodel.Element{Symbol: &symbols.Symbol{}}},
			{Kind: metamodel.ElementValue, Element: metamodel.Element{}},
		},
	}
	element, failures := appendFullProperty(nil, rdf.IRI("urn:source"), property, property, value, true, newSemanticSubjectMap())
	if len(element) != 0 {
		t.Fatalf("unserializable computed value wrote properties: %#v", element)
	}
	if len(failures) != 2 {
		t.Fatalf("got %d serialization failures, want one per missing reference", len(failures))
	}
	reasons := map[string]int{}
	for _, failure := range failures {
		reasons[failure.Reason]++
		if failure.Property != property.QualifiedName() {
			t.Errorf("failure property = %q, want %q", failure.Property, property.QualifiedName())
		}
	}
	if reasons[`model symbol "<unnamed element>" (unknown) has no graph subject`] != 1 || reasons["handle-key mismatch"] != 1 {
		t.Fatalf("failure reasons = %#v", reasons)
	}
}

func TestFullWriterUsesDeclaredPropertyCardinality(t *testing.T) {
	elementHandle := metamodel.Element{Symbol: &symbols.Symbol{}}
	subjects := newSemanticSubjectMap()
	subjects.byElement[elementHandle.Key()] = rdf.IRI("urn:target")
	property := ontology.Property{DefiningClass: "Relationship", Name: "source", Many: true}
	effective := ontology.Property{DefiningClass: "Import", Name: "importOwningNamespace"}
	element, failures := appendFullProperty(nil, rdf.IRI("urn:source"), property, effective,
		metamodel.Value{Kind: metamodel.ElementValue, Element: elementHandle}, true, subjects)
	if len(failures) != 0 {
		t.Fatalf("serialization failures = %v, want none", failures)
	}
	values, ok := element[0].value.([]any)
	if !ok || len(values) != 1 {
		t.Fatalf("Relationship::source value = %#v, want one-item array", element[0].value)
	}

	single := ontology.Property{DefiningClass: "Import", Name: "importedElement"}
	second := metamodel.Element{Symbol: &symbols.Symbol{}}
	subjects.byElement[second.Key()] = rdf.IRI("urn:other-target")
	element, failures = appendFullProperty(nil, rdf.IRI("urn:source"), single, single,
		metamodel.Value{Kind: metamodel.SequenceValue, Values: []metamodel.Value{
			{Kind: metamodel.ElementValue, Element: elementHandle},
			{Kind: metamodel.ElementValue, Element: second},
		}}, true, subjects)
	if len(element) != 0 || len(failures) != 1 || failures[0].Reason != "multiple values for single-valued property" {
		t.Fatalf("single-valued sequence wrote %#v with failures %v", element, failures)
	}
}

func TestFullAPIJSONMapsEvaluatedMembershipsToGraphSubjects(t *testing.T) {
	const model = `package P {
		part def Base { part size; }
		part def Derived specializes Base { part mass; }
	}`
	file := source.New("m.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var pkg *symbols.Symbol
	for _, sym := range res.Index().LookupQualified("P") {
		pkg = sym
		break
	}
	if pkg == nil {
		t.Fatal("resolver omitted package P")
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	evaluator := metamodel.New(res, encoders[0].ids.model, metamodel.Options{Structure: newGraphStructure(graph, subjects)})
	value, ok := evaluator.Property(metamodel.ElementOf(pkg), "Namespace", "membership")
	if !ok {
		t.Fatal("evaluator did not compute Namespace::membership")
	}
	for _, member := range value.Values {
		if member.Kind != metamodel.MembershipValue {
			t.Fatalf("membership contains %#v, want a membership", member)
		}
		if term := subjects.byMembership[member.Membership.Key()]; term.Value == "" {
			fqn := symbols.FQNOf(member.Membership.Member)
			t.Fatalf("membership %q has no graph identity", fqn)
		}
	}
}

func TestFullAPIJSONMapsSyntheticSubjectTyping(t *testing.T) {
	const model = `package P {
		item def Vehicle;
		requirement def R { subject v : Vehicle; }
	}`
	file := source.New("m.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var subject *symbols.Symbol
	for _, candidate := range res.Index().LookupQualified("P::R::v") {
		subject = candidate
		break
	}
	if subject == nil {
		t.Fatal("resolver omitted requirement subject")
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	settled, err := prepareFullAPIJSONGraph(graph, subjects)
	if err != nil {
		t.Fatal(err)
	}
	evaluator := metamodel.New(res, encoders[0].ids.model, metamodel.Options{
		Structure: newGraphStructure(settled, subjects),
	})
	value, ok := evaluator.Property(
		metamodel.ElementOf(subject), "Feature", "ownedTyping",
	)
	if !ok || len(value.Values) != 1 {
		t.Fatalf("Feature::ownedTyping = %#v, supported = %t", value, ok)
	}
	term := subjects.byElement[value.Values[0].Element.Key()]
	if term.Value == "" {
		t.Fatal("evaluated subject typing has no graph identity")
	}
	if !graph.Has(rdf.Triple{
		Subject: term, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("FeatureTyping"),
	}) {
		t.Fatalf("subject typing %s is not a FeatureTyping", term.Value)
	}
}

func TestFullAPIJSONMapsEverySysMLSubject(t *testing.T) {
	const model = `package P {
		item def Fuel;
		part def Sender { out item outlet : Fuel; }
		part def Receiver { in item inlet : Fuel; }
		part def Context {
			part source : Sender;
			part target : Receiver;
			flow transfer from source.outlet to target.inlet;
		}
	}`
	file := source.New("all-origins.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	var unmapped []string
	for _, subject := range graph.Subjects() {
		if hasSysMLType(graph.Objects(subject, rdf.RDFType)) {
			if _, ok := subjects.byIRI[subject.Value]; !ok {
				unmapped = append(unmapped, subject.Value)
			}
		}
	}
	if len(unmapped) > 0 {
		t.Fatalf("SysML subjects without semantic handles: %v", unmapped)
	}

	output, err := ModelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	for i, element := range elements {
		if _, ok := element["isLibraryElement"]; !ok {
			t.Errorf("element %d (%s) has no isLibraryElement property: %s", i, element["@type"], output)
		}
	}
}

func TestFullAPIJSONSpecializationOwningTypeMatchesTypedFeature(t *testing.T) {
	file := source.New("specialization-owning-type.sysml", []byte(`package P {
		part def Base;
		part value : Base;
	}`))
	root := parser.New(file).ParseFile()
	output, err := ModelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		var metaclass string
		if err := json.Unmarshal(element["@type"], &metaclass); err != nil || metaclass != "FeatureTyping" {
			continue
		}
		var typedFeature, owningType struct {
			ID string `json:"@id"`
		}
		if err := json.Unmarshal(element["typedFeature"], &typedFeature); err != nil {
			t.Fatalf("typedFeature: %v", err)
		}
		if err := json.Unmarshal(element["owningType"], &owningType); err != nil {
			t.Fatalf("owningType: %v", err)
		}
		if owningType.ID != typedFeature.ID {
			t.Fatalf("Specialization::owningType = %q, want typed feature %q", owningType.ID, typedFeature.ID)
		}
		return
	}
	t.Fatal("full API JSON has no FeatureTyping subject")
}

func TestGraphStructureResolvesAnnotationOpposites(t *testing.T) {
	graph := rdf.NewGraph()
	annotation := rdf.ElementIRIForID("annotation")
	annotatingElement := rdf.ElementIRIForID("metadata")
	annotatedElement := rdf.ElementIRIForID("target")
	graph.Add(annotation, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Annotation"))
	graph.Add(annotatingElement, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("MetadataUsage"))
	graph.Add(annotatedElement, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("PartDefinition"))
	graph.Add(annotation, rdf.SysMLTerm(pAnnotatingElement), annotatingElement)
	graph.Add(annotation, rdf.SysMLTerm(pAnnotatedElement), annotatedElement)

	subjects := newSemanticSubjectMap()
	annotationHandle := metamodel.Element{Aspect: "annotation"}
	annotatingHandle := metamodel.Element{Aspect: "metadata"}
	annotatedHandle := metamodel.Element{Aspect: "target"}
	subjects.add(annotation, annotationHandle)
	subjects.add(annotatingElement, annotatingHandle)
	subjects.add(annotatedElement, annotatedHandle)
	structure := newGraphStructure(graph, subjects)

	for _, property := range []string{"annotation", "annotatedElement"} {
		if got, known := structure.RelatedElements(annotatingHandle, property); known {
			t.Errorf("AnnotatingElement::%s = %#v, %t; want derived property to be unknown", property, got, known)
		}
	}
	owned, known := structure.OwnedRelationships(annotatedHandle)
	if !known || len(owned) != 1 || owned[0].Key() != annotationHandle.Key() {
		t.Errorf("AnnotatedElement owned relationships = %#v, %t; want indexed Annotation", owned, known)
	}
	owner, present, known := structure.OwningRelatedElement(annotationHandle)
	if !known || !present || owner.Key() != annotatedHandle.Key() {
		t.Errorf("Annotation owningRelatedElement = %#v, %t, %t; want annotated element", owner, present, known)
	}
}

func TestFullAPIJSONAnnotationOppositeReferencesRelationship(t *testing.T) {
	const model = `package P {
		part def A;
		part first : A;
		part second : A;
		#refinement dependency first to second;
	}`
	file := source.New("annotation-opposite.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	documents := []ModelDocument{{File: file, Root: root}}
	graph, resolver, encoders, err := modelToRDF(documents, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	subjects := buildSemanticSubjects(graph, resolver, encoders)
	var annotationSubject, metadataSubject rdf.Term
	for _, subject := range graph.Subjects() {
		switch graph.Type(subject) {
		case rdf.SysML + "Annotation":
			annotationSubject = subject
		case rdf.SysML + "MetadataUsage":
			metadataSubject = subject
		}
	}
	if annotationSubject.Value == "" || metadataSubject.Value == "" {
		t.Fatalf("prefix annotation fixture subjects: Annotation=%v MetadataUsage=%v", annotationSubject, metadataSubject)
	}
	if annotation := subjects.byIRI[annotationSubject.Value]; annotation.Key() == subjects.byIRI[metadataSubject.Value].Key() {
		t.Fatalf("Annotation and MetadataUsage share ElementKey %v", annotation.Key())
	}
	output, err := ModelToAPIJSON(documents, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := ModelToAPIJSON(documents, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, repeated) {
		t.Fatal("full API JSON output changed between repeated prefix-annotation writes")
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]json.RawMessage, len(elements))
	byType := make(map[string][]map[string]json.RawMessage)
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		byID[id] = element
		var metaclass string
		if err := json.Unmarshal(element["@type"], &metaclass); err == nil {
			byType[metaclass] = append(byType[metaclass], element)
		}
	}
	if len(byType["Annotation"]) != 1 || len(byType["MetadataUsage"]) != 1 || len(byType["Dependency"]) != 1 {
		t.Fatalf("prefix annotation fixture has Annotation=%d MetadataUsage=%d Dependency=%d",
			len(byType["Annotation"]), len(byType["MetadataUsage"]), len(byType["Dependency"]))
	}
	annotation := byType["Annotation"][0]
	metadata := byType["MetadataUsage"][0]
	dependency := byType["Dependency"][0]
	annotationID := apiJSONElementID(t, annotation)
	metadataID := apiJSONElementID(t, metadata)
	dependencyID := apiJSONElementID(t, dependency)
	if got := apiJSONReferenceIDs(t, annotation["ownedRelationship"]); len(got) != 0 {
		t.Fatalf("Annotation::ownedRelationship = %v, want []", got)
	}
	if got := apiJSONReferenceIDs(t, annotation["ownedRelatedElement"]); len(got) != 1 || got[0] != metadataID {
		t.Fatalf("Annotation::ownedRelatedElement = %v, want [%s]", got, metadataID)
	}
	if got := apiJSONReferenceIDs(t, annotation["owningRelatedElement"]); len(got) != 1 || got[0] != dependencyID {
		t.Fatalf("Annotation::owningRelatedElement = %v, want [%s]", got, dependencyID)
	}
	if raw := strings.TrimSpace(string(annotation["owner"])); raw != "null" {
		t.Fatalf("Annotation::owner = %s, want null", raw)
	}
	annotationOwned := apiJSONReferenceIDs(t, dependency["ownedRelationship"])
	if !containsString(annotationOwned, annotationID) {
		t.Fatalf("Dependency::ownedRelationship = %v, want Annotation %s", annotationOwned, annotationID)
	}
	if got := apiJSONReferenceIDs(t, dependency["ownedElement"]); len(got) != 1 || got[0] != metadataID {
		t.Fatalf("Dependency::ownedElement = %v, want MetadataUsage %s", got, metadataID)
	}
	metadataRelationships := apiJSONReferenceIDs(t, metadata["ownedRelationship"])
	var featureTyping map[string]json.RawMessage
	for _, relationshipID := range metadataRelationships {
		candidate := byID[relationshipID]
		var metaclass string
		if candidate != nil && json.Unmarshal(candidate["@type"], &metaclass) == nil && metaclass == "FeatureTyping" {
			featureTyping = candidate
		}
	}
	if featureTyping == nil {
		t.Fatalf("MetadataUsage %s owns no FeatureTyping relationship", metadataID)
	}
	if raw := strings.TrimSpace(string(featureTyping["owningNamespace"])); raw != "null" {
		t.Fatalf("FeatureTyping::owningNamespace = %s, want null", raw)
	}
}

func apiJSONElementID(t *testing.T, element map[string]json.RawMessage) string {
	t.Helper()
	var id string
	if err := json.Unmarshal(element["@id"], &id); err != nil {
		t.Fatal(err)
	}
	return id
}

func apiJSONReferenceIDs(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var many []struct {
		ID string `json:"@id"`
	}
	if err := json.Unmarshal(raw, &many); err == nil && many != nil {
		ids := make([]string, 0, len(many))
		for _, reference := range many {
			ids = append(ids, reference.ID)
		}
		return ids
	}
	var one struct {
		ID string `json:"@id"`
	}
	if err := json.Unmarshal(raw, &one); err == nil && one.ID != "" {
		return []string{one.ID}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestGraphStructureResolvesGuardExpression(t *testing.T) {
	graph := rdf.NewGraph()
	transition := rdf.ElementIRIForID("transition")
	guard := rdf.ElementIRIForID("guard")
	graph.Add(guard, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Expression"))
	graph.Add(transition, rdf.IRI(rdf.OpenSysML+xGuard), guard)

	subjects := newSemanticSubjectMap()
	transitionHandle := metamodel.Element{Aspect: "transition"}
	guardHandle := metamodel.Element{Aspect: "guard"}
	subjects.add(transition, transitionHandle)
	subjects.add(guard, guardHandle)
	structure := newGraphStructure(graph, subjects)

	got, ok := structure.RelatedElements(transitionHandle, "guardExpression")
	if !ok || len(got) != 1 || got[0].Key() != guardHandle.Key() {
		t.Fatalf("TransitionUsage::guardExpression = %#v, %t; want [%v]", got, ok, guardHandle)
	}
}

func TestGraphStructureRejectsDerivedAndUndeclaredPredicates(t *testing.T) {
	graph := rdf.NewGraph()
	feature := rdf.ElementIRIForID("feature")
	graph.Add(feature, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Feature"))
	graph.Add(feature, rdf.SysMLTerm("type"), rdf.ElementIRIForID("type"))
	graph.Add(feature, rdf.SysMLTerm("name"), rdf.String("derived"))
	graph.Add(feature, rdf.SysMLTerm("notDeclared"), rdf.ElementIRIForID("unknown"))

	handle := metamodel.Element{Aspect: "feature"}
	subjects := newSemanticSubjectMap()
	subjects.add(feature, handle)
	structure := newGraphStructure(graph, subjects)

	for _, property := range []string{"type", "notDeclared"} {
		if _, ok := structure.RelatedElements(handle, property); ok {
			t.Errorf("RelatedElements(%q) reported an undeclared or derived predicate as known", property)
		}
	}
	for _, property := range []string{"name", "notDeclared"} {
		if _, ok := structure.Attribute(handle, property); ok {
			t.Errorf("Attribute(%q) reported an undeclared or derived predicate as known", property)
		}
	}
	isVariable, ok := structure.Attribute(handle, "isVariable")
	if !ok || isVariable.Kind != metamodel.BooleanValue || isVariable.Boolean {
		t.Fatalf("Attribute(isVariable) = %#v, %t; want its false ontology default", isVariable, ok)
	}
}

func TestFullWriterUsesGraphValueForRedefinedProperty(t *testing.T) {
	file := source.New("redefined-property.sysml", []byte("package P;"))
	root := parser.New(file).ParseFile()
	_, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}

	graph := rdf.NewGraph()
	subject := rdf.ElementIRIForID("typing")
	feature := rdf.ElementIRIForID("feature")
	graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("FeatureTyping"))
	graph.Add(subject, rdf.SysMLTerm("owningFeature"), feature)
	element, err := apiJSONElement(graph, subject)
	if err != nil {
		t.Fatal(err)
	}
	subjects := newSemanticSubjectMap()
	subjects.byIRI[subject.Value] = metamodel.Element{Node: &ast.Relationship{}}
	evaluator := metamodel.New(res, encoders[0].ids.model, metamodel.Options{Structure: newGraphStructure(graph, subjects)})
	element, err = appendFullProperties(graph, subject, element, evaluator, subjects)
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range []string{"owningType", "owningFeature"} {
		if _, ok := apiJSONMemberValue(element, property); ok {
			t.Errorf("full API JSON trusted the derived graph shortcut %s", property)
		}
	}
}

func TestFullAPIJSONKeepsCompactGraphValues(t *testing.T) {
	const model = `package P {
		part def Vehicle {
			part wheel;
		}
	}`
	file := source.New("compact-graph-values.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, _, _, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	output, _, err := modelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]json.RawMessage, len(elements))
	for _, element := range elements {
		byID[apiJSONElementID(t, element)] = element
	}
	found := false
	for _, subject := range graph.Subjects() {
		metaclass := strings.TrimPrefix(graph.Type(subject), rdf.SysML)
		if !ontology.IsAncestorOrSelf(metaclass, "OwningMembership") {
			continue
		}
		members := graph.Objects(subject, rdf.SysML+"memberElement")
		if len(members) != 1 {
			continue
		}
		membershipID, ok := graph.Lexical(subject, rdf.SysML+"elementId")
		if !ok {
			t.Fatalf("owning membership %s has no elementId", subject.Value)
		}
		memberID, ok := graph.Lexical(members[0], rdf.SysML+"elementId")
		if !ok {
			t.Fatalf("member %s has no elementId", members[0].Value)
		}
		element := byID[membershipID]
		if element == nil {
			t.Fatalf("full API JSON omitted owning membership %s", membershipID)
		}
		references := apiJSONReferenceIDs(t, element["memberElement"])
		if len(references) != 1 || references[0] != memberID {
			t.Errorf("Membership::memberElement = %v, want compact graph value %q", references, memberID)
		}
		derived := apiJSONReferenceIDs(t, element["ownedMemberElement"])
		if len(derived) > 0 && (len(derived) != 1 || derived[0] != memberID) {
			t.Errorf("OwningMembership::ownedMemberElement = %v, disagrees with owned member %q", derived, memberID)
		}
		found = true
	}
	if !found {
		t.Fatal("model has no graph-stated OwningMembership::memberElement")
	}
}

func TestFullWriterReplacesGraphValueForDerivedProperty(t *testing.T) {
	file := source.New("derived-property.sysml", []byte(`package P {
		part def T;
		part usage : T;
	}`))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var usage *symbols.Symbol
	for _, candidate := range res.Index().LookupQualified("P::usage") {
		if candidate != nil {
			usage = candidate
			break
		}
	}
	if usage == nil {
		t.Fatal("resolver did not index P::usage")
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	handle := metamodel.ElementOf(usage)
	subject, ok := subjects.byElement[handle.Key()]
	if !ok {
		t.Fatal("P::usage has no graph subject")
	}
	evaluator := metamodel.New(res, encoders[0].ids.model, metamodel.Options{
		Structure: newGraphStructure(graph, subjects),
	})
	want, ok := evaluator.Property(handle, "Element", "qualifiedName")
	if !ok || want.Kind != metamodel.StringValue {
		t.Fatalf("evaluator qualifiedName = %#v, %t", want, ok)
	}
	element, err := apiJSONElement(graph, subject)
	if err != nil {
		t.Fatal(err)
	}
	element = removeAPIJSONMember(element, "qualifiedName")
	element = append(element, apiJSONMember{key: "qualifiedName", value: "incorrect shortcut"})
	element, err = appendFullProperties(graph, subject, element, evaluator, subjects)
	if err != nil {
		t.Fatal(err)
	}
	qualifiedName, ok := apiJSONMemberValue(element, "qualifiedName")
	if !ok || qualifiedName != want.String {
		t.Fatalf("full qualifiedName = %#v, want evaluator value %q", qualifiedName, want.String)
	}
}

func TestFullWriterDoesNotCopyDerivedRedefinedGraphValue(t *testing.T) {
	file := source.New("derived-redefinition.sysml", []byte(`package P {
		occurrence def Owner specializes Occurrences::Occurrence {
			part member;
		}
	}`))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	var member *symbols.Symbol
	for _, candidate := range res.Index().LookupQualified("P::Owner::member") {
		if candidate != nil {
			member = candidate
			break
		}
	}
	if member == nil {
		t.Fatal("resolver did not index P::Owner::member")
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	handle := metamodel.ElementOf(member)
	subject, ok := subjects.byElement[handle.Key()]
	if !ok {
		t.Fatal("P::Owner::member has no graph subject")
	}
	graph.Add(subject, rdf.SysMLTerm("mayTimeVary"), rdf.Bool(false))
	element, err := apiJSONElement(graph, subject)
	if err != nil {
		t.Fatal(err)
	}
	evaluator := metamodel.New(res, encoders[0].ids.model, metamodel.Options{
		Structure: newGraphStructure(graph, subjects),
	})
	element, err = appendFullProperties(graph, subject, element, evaluator, subjects)
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range []string{"isVariable", "mayTimeVary"} {
		value, ok := apiJSONMemberValue(element, property)
		if !ok || value != true {
			t.Errorf("full %s = %#v, present %t; want evaluator-derived true", property, value, ok)
		}
	}
}

func TestFullWriterDoesNotReuseDerivedRedefinitionShortcut(t *testing.T) {
	file := source.New("redefined-multiplicity.sysml", []byte("package P;"))
	root := parser.New(file).ParseFile()
	_, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}

	graph := rdf.NewGraph()
	subject := rdf.ElementIRIForID("membership")
	namespace := rdf.ElementIRIForID("namespace")
	graph.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("OwningMembership"))
	graph.Add(subject, rdf.SysMLTerm("membershipOwningNamespace"), namespace)
	element, err := apiJSONElement(graph, subject)
	if err != nil {
		t.Fatal(err)
	}
	subjects := newSemanticSubjectMap()
	subjects.byIRI[subject.Value] = metamodel.Element{Node: &ast.Relationship{}}
	evaluator := metamodel.New(res, encoders[0].ids.model)
	element, err = appendFullProperties(graph, subject, element, evaluator, subjects)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := apiJSONMemberValue(element, "source"); ok {
		t.Fatal("full API JSON reused derived membershipOwningNamespace as Relationship::source")
	}
}

func TestFullAPIJSONFlowEndsUseSemanticOrigins(t *testing.T) {
	const model = `package P {
		item def Fuel;
		part def Sender { out item outlet : Fuel; }
		part def Receiver { in item inlet : Fuel; }
		part def Context {
			part source : Sender;
			part target : Receiver;
			flow transfer from source.outlet to target.inlet;
		}
	}`
	file := source.New("flow-origins.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	graph, res, encoders, err := modelToRDF([]ModelDocument{{File: file, Root: root}}, IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	context := res.Index().Declaring("P::Context")
	if context == nil || context.Scope == nil {
		t.Fatal("resolver omitted Context")
	}
	var flow *symbols.Symbol
	for _, member := range context.Scope.AllMembers() {
		if kind, ok := member.UsageKind(); ok && kind == ast.UsageFlow {
			flow = member
			break
		}
	}
	if flow == nil {
		t.Fatal("resolver omitted flow usage")
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	evaluator := metamodel.New(res, encoders[0].ids.model, metamodel.Options{
		Structure: newGraphStructure(graph, subjects),
	})
	for _, property := range []string{"flowEnd", "sourceOutputFeature", "targetInputFeature"} {
		value, ok := evaluator.Property(metamodel.ElementOf(flow), "Flow", property)
		if !ok {
			t.Fatalf("Flow::%s is unsupported", property)
		}
		if property == "flowEnd" && len(value.Values) != 2 {
			t.Fatalf("Flow::flowEnd has %d values, want 2", len(value.Values))
		}
		for _, item := range value.Values {
			if item.Kind != metamodel.ElementValue {
				t.Fatalf("Flow::%s item = %#v, want element", property, item)
			}
			if term := subjects.byElement[item.Element.Key()]; term.Value == "" {
				t.Fatalf("Flow::%s handle %#v has no graph subject", property, item.Element)
			}
		}
	}
}
