package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// ReadAPIJSON parses the SysML v2 API's element form — a JSON array of element
// objects, or a single element object — into the same graph ParseTurtle yields:
// the "@type" and "@id" of each object and each metamodel key in document
// order. It is the inverse of WriteAPIJSON, so a graph it builds writes the
// same elements back; the unnamed root Namespace a document wraps its
// top-level elements in is dropped, as the Turtle form does not carry it.
func ReadAPIJSON(data []byte) (*rdf.Graph, error) {
	var references []APIJSONLibraryReference
	expanded, err := expandAPIJSONCompact(data, &references)
	if err != nil {
		return nil, err
	}
	data = expanded
	plan, err := apiJSONExpressionIDs(data)
	if err != nil {
		return nil, err
	}
	graph := plan.graph
	graph.RewriteTriples(func(triple *rdf.Triple) bool {
		if _, synthetic := plan.syntheticRootIndexes[*triple]; synthetic {
			return false
		}
		triple.Subject = apiJSONExpressionTerm(triple.Subject, plan.expressionIDs)
		triple.Object = apiJSONExpressionTerm(triple.Object, plan.expressionIDs)
		return true
	})
	graph = apiJSONGraphWithQualifiedNames(graph, plan.objects, plan.qualifiedNames)
	for _, annotation := range plan.annotations {
		annotation.subject = apiJSONExpressionTerm(annotation.subject, plan.expressionIDs)
		for i := range annotation.members {
			annotation.members[i] = apiJSONExpressionTerm(annotation.members[i], plan.expressionIDs)
		}
		text, err := rdf.CollectionJSON(annotation.subject, annotation.members)
		if err != nil {
			return nil, err
		}
		graph.Add(annotation.subject, rdf.AnnotationJSONTerm(annotation.key), rdf.String(text))
	}
	if len(plan.expressionIDs) > 0 {
		graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	}
	if err := addAPIJSONLibraryReferences(graph, references); err != nil {
		return nil, err
	}
	return withoutRootNamespace(graph), nil
}

func apiJSONExpressionTerm(term rdf.Term, expressionIDs map[string]bool) rdf.Term {
	if id, ok := rdf.SubjectID(term); ok && expressionIDs[id] {
		return rdf.IRI(rdf.Expression + id)
	}
	return term
}

type apiJSONQualifiedNamePosition struct {
	subject   rdf.Term
	predicate string
}

func apiJSONGraphWithQualifiedNames(
	graph *rdf.Graph,
	objects []apiJSONElementData,
	names map[string]string,
) *rdf.Graph {
	if len(names) == 0 {
		return graph
	}
	before := make(map[apiJSONQualifiedNamePosition]string)
	after := make(map[apiJSONQualifiedNamePosition]string)
	afterType := make(map[rdf.Term]string)
	for _, object := range objects {
		name := names[object.id]
		if name == "" {
			continue
		}
		subject := rdf.ReferenceIRI(rdf.Term{}, object.id)
		if hasAddress(graph, subject) {
			continue
		}
		if qualifiedNameIndex := slices.Index(object.mappingPropertyOrder, "qualifiedName"); qualifiedNameIndex >= 0 {
			positioned := false
			for i := qualifiedNameIndex + 1; i < len(object.mappingPropertyOrder); i++ {
				predicate, ok := apiJSONQualifiedNameMemberPredicate(object.mappingPropertyOrder[i])
				if ok && graph.HasProperty(subject, predicate) {
					before[apiJSONQualifiedNamePosition{subject, predicate}] = name
					positioned = true
					break
				}
			}
			if !positioned {
				for i := qualifiedNameIndex - 1; i >= 0; i-- {
					predicate, ok := apiJSONQualifiedNameMemberPredicate(object.mappingPropertyOrder[i])
					if ok && graph.HasProperty(subject, predicate) {
						after[apiJSONQualifiedNamePosition{subject, predicate}] = name
						positioned = true
						break
					}
				}
			}
			if !positioned {
				afterType[subject] = name
			}
			continue
		}
		qualifiedNameIndex := -1
		for i, member := range object.members {
			if member.key == apiJSONMappingQualifiedNameKey {
				qualifiedNameIndex = i
				break
			}
			if qualifiedNameIndex < 0 && member.key == "qualifiedName" {
				qualifiedNameIndex = i
			}
		}
		if qualifiedNameIndex < 0 && object.typ == "ReferenceUsage" &&
			graph.HasProperty(subject, rdf.SysML+"elementId") {
			for i, member := range object.members {
				if member.key == apiJSONMappingValuesKey || member.key == apiJSONMappingQualifiedNameKey ||
					strings.HasPrefix(member.key, "@") {
					continue
				}
				if member.key == "elementId" {
					qualifiedNameIndex = i
				}
				break
			}
		}
		if qualifiedNameIndex >= 0 {
			positioned := false
			for i := qualifiedNameIndex + 1; i < len(object.members); i++ {
				predicate, ok := apiJSONQualifiedNameMemberPredicate(object.members[i].key)
				if ok && graph.HasProperty(subject, predicate) {
					before[apiJSONQualifiedNamePosition{subject, predicate}] = name
					positioned = true
					break
				}
			}
			if !positioned {
				for i := qualifiedNameIndex - 1; i >= 0; i-- {
					predicate, ok := apiJSONQualifiedNameMemberPredicate(object.members[i].key)
					if ok && graph.HasProperty(subject, predicate) {
						after[apiJSONQualifiedNamePosition{subject, predicate}] = name
						positioned = true
						break
					}
				}
			}
			if positioned {
				continue
			}
		}
		afterType[subject] = name
	}
	builder := rdf.NewGraphBuilder(len(graph.Triples()) + len(names))
	for prefix, namespace := range graph.Prefixes {
		builder.SetPrefix(prefix, namespace)
	}
	triples := graph.Triples()
	emitted := make(map[apiJSONQualifiedNamePosition]bool)
	for i, triple := range triples {
		position := apiJSONQualifiedNamePosition{triple.Subject, triple.Predicate.Value}
		if name, ok := before[position]; ok && !emitted[position] {
			builder.Add(triple.Subject, addressTerm(name), rdf.String(name))
			emitted[position] = true
		}
		builder.AddTriple(triple)
		if name, ok := afterType[triple.Subject]; ok && triple.Predicate.Value == rdf.RDFType {
			builder.Add(triple.Subject, addressTerm(name), rdf.String(name))
			delete(afterType, triple.Subject)
		}
		if name, ok := after[position]; ok &&
			(i+1 == len(triples) || triples[i+1].Subject != triple.Subject || triples[i+1].Predicate.Value != triple.Predicate.Value) {
			builder.Add(triple.Subject, addressTerm(name), rdf.String(name))
		}
	}
	return builder.Build()
}

func apiJSONQualifiedNameMemberPredicate(key string) (string, bool) {
	switch {
	case strings.HasPrefix(key, "sysx:"):
		return rdf.OpenSysML + strings.TrimPrefix(key, "sysx:"), true
	case strings.HasPrefix(key, "sysml:"):
		return rdf.SysML + strings.TrimPrefix(key, "sysml:"), true
	case strings.HasPrefix(key, "@"), strings.Contains(key, ":"):
		return "", false
	default:
		return rdf.SysML + key, true
	}
}

