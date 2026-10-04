package export

import (
	"encoding/json"
	"reflect"
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
	owned, known := structure.OwnedRelationships(annotatingHandle)
	if !known || len(owned) != 1 || owned[0].Key() != annotationHandle.Key() {
		t.Errorf("AnnotatingElement owned relationships = %#v, %t; want indexed Annotation", owned, known)
	}
}

func TestFullAPIJSONAnnotationOppositeReferencesRelationship(t *testing.T) {
	const model = `package P {
		part def A;
		part x : A;
		#refinement dependency x to x;
	}`
	file := source.New("annotation-opposite.sysml", []byte(model))
	root := parser.New(file).ParseFile()
	output, err := ModelToAPIJSON([]ModelDocument{{File: file, Root: root}}, IDQualifiedName, APIJSONFull)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]json.RawMessage
	if err := json.Unmarshal(output, &elements); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]map[string]json.RawMessage, len(elements))
	for _, element := range elements {
		var id string
		if err := json.Unmarshal(element["@id"], &id); err != nil {
			t.Fatal(err)
		}
		byID[id] = element
	}
	for _, element := range elements {
		var metaclass string
		if err := json.Unmarshal(element["@type"], &metaclass); err != nil || metaclass != "MetadataUsage" {
			continue
		}
		var metadataID string
		if err := json.Unmarshal(element["@id"], &metadataID); err != nil {
			t.Fatal(err)
		}
		rawAnnotations, ok := element["annotation"]
		if !ok {
			continue
		}
		var annotations []struct {
			ID string `json:"@id"`
		}
		if err := json.Unmarshal(rawAnnotations, &annotations); err != nil {
			t.Fatalf("%s annotation: %v", metadataID, err)
		}
		if len(annotations) == 0 {
			continue
		}
		if len(annotations) != 1 || annotations[0].ID == metadataID {
			t.Fatalf("%s annotation = %#v, want its Annotation relationship", metadataID, annotations)
		}
		annotation, ok := byID[annotations[0].ID]
		if !ok {
			t.Fatalf("annotation relationship %s has no subject", annotations[0].ID)
		}
		var annotationType string
		if err := json.Unmarshal(annotation["@type"], &annotationType); err != nil || annotationType != "Annotation" {
			t.Fatalf("annotation relationship %s has type %q, want Annotation", annotations[0].ID, annotationType)
		}
		return
	}
	t.Fatal("full API JSON has no MetadataUsage with an Annotation relationship")
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
	owningType, ok := apiJSONMemberValue(element, "owningType")
	if !ok {
		t.Fatal("full API JSON omitted the redefined owningType property")
	}
	if owningType != (apiJSONReference{ID: "feature"}) {
		t.Fatalf("Specialization::owningType = %#v, want the graph-stated FeatureTyping::owningFeature", owningType)
	}
}

func TestFullWriterPreservesMultiplicityForRedefinedProperty(t *testing.T) {
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
	source, ok := apiJSONMemberValue(element, "source")
	if !ok {
		t.Fatal("full API JSON omitted the redefined source property")
	}
	want := []any{apiJSONReference{ID: "namespace"}}
	if !reflect.DeepEqual(source, want) {
		t.Fatalf("Relationship::source = %#v, want %#v", source, want)
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