func expandAPIJSONCompact(data []byte, references *[]APIJSONLibraryReference) ([]byte, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return data, nil
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(data, &shape); err != nil {
		return data, nil
	}
	var format string
	if err := json.Unmarshal(shape["format"], &format); err != nil || format != CompactAPIJSONFormat {
		return data, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("cannot read the compact API element document")
	}
	document := apiJSONCompactDocument{seen: map[string]bool{}}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("cannot read the compact API element document: %w", err)
		}
		key, ok := token.(string)
		if !ok || document.seen[key] {
			return nil, fmt.Errorf("compact API element document has an invalid or duplicate key %v", token)
		}
		document.seen[key] = true
		if err := document.readField(key, decoder, references); err != nil {
			return nil, err
		}
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("cannot read compact API element document: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("compact API element document holds trailing JSON values")
	}
	return document.expand()
}

// apiJSONCompactDocument is a compact API element document as its fields are read.
type apiJSONCompactDocument struct {
	ids              []string
	elementCount     int
	elementIDsAreIDs bool
	referenceKeys    []string
	elements         []apiJSONObject
	seen             map[string]bool
}

// readField decodes the value of the document field key; the library
// references go to references.
func (document *apiJSONCompactDocument) readField(key string, decoder *json.Decoder, references *[]APIJSONLibraryReference) error {
	switch key {
	case "format":
		var got string
		if err := decoder.Decode(&got); err != nil || got != CompactAPIJSONFormat {
			return fmt.Errorf("compact API element document has an invalid format")
		}
	case "ids":
		if err := decoder.Decode(&document.ids); err != nil {
			return fmt.Errorf("cannot read compact API id table: %w", err)
		}
	case "elementCount":
		if err := decoder.Decode(&document.elementCount); err != nil {
			return fmt.Errorf("cannot read compact API element count: %w", err)
		}
	case "elementIdsAreIds":
		if err := decoder.Decode(&document.elementIDsAreIDs); err != nil {
			return fmt.Errorf("cannot read compact API element id marker: %w", err)
		}
	case "referenceKeys":
		if err := decoder.Decode(&document.referenceKeys); err != nil {
			return fmt.Errorf("cannot read compact API reference keys: %w", err)
		}
	case "libraryReferences":
		var err error
		*references, err = readAPIJSONLibraryReferences(decoder)
		if err != nil {
			return err
		}
	case "elements":
		var err error
		document.elements, err = apiJSONCompactElementsOf(decoder)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("compact API element document has unknown key %q", key)
	}
	return nil
}

// apiJSONCompactElementsOf reads the array of compact element objects.
func apiJSONCompactElementsOf(decoder *json.Decoder) ([]apiJSONObject, error) {
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, fmt.Errorf("compact API elements must be an array")
	}
	var elements []apiJSONObject
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil || token != json.Delim('{') {
			return nil, fmt.Errorf("compact API elements must be objects")
		}
		element, err := apiJSONCompactObjectOf(decoder)
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("cannot read compact API element array: %w", err)
	}
	return elements, nil
}

// expand checks the document and writes its elements as the standard API
// element array, each element given its id from the id table.
func (document *apiJSONCompactDocument) expand() ([]byte, error) {
	seen, ids, elements := document.seen, document.ids, document.elements
	if !seen["format"] || !seen["ids"] || !seen["elementCount"] || !seen["referenceKeys"] || !seen["elements"] {
		return nil, fmt.Errorf("compact API element document is missing a required field")
	}
	if document.elementCount < 0 || document.elementCount != len(elements) || document.elementCount > len(ids) {
		return nil, fmt.Errorf("compact API element document has %d elements, %d element objects, and %d ids",
			document.elementCount, len(elements), len(ids))
	}
	bareReferences := make(map[string]bool, len(document.referenceKeys))
	for _, key := range document.referenceKeys {
		bareReferences[key] = true
	}
	expandedElements := make([]apiJSONObject, len(elements))
	for i, compactElement := range elements {
		for _, member := range compactElement {
			if member.key == "@id" {
				return nil, fmt.Errorf("compact element %d carries an @id instead of using the id table", i)
			}
			if document.elementIDsAreIDs && member.key == "elementId" {
				return nil, fmt.Errorf("compact element %d carries elementId with elementIdsAreIds set", i)
			}
		}
		element := make(apiJSONObject, 0, len(compactElement)+2)
		for _, member := range compactElement {
			value, err := expandAPIJSONCompactValue(member.value, bareReferences[member.key], ids)
			if err != nil {
				return nil, fmt.Errorf("compact element %d property %q: %w", i, member.key, err)
			}
			element = append(element, apiJSONMember{key: member.key, value: value})
		}
		element = append(element, apiJSONMember{key: "@id", value: ids[i]})
		if document.elementIDsAreIDs {
			element = append(element, apiJSONMember{key: "elementId", value: ids[i]})
		}
		expandedElements[i] = element
	}
	expanded, err := json.Marshal(expandedElements)
	if err != nil {
		return nil, fmt.Errorf("cannot expand the compact API element document: %w", err)
	}
	return expanded, nil
}

func apiJSONCompactObjectOf(decoder *json.Decoder) (apiJSONObject, error) {
	var object apiJSONObject
	seen := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("cannot read compact API element: %w", err)
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return nil, fmt.Errorf("compact API element has an invalid or duplicate key %v", token)
		}
		seen[key] = true
		var value any
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("cannot read compact API property %q: %w", key, err)
		}
		object = append(object, apiJSONMember{key: key, value: value})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("cannot read compact API element object: %w", err)
	}
	return object, nil
}

func expandAPIJSONCompactValue(value any, bareReference bool, ids []string) (any, error) {
	switch value := value.(type) {
	case json.Number:
		if !bareReference {
			return value, nil
		}
		id, err := expandAPIJSONCompactReference(value, ids)
		if err != nil {
			return nil, err
		}
		return map[string]any{"@id": id}, nil
	case []any:
		expanded := make([]any, len(value))
		for i, member := range value {
			item, err := expandAPIJSONCompactValue(member, bareReference, ids)
			if err != nil {
				return nil, err
			}
			expanded[i] = item
		}
		return expanded, nil
	case map[string]any:
		if id, ok := value["@id"].(json.Number); ok {
			expandedID, err := expandAPIJSONCompactReference(id, ids)
			if err != nil {
				return nil, err
			}
			return map[string]any{"@id": expandedID}, nil
		}
		expanded := make(map[string]any, len(value))
		for key, member := range value {
			item, err := expandAPIJSONCompactValue(member, false, ids)
			if err != nil {
				return nil, err
			}
			expanded[key] = item
		}
		return expanded, nil
	}
	return value, nil
}

func expandAPIJSONCompactReference(handle json.Number, ids []string) (string, error) {
	index, err := handle.Int64()
	if err != nil || index < 0 || index >= int64(len(ids)) {
		return "", fmt.Errorf("reference handle %q is outside the %d-entry id table", handle, len(ids))
	}
	return ids[index], nil
}

// APIJSONReadError reports a failure to parse an API element document.
type APIJSONReadError struct {
	err error
}

func (e *APIJSONReadError) Error() string {
	if e == nil || e.err == nil {
		return "cannot read the API element document"
	}
	return e.err.Error()
}

func (e *APIJSONReadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// apiJSONElementData is one parsed element object: its identity, its class as
// an expression node, and its properties in written order.
type apiJSONElementData struct {
	id                   string
	typ                  string
	expression           bool
	members              []apiJSONMember
	indexMember          string
	indexOwner           string
	mappingQualifiedName string
	mappingValues        map[string][]any
	mappingValuesOrder   []string
	mappingPropertyOrder []string
	specQualifiedName    string
	hasSpecQualifiedName bool
	toolkitForm          bool
	statedDefaults       bool
	explicitDefaults     map[string]bool
}

var apiJSONInteger = regexp.MustCompile(`^-?[0-9]+$`)

const apiJSONEstimatedTriplesPerElement = 10

type apiJSONImportPlan struct {
	graph                *rdf.Graph
	objects              []apiJSONElementData
	expressionIDs        map[string]bool
	qualifiedNames       map[string]string
	syntheticRootIndexes map[rdf.Triple]struct{}
	annotations          []apiJSONAnnotation
}

// apiJSONExpressionIDs classifies expression nodes and resolves mapping names
// before the imported graph is finalized.
func apiJSONExpressionIDs(data []byte) (*apiJSONImportPlan, error) {
	var objects []apiJSONElementData
	if err := apiJSONElementsOf(data, apiJSONObjectOf, func(object apiJSONElementData) error {
		objects = append(objects, object)
		return nil
	}); err != nil {
		return nil, err
	}
	mappingNames := make(map[string]string)
	specNames := make(map[string]string)
	toolkitForm := false
	statedDefaults := apiJSONStatedDefaults(objects)
	for _, object := range objects {
		if object.mappingQualifiedName != "" {
			mappingNames[object.id] = object.mappingQualifiedName
		}
		if object.hasSpecQualifiedName {
			specNames[object.id] = object.specQualifiedName
		}
		toolkitForm = toolkitForm || (object.toolkitForm && !statedDefaults)
	}
	graph, syntheticRootIndexes, annotations, graphIndex, err := apiJSONMappingGraph(objects)
	if err != nil {
		return nil, err
	}
	mappingGraph := apiJSONEvaluationGraph(graph)
	mappingContext := newMappingQualifiedNameContext(mappingGraph, graphIndex)
	reconstructedNames := mappingQualifiedNamesWithContext(mappingGraph, mappingContext)
	index, err := newAPIJSONIndex(objects)
	if err != nil {
		return nil, err
	}
	index.classifyExpressions(objects, graph, mappingContext.elementSubjects, reconstructedNames, mappingNames, specNames)
	qualifiedNames := make(map[string]string, len(reconstructedNames)+len(mappingNames))
	if toolkitForm {
		qualifiedNames = make(map[string]string)
		for _, object := range objects {
			if name := mappingNames[object.id]; name != "" {
				qualifiedNames[object.id] = name
				continue
			}
			if name := specNames[object.id]; name != "" {
				qualifiedNames[object.id] = name
			}
		}
	} else {
		for _, object := range objects {
			if name := mappingNames[object.id]; name != "" {
				qualifiedNames[object.id] = name
				continue
			}
			subject := rdf.ReferenceIRI(rdf.Term{}, object.id)
			if mappingContext.qualifiedNameOwners[subject.Value].hasOwner {
				if name := reconstructedNames[object.id]; name != "" {
					qualifiedNames[object.id] = name
				}
				continue
			}
			if mappingContext.elementSubjects[subject.Value] {
				if name := specNames[object.id]; name != "" {
					qualifiedNames[object.id] = name
				}
			}
		}
	}
	return &apiJSONImportPlan{
		graph:                graph,
		objects:              objects,
		expressionIDs:        index.expressionIDs,
		qualifiedNames:       qualifiedNames,
		syntheticRootIndexes: syntheticRootIndexes,
		annotations:          annotations,
	}, nil
}

func apiJSONMappingGraph(objects []apiJSONElementData) (*rdf.Graph, map[rdf.Triple]struct{}, []apiJSONAnnotation, *apiJSONGraphIndex, error) {
	builder := rdf.NewGraphBuilder(len(objects) * apiJSONEstimatedTriplesPerElement)
	cache := newAPIJSONTermCache()
	var annotations []apiJSONAnnotation
	statedDefaults := apiJSONStatedDefaults(objects)
	for i := range objects {
		if err := apiJSONMappingObjectTriples(builder, cache, &objects[i], statedDefaults, &annotations); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	graph := builder.Build()
	graph = apiJSONGraphWithPropertyOrder(graph, objects)
	graphIndex := newAPIJSONGraphIndex(graph, nil)
	syntheticRootIndexes := apiJSONSyntheticRootIndexes(graph, graphIndex, objects)
	return graph, syntheticRootIndexes, annotations, graphIndex, nil
}

// apiJSONMappingEmitter writes one mapping element's triples: its members as
// stated, and each mappingValues override once, where the members put it.
type apiJSONMappingEmitter struct {
	builder          *rdf.GraphBuilder
	cache            *apiJSONTermCache
	annotations      *[]apiJSONAnnotation
	object           *apiJSONElementData
	subject          rdf.Term
	typ              rdf.Term
	metaclass        string
	statedDefaults   bool
	memberProperties map[string]bool
	emitted          map[string]bool
}

// apiJSONMappingObjectTriples adds object's type and property triples to builder.
func apiJSONMappingObjectTriples(builder *rdf.GraphBuilder, cache *apiJSONTermCache, object *apiJSONElementData, statedDefaults bool, annotations *[]apiJSONAnnotation) error {
	subject := rdf.ReferenceIRI(rdf.Term{}, object.id)
	metaclass := strings.TrimPrefix(
		strings.TrimPrefix(strings.TrimPrefix(object.typ, "sysx:"), "sysml:"),
		rdf.SysML,
	)
	typ, err := cache.typeIRI(object.typ)
	if err != nil {
		return fmt.Errorf("element %q: %w", object.id, err)
	}
	builder.Add(subject, rdf.IRI(rdf.RDFType), typ)
	emitter := &apiJSONMappingEmitter{
		builder:          builder,
		cache:            cache,
		annotations:      annotations,
		object:           object,
		subject:          subject,
		typ:              typ,
		metaclass:        metaclass,
		statedDefaults:   statedDefaults,
		memberProperties: make(map[string]bool, len(object.members)),
		emitted:          make(map[string]bool, len(object.mappingValues)),
	}
	for _, member := range object.members {
		if member.key != apiJSONMappingQualifiedNameKey && member.key != apiJSONMappingValuesKey {
			emitter.memberProperties[member.key] = true
		}
	}
	for _, member := range object.members {
		if err := emitter.member(member); err != nil {
			return err
		}
	}
	return emitter.remainingMappingValues()
}

// member writes one member: the mappingValues member writes the overrides in
// stated order, an overridden member its override, an implied default nothing.
func (emitter *apiJSONMappingEmitter) member(member apiJSONMember) error {
	object := emitter.object
	switch member.key {
	case apiJSONMappingQualifiedNameKey:
		return nil
	case apiJSONMappingValuesKey:
		return emitter.orderedMappingValues()
	}
	if _, overridden := object.mappingValues[member.key]; overridden {
		return emitter.mappingValues(member.key)
	}
	if member.key == "qualifiedName" {
		return nil
	}
	if emitter.statedDefaults && !object.explicitDefaults[member.key] &&
		apiJSONImpliedBoolean(emitter.typ, member, object.members) {
		return nil
	}
	predicate, sysmlKey, err := emitter.cache.predicate(member.key)
	if err != nil {
		return fmt.Errorf("element %q: %w", object.id, err)
	}
	if err := apiJSONTriples(emitter.builder, apiJSONProperty{
		subject:   emitter.subject,
		predicate: predicate,
		sysmlKey:  sysmlKey,
		metaclass: emitter.metaclass,
	}, member.value, nil, emitter.cache, emitter.annotations); err != nil {
		return fmt.Errorf("element %q: %w", object.id, err)
	}
	return nil
}

// orderedMappingValues writes the overrides in their stated order, except those
// a member of the element places or an earlier pass wrote.
func (emitter *apiJSONMappingEmitter) orderedMappingValues() error {
	for _, property := range emitter.object.mappingValuesOrder {
		if emitter.memberProperties[property] || emitter.emitted[property] {
			continue
		}
		if err := emitter.mappingValues(property); err != nil {
			return err
		}
	}
	return nil
}

// remainingMappingValues writes the overrides no member placed, in name order.
func (emitter *apiJSONMappingEmitter) remainingMappingValues() error {
	properties := make([]string, 0, len(emitter.object.mappingValues))
	for property := range emitter.object.mappingValues {
		properties = append(properties, property)
	}
	sort.Strings(properties)
	for _, property := range properties {
		if emitter.emitted[property] {
			continue
		}
		if err := emitter.mappingValues(property); err != nil {
			return err
		}
	}
	return nil
}

// mappingValues writes the override of property and records it as written.
func (emitter *apiJSONMappingEmitter) mappingValues(property string) error {
	object := emitter.object
	predicate, sysmlKey, err := emitter.cache.predicate(property)
	if err != nil {
		return fmt.Errorf("element %q mappingValues: %w", object.id, err)
	}
	if err := apiJSONTriples(emitter.builder, apiJSONProperty{
		subject:       emitter.subject,
		predicate:     predicate,
		sysmlKey:      sysmlKey,
		mappingValues: true,
		metaclass:     emitter.metaclass,
	}, object.mappingValues[property], nil, emitter.cache, emitter.annotations); err != nil {
		return fmt.Errorf("element %q mappingValues.%s: %w", object.id, property, err)
	}
	emitter.emitted[property] = true
	return nil
}

// apiJSONSyntheticRootIndexes gives every root element without a member index
// one in document order, and returns the triples it added.
func apiJSONSyntheticRootIndexes(graph *rdf.Graph, graphIndex *apiJSONGraphIndex, objects []apiJSONElementData) map[rdf.Triple]struct{} {
	syntheticRootIndexes := make(map[rdf.Triple]struct{})
	rootIndex := 0
	for _, object := range objects {
		subject := rdf.ReferenceIRI(rdf.Term{}, object.id)
		if graph.BoolValue(subject, rdf.SysML+"isLibraryElement") ||
			mappingHasOwnerFact(graph, subject) {
			continue
		}
		if _, hasOwner := mappingQualifiedNameOwner(graph, subject, graphIndex.incomingMemberships, make(map[string]bool)); hasOwner {
			continue
		}
		if mappingTypeConforms(graph, subject, "Namespace") &&
			!graph.HasProperty(subject, rdf.SysML+pDeclaredName) &&
			graph.HasProperty(subject, rdf.SysML+pOwnedRelationship) {
			continue
		}
		if mappingExpressionType(graph, subject) {
			continue
		}
		if !graph.HasProperty(subject, rdf.OpenSysML+"memberIndex") &&
			!mappingTypeConforms(graph, subject, "Relationship") {
			triple := rdf.Triple{
				Subject:   subject,
				Predicate: rdf.OpenSysMLTerm("memberIndex"),
				Object:    rdf.Int(rootIndex),
			}
			graph.AddTriple(triple)
			syntheticRootIndexes[triple] = struct{}{}
		}
		rootIndex++
	}
	return syntheticRootIndexes
}

func apiJSONGraphWithPropertyOrder(graph *rdf.Graph, objects []apiJSONElementData) *rdf.Graph {
	orders := make(map[string]map[string]int)
	for _, object := range objects {
		if len(object.mappingPropertyOrder) == 0 {
			continue
		}
		ranks := make(map[string]int, len(object.mappingPropertyOrder))
		for i, key := range object.mappingPropertyOrder {
			predicate, _, err := apiJSONPredicate(key)
			if err != nil {
				continue
			}
			ranks[predicate.Value] = i
		}
		orders[object.id] = ranks
	}
	if len(orders) == 0 {
		return graph
	}

	builder := rdf.NewGraphBuilder(len(graph.Triples()))
	for prefix, namespace := range graph.Prefixes {
		builder.SetPrefix(prefix, namespace)
	}
	triples := graph.Triples()
	for i := 0; i < len(triples); {
		j := i + 1
		for j < len(triples) && triples[j].Subject == triples[i].Subject {
			j++
		}
		id, _ := rdf.SubjectID(triples[i].Subject)
		ranks := orders[id]
		type predicateGroup struct {
			predicate string
			triples   []rdf.Triple
		}
		groups := make([]predicateGroup, 0, j-i)
		groupIndexes := make(map[string]int, j-i)
		for _, triple := range triples[i:j] {
			predicate := triple.Predicate.Value
			index, ok := groupIndexes[predicate]
			if !ok {
				index = len(groups)
				groupIndexes[predicate] = index
				groups = append(groups, predicateGroup{predicate: predicate})
			}
			groups[index].triples = append(groups[index].triples, triple)
		}
		if ranks != nil {
			sort.SliceStable(groups, func(left, right int) bool {
				leftPredicate := groups[left].predicate
				rightPredicate := groups[right].predicate
				if leftPredicate == rdf.RDFType {
					return rightPredicate != rdf.RDFType
				}
				if rightPredicate == rdf.RDFType {
					return false
				}
				leftRank, leftListed := ranks[leftPredicate]
				rightRank, rightListed := ranks[rightPredicate]
				if leftListed != rightListed {
					return leftListed
				}
				return leftListed && leftRank < rightRank
			})
		}
		for _, group := range groups {
			for _, triple := range group.triples {
				builder.AddTriple(triple)
			}
		}
		i = j
	}
	return builder.Build()
}

// apiJSONElementsOf visits each element object in either supported document
// shape, rejecting trailing JSON values.
func apiJSONElementsOf(data []byte, read func(*json.Decoder) (apiJSONElementData, error), visit func(apiJSONElementData) error) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	start, err := dec.Token()
	if err != nil {
		return fmt.Errorf("cannot read the API element document: %w", err)
	}
	switch start {
	case json.Delim('['):
		for dec.More() {
			if token, err := dec.Token(); err != nil || token != json.Delim('{') {
				return fmt.Errorf("an API element array holds element objects, not %v", token)
			}
			object, err := read(dec)
			if err != nil {
				return err
			}
			if err := visit(object); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil {
			return fmt.Errorf("cannot read the API element array: %w", err)
		}
	case json.Delim('{'):
		object, err := read(dec)
		if err != nil {
			return err
		}
		if err := visit(object); err != nil {
			return err
		}
	default:
		return fmt.Errorf("an API element document is an array of element objects or a single object, not %v", start)
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("the API element document holds more than one JSON value")
	}
	return nil
}

// apiJSONIndex is what classifying the expression namespace reads across the
// document: every id and its metaclass, the owner and membership metaclass of
// each node a membership states as its member, the owner of each
// relationship, and the ids classified so far.
type apiJSONIndex struct {
	ids               map[string]bool
	types             map[string]string
	nodeOwner         map[string]string
	nodeOwnerMeta     map[string]string
	relationshipOwner map[string]string
	expressionIDs     map[string]bool
}

func newAPIJSONIndex(objects []apiJSONElementData) (*apiJSONIndex, error) {
	index := &apiJSONIndex{
		ids:               map[string]bool{},
		types:             map[string]string{},
		nodeOwner:         map[string]string{},
		nodeOwnerMeta:     map[string]string{},
		relationshipOwner: map[string]string{},
		expressionIDs:     map[string]bool{},
	}
	subjects := map[string]string{}
	for _, object := range objects {
		if err := apiJSONIdentity(object); err != nil {
			return nil, err
		}
		if index.ids[object.id] {
			return nil, fmt.Errorf("the id %q names two element objects", object.id)
		}
		index.ids[object.id] = true
		index.types[object.id] = object.typ
		resolved := rdf.ReferenceIRI(rdf.Term{}, object.id).Value
		if earlier, seen := subjects[resolved]; seen {
			return nil, fmt.Errorf("the ids %q and %q name the same element", earlier, object.id)
		}
		subjects[resolved] = object.id
	}
	// Opaque ids such as UUIDs carry no parent, so the membership that states
	// the node as its member stands in.
	for _, object := range objects {
		member, owner := object.indexMember, object.indexOwner
		if owner != "" && isRelationship(object.typ) {
			index.relationshipOwner[object.id] = owner
		}
		if member != "" && owner != "" && nodeMembershipMetaclass(object.typ) {
			index.nodeOwner[member] = owner
			index.nodeOwnerMeta[member] = object.typ
		}
	}
	return index, nil
}

// apiJSONIdentity checks an element object's "@id" and "@type".
func apiJSONIdentity(object apiJSONElementData) error {
	switch {
	case object.id == "":
		return fmt.Errorf("an element object needs a non-empty string \"@id\"")
	case strings.HasPrefix(object.id, ":"):
		return fmt.Errorf("the id %q has an empty scope qualifier; an element's own id names its scope or none", object.id)
	case object.typ == "":
		return fmt.Errorf("element %q needs a string \"@type\"", object.id)
	}
	return nil
}

// classifyExpressions propagates expression classification from parent nodes,
// including when a child appears before its parent in the document.
func (index *apiJSONIndex) classifyExpressions(
	objects []apiJSONElementData,
	graph *rdf.Graph,
	mappingElements map[string]bool,
	reconstructedNames, mappingNames map[string]string,
	specNames map[string]string,
) {
	mappingElement := make([]bool, len(objects))
	dependents := make(map[string][]int, len(objects))
	for i := range objects {
		object := &objects[i]
		subject := rdf.ReferenceIRI(rdf.Term{}, object.id)
		isMappingElement := mappingElements[subject.Value]
		hasMappingFacts := mappingHasIdentity(graph, subject) ||
			mappingHasOwnerFact(graph, subject) ||
			mappingNames[object.id] != ""
		isMembership := mappingTypeConforms(graph, subject, "Membership")
		_, hasSpecName := specNames[object.id]
		mappingElement[i] = isMappingElement && !isMembership && hasMappingFacts
		if !isMembership && (mappingNames[object.id] != "" ||
			(hasSpecName && reconstructedNames[object.id] == "" && !mappingHasOwnerFact(graph, subject))) {
			mappingElement[i] = true
		}

		if base, membership := strings.CutSuffix(object.id, rdf.OwningMembershipSuffix); membership {
			dependents[base] = append(dependents[base], i)
		}
		if owner, ok := rdf.ExpressionNodeOwner(object.id, func(prefix string) bool { return index.ids[prefix] }); ok {
			dependents[owner] = append(dependents[owner], i)
		}
		if owner := index.nodeOwner[object.id]; owner != "" {
			dependents[owner] = append(dependents[owner], i)
		}
		if owner := index.relationshipOwner[object.id]; owner != "" {
			dependents[owner] = append(dependents[owner], i)
		}
	}

	queue := make([]int, 0, len(objects))
	queued := make([]bool, len(objects))
	for i := range objects {
		if mappingElement[i] || !index.expression(&objects[i]) {
			continue
		}
		objects[i].expression = true
		index.expressionIDs[objects[i].id] = true
		queue = append(queue, i)
		queued[i] = true
	}
	for head := 0; head < len(queue); head++ {
		parent := queue[head]
		for _, child := range dependents[objects[parent].id] {
			if queued[child] || mappingElement[child] || !index.expression(&objects[child]) {
				continue
			}
			objects[child].expression = true
			index.expressionIDs[objects[child].id] = true
			queue = append(queue, child)
			queued[child] = true
		}
	}
}

// expression reports whether the object is classified an expression node by
// what is classified so far.
func (index *apiJSONIndex) expression(object *apiJSONElementData) bool {
	ids, types, expressionIDs := index.ids, index.types, index.expressionIDs
	if base, isMembership := strings.CutSuffix(object.id, rdf.OwningMembershipSuffix); isMembership {
		return ids[base] && expressionIDs[base]
	}
	if owner, ok := rdf.ExpressionNodeOwner(object.id, func(prefix string) bool { return ids[prefix] }); ok {
		return expressionIDs[owner] || isExpressionRoot(object.typ) || expressionMetaclasses[types[owner]] ||
			messageParameterEnd(types[index.nodeOwner[object.id]], index.nodeOwnerMeta[object.id], object.typ)
	}
	if object.typ != mMembership && expressionMetaclasses[object.typ] {
		// A Membership carries a referent element, not an owned node;
		// only an owning membership marks one a node.
		return true
	}
	if owner := index.nodeOwner[object.id]; owner != "" {
		return expressionIDs[owner] || expressionMetaclasses[types[owner]] ||
			messageParameterEnd(types[owner], index.nodeOwnerMeta[object.id], object.typ)
	}
	if owner := index.relationshipOwner[object.id]; owner != "" && expressionIDs[owner] {
		// A referent Membership is a node; other owned relationships are
		// nodes only while their id derives from the node's (qualified form).
		return object.typ == mMembership || strings.HasPrefix(object.id, owner+"_")
	}
	return false
}

// messageParameterEnd reports whether an element is a `message` end: an
// EventOccurrenceUsage a FlowUsage owns through a ParameterMembership
// (SysML.xtext MessageEventMember), a part of the flow's head. A declared
// `event occurrence` usage is never a node — its owner is a namespace, not a
// FlowUsage, so the metaclass alone decides nothing here.
func messageParameterEnd(ownerType, membershipType, memberType string) bool {
	return ownerType == "FlowUsage" && membershipType == mParameterMembership && memberType == mEventOccurrenceUsage
}

// nodeMembershipMetaclass reports whether an element of this metaclass can
// own an expression node as its member: a membership family class or a
// feature value.
func nodeMembershipMetaclass(metaclass string) bool {
	return metaclass != mMembership && (metaclass == mFeatureValue || strings.HasSuffix(metaclass, "Membership"))
}

// membershipMemberProperty reports whether a property names the member a
// membership owns.
func membershipMemberProperty(key string) bool {
	switch key {
	case "memberElement", "ownedMemberElement", "ownedMemberFeature", "ownedMemberParameter",
		"ownedMember", "ownedRelatedElement", "ownedResultExpression", "member", "memberFeature":
		return true
	}
	return false
}

// membershipOwnerProperty reports whether a property names the element a
// membership sits under.
func membershipOwnerProperty(key string) bool {
	switch key {
	case "owningRelatedElement", "membershipOwningNamespace", "owner", "featureWithValue":
		return true
	}
	return false
}

// memberReference reads the id of an {"@id": "…"} member value.
func memberReference(value any) (string, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	id, ok := object["@id"].(string)
	return id, ok
}

// apiJSONObjectOf reads one '{...}' from the decoder — its '{' already consumed
// — returning "@type"/"@id" extracted and the other members in written order.
func apiJSONObjectOf(dec *json.Decoder) (apiJSONElementData, error) {
	var object apiJSONElementData
	seen := map[string]bool{}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return object, err
		}
		key, ok := token.(string)
		if !ok {
			return object, fmt.Errorf("an element object's member key is a string, not %v", token)
		}
		if seen[key] {
			return object, fmt.Errorf("element object states %q twice", key)
		}
		seen[key] = true
		if key == apiJSONMappingValuesKey {
			object.mappingValues, object.mappingValuesOrder, err = apiJSONMappingValuesOf(dec)
			if err != nil {
				return object, err
			}
			object.members = append(object.members, apiJSONMember{key: key})
			continue
		}
		value, err := apiJSONValueOf(dec)
		if err != nil {
			return object, fmt.Errorf("%s: %w", key, err)
		}
		if key == apiJSONMappingPropertyOrderKey {
			object.mappingPropertyOrder, err = apiJSONMappingPropertyOrderOf(value)
			if err != nil {
				return object, fmt.Errorf("%s: %w", key, err)
			}
			continue
		}
		if key == apiJSONStatedDefaultsKey {
			if object.statedDefaults, ok = value.(bool); !ok {
				return object, fmt.Errorf("%s is a boolean, not %v", key, value)
			}
			continue
		}
		if key == apiJSONExplicitDefaultsKey {
			names, ok := value.([]any)
			if !ok {
				return object, fmt.Errorf("%s is an array of property names, not %v", key, value)
			}
			object.explicitDefaults = make(map[string]bool, len(names))
			for _, name := range names {
				text, ok := name.(string)
				if !ok {
					return object, fmt.Errorf("%s names a property with a string, not %v", key, name)
				}
				object.explicitDefaults[text] = true
			}
			continue
		}
		switch {
		case key == "@type":
			text, ok := value.(string)
			if !ok {
				return object, fmt.Errorf("\"@type\" is a string, not %v", value)
			}
			object.typ = text
		case key == "@id":
			text, ok := value.(string)
			if !ok {
				return object, fmt.Errorf("\"@id\" is a string, not %v", value)
			}
			object.id = text
		case strings.HasPrefix(key, "@"):
			return object, fmt.Errorf("the key %q is not a keyword this document carries", key)
		default:
			switch key {
			case "qualifiedName":
				if name, ok := value.(string); ok && name != "" {
					object.specQualifiedName = mappingQualifiedNameFromSpec(name)
					object.hasSpecQualifiedName = true
				}
			case apiJSONMappingQualifiedNameKey:
				if name, ok := value.(string); ok && name != "" {
					object.mappingQualifiedName = name
				}
			case "isImpliedIncluded":
				// Only an external writer's element form states this flag: the
				// mapping's graph never does, and its own documents carry the marker.
				object.toolkitForm = true
			}
			if id, ok := memberReference(value); ok {
				if membershipMemberProperty(key) {
					object.indexMember = id
				}
				if membershipOwnerProperty(key) {
					object.indexOwner = id
				}
			}
			object.members = append(object.members, apiJSONMember{key: key, value: value})
		}
	}
	if _, err := dec.Token(); err != nil {
		return object, err
	}
	return object, nil
}

func apiJSONMappingPropertyOrderOf(value any) ([]string, error) {
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be an array")
	}
	order := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for i, value := range values {
		key, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("property %d is not a string", i)
		}
		if key == "" || strings.HasPrefix(key, "@") {
			return nil, fmt.Errorf("property %d has invalid name %q", i, key)
		}
		if seen[key] {
			return nil, fmt.Errorf("property %q occurs more than once", key)
		}
		if _, _, err := apiJSONPredicate(key); err != nil {
			return nil, fmt.Errorf("property %d: %w", i, err)
		}
		seen[key] = true
		order = append(order, key)
	}
	return order, nil
}

func apiJSONMappingValuesOf(dec *json.Decoder) (map[string][]any, []string, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if token != json.Delim('{') {
		return nil, nil, fmt.Errorf("\"sysx:mappingValues\" must be a non-empty property map")
	}
	values := make(map[string][]any)
	var order []string
	for dec.More() {
		propertyToken, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		property, ok := propertyToken.(string)
		if !ok {
			return nil, nil, fmt.Errorf("\"sysx:mappingValues\" property key is a string, not %v", propertyToken)
		}
		if _, seen := values[property]; seen {
			return nil, nil, fmt.Errorf("\"sysx:mappingValues\" states property %q twice", property)
		}
		if property == "" || strings.Contains(property, ":") {
			return nil, nil, fmt.Errorf("\"sysx:mappingValues\" has invalid property %q", property)
		}
		value, err := apiJSONValueOf(dec)
		if err != nil {
			return nil, nil, err
		}
		list, ok := value.([]any)
		if !ok || len(list) == 0 {
			return nil, nil, fmt.Errorf("\"sysx:mappingValues\" property %q must be a non-empty array", property)
		}
		for _, item := range list {
			switch item := item.(type) {
			case string:
				values[property] = append(values[property], item)
			case map[string]any:
				if len(item) != 1 {
					return nil, nil, fmt.Errorf("\"sysx:mappingValues\" property %q has an invalid object value", property)
				}
				id, ok := item["@id"].(string)
				if !ok || id == "" {
					return nil, nil, fmt.Errorf("\"sysx:mappingValues\" property %q must use a non-empty @id", property)
				}
				values[property] = append(values[property], item)
			default:
				return nil, nil, fmt.Errorf("\"sysx:mappingValues\" property %q values must be strings or @id objects", property)
			}
		}
		order = append(order, property)
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	if len(values) == 0 {
		return nil, nil, fmt.Errorf("\"sysx:mappingValues\" must be a non-empty property map")
	}
	return values, order, nil
}

// apiJSONValueOf reads one value from the decoder — the same shapes a whole
// document would decode to — refusing a repeated key in any object it builds.
func apiJSONValueOf(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := map[string]any{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("an object member key is a string, not %v", keyToken)
			}
			if _, seen := object[key]; seen {
				return nil, fmt.Errorf("object states %q twice", key)
			}
			value, err := apiJSONValueOf(dec)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			object[key] = value
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return object, nil
	case json.Delim('['):
		array := []any{}
		for dec.More() {
			value, err := apiJSONValueOf(dec)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		if _, err := dec.Token(); err != nil {
			return nil, err
		}
		return array, nil
	}
	return token, nil
}

// apiJSONTypeIRI maps a "@type" to its metaclass IRI: a bare name is a sysml:
// class, the sysx: CURIE an extension class; anything else cannot be named.
func apiJSONTypeIRI(typ string) (rdf.Term, error) {
	if name, ok := strings.CutPrefix(typ, "sysx:"); ok {
		if name == "" {
			return rdf.Term{}, fmt.Errorf("the \"@type\" %q has no metaclass name", typ)
		}
		return rdf.OpenSysMLTerm(name), nil
	}
	if strings.Contains(typ, ":") {
		return rdf.Term{}, fmt.Errorf("the \"@type\" %q carries a prefix this mapping does not know", typ)
	}
	return rdf.SysMLTerm(typ), nil
}

// apiJSONPredicate maps a member key to its predicate IRI, reporting the sysml:
// local name for the collection annotation it may carry.
func apiJSONPredicate(key string) (rdf.Term, string, error) {
	if name, ok := strings.CutPrefix(key, "sysx:"); ok {
		if name == "" {
			return rdf.Term{}, "", fmt.Errorf("the key %q has no property name", key)
		}
		return rdf.OpenSysMLTerm(name), "", nil
	}
	if strings.Contains(key, ":") {
		return rdf.Term{}, "", fmt.Errorf("the key %q carries a prefix this mapping does not know", key)
	}
	return rdf.SysMLTerm(key), key, nil
}

type apiJSONPredicateValue struct {
	term rdf.Term
	key  string
}

type apiJSONReferenceKey struct {
	scope string
	id    string
}

type apiJSONTermCache struct {
	types       map[string]rdf.Term
	predicates  map[string]apiJSONPredicateValue
	references  map[apiJSONReferenceKey]rdf.Term
	literals    map[string]string
	collections map[apiJSONCollectionKey][]rdf.CollectionMember
}

type apiJSONCollectionKey struct {
	subject rdf.Term
	key     string
}

func newAPIJSONTermCache() *apiJSONTermCache {
	return &apiJSONTermCache{
		types:       map[string]rdf.Term{},
		predicates:  map[string]apiJSONPredicateValue{},
		references:  map[apiJSONReferenceKey]rdf.Term{},
		literals:    map[string]string{},
		collections: map[apiJSONCollectionKey][]rdf.CollectionMember{},
	}
}

func (cache *apiJSONTermCache) collectionMembers(
	graph *rdf.Graph,
	subject rdf.Term,
	key string,
) ([]rdf.CollectionMember, error) {
	if cache.collections == nil {
		cache.collections = make(map[apiJSONCollectionKey][]rdf.CollectionMember)
	}
	cacheKey := apiJSONCollectionKey{subject: subject, key: key}
	if members, ok := cache.collections[cacheKey]; ok {
		return members, nil
	}
	annotation, ok := graph.Object(subject, rdf.AnnotationJSON+key)
	if !ok || !annotation.IsLiteral() {
		return nil, nil
	}
	members, err := rdf.ParseCollectionJSON(annotation.Value)
	if err != nil {
		return nil, err
	}
	cache.collections[cacheKey] = members
	return members, nil
}

func (cache *apiJSONTermCache) literal(value string) string {
	if cache.literals == nil {
		return value
	}
	if stored, ok := cache.literals[value]; ok {
		return stored
	}
	cache.literals[value] = value
	return value
}

func (cache *apiJSONTermCache) typeIRI(typ string) (rdf.Term, error) {
	if term, ok := cache.types[typ]; ok {
		return term, nil
	}
	term, err := apiJSONTypeIRI(typ)
	if err == nil {
		cache.types[typ] = term
	}
	return term, err
}

func (cache *apiJSONTermCache) predicate(key string) (rdf.Term, string, error) {
	if value, ok := cache.predicates[key]; ok {
		return value.term, value.key, nil
	}
	term, local, err := apiJSONPredicate(key)
	if err == nil {
		cache.predicates[key] = apiJSONPredicateValue{term: term, key: local}
	}
	return term, local, err
}

// referenceTarget resolves one JSON id or unresolved name to its graph term.
func (cache *apiJSONTermCache) referenceTarget(subject rdf.Term, object map[string]any, expressionIDs map[string]bool) (rdf.Term, error) {
	if len(object) == 1 {
		if id, ok := object["@id"].(string); ok && id != "" {
			key := apiJSONReferenceKey{id: id}
			if !strings.Contains(id, ":") {
				if owner, ok := rdf.SubjectID(subject); ok {
					key.scope, _, _ = strings.Cut(owner, ":")
				}
			}
			if target, ok := cache.references[key]; ok {
				return target, nil
			}
			if strings.HasPrefix(id, rdf.Expression) || strings.HasPrefix(id, rdf.Element) {
				// A service that hands a reference back as the IRI itself names
				// the element the IRI does.
				target := rdf.IRI(id)
				cache.references[key] = target
				return target, nil
			}
			target := rdf.ReferenceIRI(subject, id)
			if targetID, ok := rdf.SubjectID(target); ok && expressionIDs[targetID] {
				target = rdf.IRI(rdf.Expression + targetID)
			}
			cache.references[key] = target
			return target, nil
		}
		if name, ok := object["@ref"].(string); ok && name != "" {
			return rdf.String(cache.literal(name)), nil
		}
	}
	return rdf.Term{}, fmt.Errorf("an object value is a reference {\"@id\": <id>} or {\"@ref\": <name>}")
}

// apiJSONAnnotation is a collection awaiting its json: literal: the members
// a sysml: array stated, settled once every triple stands.
type apiJSONAnnotation struct {
	subject rdf.Term
	key     string
	members []rdf.Term
}

// apiJSONTriples states one property of an element object as graph triples.
// apiJSONProperty is one property of an element object: the element, the
// predicate the property maps to and the sysml: key it was served under.
type apiJSONProperty struct {
	subject, predicate rdf.Term
	sysmlKey           string
	metaclass          string
	mappingValues      bool
}

func apiJSONTriples(graph *rdf.GraphBuilder, p apiJSONProperty, value any, expressionIDs map[string]bool, cache *apiJSONTermCache, annotations *[]apiJSONAnnotation) error {
	subject, predicate := p.subject, p.predicate
	switch v := value.(type) {
	case nil:
		// The API serves absent properties as null; nothing to state.
		return nil
	case map[string]any:
		target, err := cache.referenceTarget(subject, v, expressionIDs)
		if err != nil {
			return err
		}
		graph.Add(subject, predicate, target)
		return nil
	case []any:
		return apiJSONCollection(graph, p, v, expressionIDs, cache, annotations)
	default:
		object, err := apiJSONPropertyScalar(p, value, expressionIDs, cache)
		if err != nil {
			return err
		}
		graph.Add(subject, predicate, object)
		return nil
	}
}

// apiJSONCollection states an array member: one triple per value, recording
// a collection of at least two on a sysml: key for its json: annotation.
func apiJSONCollection(graph *rdf.GraphBuilder, p apiJSONProperty, values []any, expressionIDs map[string]bool, cache *apiJSONTermCache, annotations *[]apiJSONAnnotation) error {
	subject, predicate, sysmlKey := p.subject, p.predicate, p.sysmlKey
	if len(values) == 0 {
		return nil
	}
	members := make([]rdf.Term, 0, len(values))
	for _, value := range values {
		member, err := apiJSONPropertyScalar(p, value, expressionIDs, cache)
		if err != nil {
			return err
		}
		members = append(members, member)
	}
	name := sysmlKey
	if name == "" {
		name = rdf.LocalName(predicate.Value)
	}
	for i, member := range members {
		for _, earlier := range members[:i] {
			if !member.Equal(earlier) {
				continue
			}
			property, known := ontology.PropertyOf(p.metaclass, name)
			if known && property.Nonunique {
				// A derived {nonunique} list repeats a member the triples hold once.
				break
			}
			return fmt.Errorf("the array on %s of <%s> repeats the member %s", name, subject.Value, member)
		}
		graph.Add(subject, predicate, member)
	}
	// Like AnnotateCollections, only a collection of two or more is annotated.
	if sysmlKey != "" && len(members) >= 2 {
		*annotations = append(*annotations, apiJSONAnnotation{subject: subject, key: sysmlKey, members: members})
	}
	return nil
}

func apiJSONPropertyScalar(
	p apiJSONProperty,
	value any,
	expressionIDs map[string]bool,
	cache *apiJSONTermCache,
) (rdf.Term, error) {
	if p.mappingValues {
		if text, ok := value.(string); ok {
			if property, known := ontology.PropertyOf(p.metaclass, p.sysmlKey); known &&
				property.Kind == ontology.DatatypeProperty {
				switch property.Range {
				case rdf.XSD + "boolean", rdf.XSD + "byte", rdf.XSD + "short",
					rdf.XSD + "int", rdf.XSD + "long", rdf.XSD + "integer":
					return rdf.TypedLiteral(text, property.Range), nil
				case rdf.XSD + "decimal", "http://www.w3.org/2002/07/owl#real":
					return rdf.TypedLiteral(text, rdf.XSD+"decimal"), nil
				}
			}
			return rdf.String(text), nil
		}
	}
	return apiJSONScalarOf(p.subject, p.predicate, p.sysmlKey, value, expressionIDs, cache)
}

// apiJSONScalarOf turns one JSON value into the term a triple holds: a
// reference object an IRI, a bool or number its typed literal, a string a plain
// literal — or the expression text a reference-valued property carries.
func apiJSONScalarOf(subject rdf.Term, predicate rdf.Term, sysmlKey string, value any, expressionIDs map[string]bool, cache *apiJSONTermCache) (rdf.Term, error) {
	switch v := value.(type) {
	case bool:
		return rdf.Bool(v), nil
	case json.Number:
		lexical := cache.literal(v.String())
		if apiJSONInteger.MatchString(lexical) {
			return rdf.TypedLiteral(lexical, rdf.XSD+"integer"), nil
		}
		return exactRealLiteral(lexical)
	case string:
		v = cache.literal(v)
		if apiJSONIsExpressionText(sysmlKey, v) {
			return rdf.TypedLiteral(v, rdf.OpenSysML+dtExpression), nil
		}
		return rdf.String(v), nil
	case map[string]any:
		return cache.referenceTarget(subject, v, expressionIDs)
	case nil:
		return rdf.Term{}, fmt.Errorf("a collection member cannot be null")
	case []any:
		return rdf.Term{}, fmt.Errorf("a collection member cannot be an array")
	}
	return rdf.Term{}, fmt.Errorf("a member cannot be %v", value)
}

// apiJSONIsExpressionText says whether a string on a sysml: key is expression
// text, as the encoder decided it: only a target on an expression-capable
// property that does not parse as a name is the text it was written as.
func apiJSONIsExpressionText(key, text string) bool {
	if !apiJSONExpressionKeys[key] {
		return false
	}
	p := parser.New(source.New("<naming>", []byte(text)))
	switch p.ParseExpression().(type) {
	case *ast.FeatureReference, *ast.QualifiedName:
		return false
	}
	return true
}

// apiJSONExpressionKeys is the set of sysml: properties the encoder writes
// expression text on when a target is not a name: the head-relationship
// targets, the relationship-element ends and the reference subsetting's
// target. Every other string is a name literal, quoted in the notation.
var apiJSONExpressionKeys = func() map[string]bool {
	keys := map[string]bool{}
	for _, property := range relationshipProperty {
		keys[property] = true
	}
	for _, form := range relationshipElementForm {
		keys[form.source] = true
		keys[form.target] = true
	}
	keys[conjugationForm.source] = true
	keys[conjugationForm.target] = true
	for _, property := range []string{pReferencedFeature, pSubsettedFeature, pGeneral, pTarget, pRelatedElement} {
		keys[property] = true
	}
	return keys
}()

// apiJSONStatedDefaults reports whether the document was written by this
// mapping with every non-derived boolean stated (apiJSONStatedDefaultsKey).
func apiJSONStatedDefaults(objects []apiJSONElementData) bool {
	for _, object := range objects {
		if object.statedDefaults {
			return true
		}
	}
	return false
}
