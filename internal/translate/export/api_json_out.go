package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// APIJSONOptions configures diagnostics collected while writing API JSON.
type APIJSONOptions struct {
	Omitted    func(APIJSONOmission)
	Diagnostic func(string)
}

const (
	apiJSONMappingPropertyOrderKey = "sysx:mappingPropertyOrder"
	apiJSONMappingQualifiedNameKey = "sysx:mappingQualifiedName"
	apiJSONMappingValuesKey        = "sysx:mappingValues"
)

// APIJSONOmission describes an object-property value omitted because its target
// could not be resolved.
type APIJSONOmission struct {
	Subject   string
	Metaclass string
	Property  string
	Reason    metamodel.Reason
}

// WriteAPIJSON serializes a graph as the SysML v2 API's element form: a JSON
// array of objects carrying "@type", "@id" and the metamodel properties as
// keys, with collection values in annotation order.
func WriteAPIJSON(graph *rdf.Graph) ([]byte, error) {
	return WriteAPIJSONWithOptions(graph, APIJSONOptions{})
}

// WriteAPIJSONWithOptions writes API JSON and reports unresolved object
// properties through options.
func WriteAPIJSONWithOptions(graph *rdf.Graph, options APIJSONOptions) ([]byte, error) {
	elements, err := apiJSONElementsWithOptions(graph, options)
	if err != nil {
		return nil, err
	}
	// Written compact in one pass and indented once. Encoding the array through
	// each object's MarshalJSON had the encoder parse and re-indent every
	// object it returned, which cost more than building the elements.
	var compact bytes.Buffer
	w := apiJSONWriter{buf: &compact, enc: json.NewEncoder(&compact)}
	compact.WriteByte('[')
	for i, element := range elements {
		if i > 0 {
			compact.WriteByte(',')
		}
		if err := w.object(element); err != nil {
			return nil, err
		}
	}
	compact.WriteByte(']')
	var out bytes.Buffer
	out.Grow(compact.Len() * 2)
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// apiJSONElements builds the element object of every subject of the graph,
// once its collections are settled and the root Namespace wraps it.
func apiJSONElements(graph *rdf.Graph) ([]apiJSONObject, error) {
	return apiJSONElementsWithOptions(graph, APIJSONOptions{})
}

func apiJSONElementsWithOptions(graph *rdf.Graph, options APIJSONOptions) ([]apiJSONObject, error) {
	wrapped, err := withRootNamespace(graph)
	if err != nil {
		return nil, err
	}
	settled, err := rdf.ReconcileCollectionsWithPolicy(wrapped, func(subject rdf.Term, key string) bool {
		metaclass := strings.TrimPrefix(wrapped.Type(subject), rdf.SysML)
		property, ok := ontology.PropertyOf(metaclass, key)
		return ok && property.Nonunique
	})
	if err != nil {
		return nil, err
	}
	subjects := settled.SubjectsView()
	elements := make([]apiJSONObject, 0, len(subjects))
	subjectSlots := make(map[string]int, len(subjects))
	for position, subject := range subjects {
		subjectSlots[subject.Value] = position
	}
	evaluation := newAPIJSONEvaluation(settled, subjectSlots)
	mappingNames := newAPIJSONMappingNameIndex(evaluation.structure.graph, subjectSlots)
	nameIndex := newAPIJSONNameIndexWithSubjectSlots(settled, subjectSlots)
	writer := &apiJSONElementWriter{
		graph:              settled,
		evaluation:         evaluation,
		mappingNames:       mappingNames,
		nameIndex:          nameIndex,
		comparisonCache:    &apiJSONTermCache{},
		propertyOrderGraph: wrapped,
		options:            options,
		scratch:            &apiJSONWriteScratch{},
	}
	for _, subject := range subjects {
		// The element form holds the model's own elements only; a library element
		// stays a bare reference, as an ownerless element there is a root (the OMG API JSON shape).
		if LibraryReference(settled, subject) {
			continue
		}
		element, err := writer.element(subject)
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	return elements, nil
}

// apiJSONWriter writes element objects compact into one buffer. Strings keep
// the HTML escaping (`>` as \u003e) the output has always had.
type apiJSONWriter struct {
	buf *bytes.Buffer
	enc *json.Encoder
}

func (w apiJSONWriter) object(o apiJSONObject) error {
	w.buf.WriteByte('{')
	for i, member := range o {
		if i > 0 {
			w.buf.WriteByte(',')
		}
		if err := w.value(member.key); err != nil {
			return err
		}
		w.buf.WriteByte(':')
		if err := w.value(member.value); err != nil {
			return fmt.Errorf("%s: %w", member.key, err)
		}
	}
	w.buf.WriteByte('}')
	return nil
}

// value encodes v in place, dropping the newline Encode ends each value with.
func (w apiJSONWriter) value(v any) error {
	if _, unresolved := v.(apiJSONNameReference); unresolved {
		return fmt.Errorf("unresolved object-property name reached the JSON encoder")
	}
	if err := w.enc.Encode(v); err != nil {
		return err
	}
	w.buf.Truncate(w.buf.Len() - 1)
	return nil
}

// apiJSONMember is one key-value pair of an element object, order preserved.
type apiJSONMember struct {
	key   string
	value any
}

// apiJSONObject is a JSON object that marshals its members in order.
type apiJSONObject []apiJSONMember

func (o apiJSONObject) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, member := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(member.key)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(member.value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", member.key, err)
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// apiJSONReference is the {"@id": <id>} spelling of a reference value.
type apiJSONReference struct {
	ID string `json:"@id"`
}

// apiJSONNameReference marks an object-property literal that names a target.
type apiJSONNameReference struct {
	name string
}

type apiJSONMappingNameIndex struct {
	subjectSlots map[string]int
	names        []string
	hasName      []bool
}

func newAPIJSONMappingNameIndex(graph *rdf.Graph, subjectSlots map[string]int) *apiJSONMappingNameIndex {
	defaults := mappingQualifiedNames(graph)
	index := &apiJSONMappingNameIndex{
		subjectSlots: subjectSlots,
		names:        make([]string, len(subjectSlots)),
		hasName:      make([]bool, len(subjectSlots)),
	}
	for _, subject := range graph.SubjectsView() {
		position, ok := subjectSlots[subject.Value]
		if !ok || position >= len(index.names) {
			continue
		}
		id, ok := rdf.SubjectID(subject)
		if !ok {
			continue
		}
		name, ok := defaults[id]
		if ok {
			index.names[position] = name
			index.hasName[position] = true
		}
	}
	return index
}

func (index *apiJSONMappingNameIndex) name(subject rdf.Term) (string, bool) {
	position, ok := index.subjectSlots[subject.Value]
	if !ok || position >= len(index.names) || !index.hasName[position] {
		return "", false
	}
	return index.names[position], true
}

// apiJSONElement builds the element object for one subject: "@type", "@id",
// then each property in statement order.
func apiJSONElement(graph *rdf.Graph, subject rdf.Term) (apiJSONObject, error) {
	evaluation := newAPIJSONEvaluation(graph)
	mappingNames := newAPIJSONMappingNameIndex(
		evaluation.structure.graph, evaluation.structure.subjectSlots,
	)
	return apiJSONElementWithEvaluation(graph, subject, evaluation, mappingNames)
}

func apiJSONElementWithEvaluation(
	graph *rdf.Graph,
	subject rdf.Term,
	evaluation *apiJSONEvaluation,
	mappingNames *apiJSONMappingNameIndex,
) (apiJSONObject, error) {
	return apiJSONElementWithEvaluationOptions(
		graph, subject, evaluation, mappingNames, newAPIJSONNameIndex(graph), &apiJSONTermCache{}, APIJSONOptions{},
	)
}

func apiJSONElementWithEvaluationOptions(
	graph *rdf.Graph,
	subject rdf.Term,
	evaluation *apiJSONEvaluation,
	mappingNames *apiJSONMappingNameIndex,
	nameIndex *apiJSONNameIndex,
	comparisonCache *apiJSONTermCache,
	options APIJSONOptions,
) (apiJSONObject, error) {
	writer := &apiJSONElementWriter{
		graph:              graph,
		evaluation:         evaluation,
		mappingNames:       mappingNames,
		nameIndex:          nameIndex,
		comparisonCache:    comparisonCache,
		propertyOrderGraph: graph,
		options:            options,
		scratch:            &apiJSONWriteScratch{},
	}
	return writer.element(subject)
}

// apiJSONElementWriter writes the element objects of one graph: the indexes
// and caches every subject shares, and the graph whose property order is kept.
type apiJSONElementWriter struct {
	graph              *rdf.Graph
	evaluation         *apiJSONEvaluation
	mappingNames       *apiJSONMappingNameIndex
	nameIndex          *apiJSONNameIndex
	comparisonCache    *apiJSONTermCache
	propertyOrderGraph *rdf.Graph
	options            APIJSONOptions
	scratch            *apiJSONWriteScratch
}

func (w *apiJSONElementWriter) element(subject rdf.Term) (apiJSONObject, error) {
	types := w.graph.ObjectsView(subject, rdf.RDFType)
	if len(types) != 1 {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the subject <%s>", subject.Value),
			Note: fmt.Sprintf("it has %d rdf:type statements, and an API element object carries exactly one \"@type\"", len(types)),
		}
	}
	id, ok := rdf.SubjectID(subject)
	if !ok {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the subject <%s>", subject.Value),
			Note: "its IRI is in neither the element nor the expression namespace, so it has no \"@id\"",
		}
	}
	if !strings.HasPrefix(types[0].Value, rdf.SysML) && !strings.HasPrefix(types[0].Value, rdf.OpenSysML) {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the rdf:type %s of <%s>", types[0], subject.Value),
			Note: "an element's metaclass is a sysml: or sysx: term",
		}
	}
	// The metaclass name for the ontology's multiplicity lookup, empty for a
	// sysx: element the metamodel does not declare.
	var metaclass string
	if strings.HasPrefix(types[0].Value, rdf.SysML) {
		metaclass = strings.TrimPrefix(types[0].Value, rdf.SysML)
	}
	predicates := w.graph.PredicatesView(subject)
	element := make(apiJSONObject, 0, len(predicates)+5)
	element = append(element,
		apiJSONMember{key: "@type", value: apiJSONType(types[0])},
		apiJSONMember{key: "@id", value: id},
	)
	var mappingValues apiJSONObject
	mappingValuesPosition := -1
	var specName any
	if w.evaluation != nil && w.evaluation.evaluator != nil && w.evaluation.structure != nil {
		if semanticElement, ok := w.evaluation.structure.element(subject); ok {
			value, supported := w.evaluation.evaluator.PropertyReadOnly(semanticElement, "Element", "qualifiedName")
			if supported {
				switch value.Kind {
				case metamodel.NullValue:
				case metamodel.StringValue:
					specName = value.String
				default:
					return nil, fmt.Errorf("qualifiedName has unsupported evaluator value kind %d", value.Kind)
				}
			}
		}
	}
	mappingName, hasMappingName, err := apiJSONMappingQualifiedName(w.graph, subject, w.mappingNames, specName)
	if err != nil {
		return nil, err
	}
	mappingNameEmitted := false
	qualifiedNameInGraph := false
	for _, predicate := range predicates {
		switch {
		case predicate == rdf.RDFType:
			continue
		case rdf.IsAnnotationJSON(predicate):
			// The collection is written at its sysml: key, as the array the
			// annotation states; the annotation itself is no key of its own.
			continue
		case strings.HasPrefix(predicate, rdf.SysML):
			key := strings.TrimPrefix(predicate, rdf.SysML)
			if key == "qualifiedName" {
				qualifiedNameInGraph = true
				if specName != nil {
					element = append(element, apiJSONMember{key: key, value: specName})
				}
				if hasMappingName {
					element = append(element, apiJSONMember{key: apiJSONMappingQualifiedNameKey, value: mappingName})
					mappingNameEmitted = true
				}
				continue
			}
			property, propertyKnown := ontology.PropertyOf(metaclass, key)
			objectProperty := propertyKnown && property.Kind == ontology.ObjectProperty
			value, err := apiJSONSysMLValue(
				w.graph, subject, predicate, key, metaclass, property, propertyKnown, objectProperty, w.nameIndex,
				w.comparisonCache,
			)
			if err != nil {
				return nil, err
			}
			value, keep := apiJSONResolveUnresolvedProperty(
				w.graph, w.nameIndex, subject, metaclass, key, value, w.options,
			)
			comparison, err := apiJSONPropertyValuesEqual(
				w.graph, w.nameIndex, subject, predicate, key, metaclass, value, keep, w.comparisonCache,
			)
			if err != nil {
				return nil, err
			}
			if !comparison.equal {
				mapped, err := apiJSONMappingValuesForProperty(
					w.graph, w.nameIndex, subject, predicate, key, comparison.members, comparison.hasAnnotation,
				)
				if err != nil {
					return nil, fmt.Errorf("sysml:%s of <%s>: %w", key, subject.Value, err)
				}
				mappingValues = append(mappingValues, apiJSONMember{key: key, value: mapped})
				if keep {
					element = append(element, apiJSONMember{key: key, value: value})
				} else if mappingValuesPosition < 0 {
					mappingValuesPosition = len(element)
				}
				continue
			}
			if keep {
				element = append(element, apiJSONMember{key: key, value: value})
			}
		case apiJSONReservedPredicate(predicate):
			continue
		case strings.HasPrefix(predicate, rdf.OpenSysML):
			key := "sysx:" + strings.TrimPrefix(predicate, rdf.OpenSysML)
			objects := w.graph.ObjectsView(subject, predicate)
			value, err := apiJSONValues(subject, "", objects, false)
			if err != nil {
				return nil, err
			}
			element = append(element, apiJSONMember{key: key, value: value})
		default:
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the predicate <%s> of <%s>", predicate, subject.Value),
				Note: "only the sysml: and sysx: vocabularies have API element keys",
			}
		}
	}
	if specName != nil && !qualifiedNameInGraph {
		element = append(element, apiJSONMember{key: "qualifiedName", value: specName})
	}
	if hasMappingName && !mappingNameEmitted {
		element = append(element, apiJSONMember{key: apiJSONMappingQualifiedNameKey, value: mappingName})
	}
	if len(mappingValues) > 0 {
		member := apiJSONMember{key: apiJSONMappingValuesKey, value: mappingValues}
		if mappingValuesPosition < 0 {
			element = append(element, member)
		} else {
			element = append(element, apiJSONMember{})
			copy(element[mappingValuesPosition+1:], element[mappingValuesPosition:])
			element[mappingValuesPosition] = member
		}
	}
	element = appendAPIJSONOwnedAbsencesWithScratch(w.graph, subject, element, types[0], w.scratch)
	if apiJSONRootSubject(w.graph, subject) && !apiJSONExternalForm(w.graph, w.scratch) {
		element = append(element, apiJSONMember{key: apiJSONStatedDefaultsKey, value: true})
	}
	graphOrder := apiJSONGraphPropertyOrder(w.propertyOrderGraph, subject)
	graphOrder = apiJSONGraphPropertyOrderWithFallback(
		graphOrder, apiJSONGraphPropertyOrder(w.graph, subject),
	)
	if len(graphOrder) > 0 && apiJSONHasPropertyOrderSidecar(element, w.graph, subject) {
		naturalOrder, sidecarOnly := apiJSONNaturalPropertyOrderWithScratch(
			w.graph, subject, element, graphOrder, w.scratch,
		)
		if apiJSONHasMisplacedSidecarProperty(graphOrder, naturalOrder, sidecarOnly) {
			element = append(element, apiJSONMember{
				key:   apiJSONMappingPropertyOrderKey,
				value: graphOrder,
			})
		}
	}
	return element, nil
}

func apiJSONHasPropertyOrderSidecar(element apiJSONObject, graph *rdf.Graph, subject rdf.Term) bool {
	for _, member := range element {
		switch member.key {
		case "qualifiedName", apiJSONMappingQualifiedNameKey, apiJSONMappingValuesKey:
			return true
		}
	}
	return graph.HasProperty(subject, rdf.SysML+pQualifiedName)
}

func apiJSONGraphPropertyOrder(graph *rdf.Graph, subject rdf.Term) []string {
	predicates := graph.PredicatesView(subject)
	order := make([]string, 0, len(predicates))
	for _, predicate := range predicates {
		if predicate == rdf.RDFType || rdf.IsAnnotationJSON(predicate) {
			continue
		}
		switch {
		case strings.HasPrefix(predicate, rdf.SysML):
			order = append(order, strings.TrimPrefix(predicate, rdf.SysML))
		case strings.HasPrefix(predicate, rdf.OpenSysML):
			order = append(order, "sysx:"+strings.TrimPrefix(predicate, rdf.OpenSysML))
		}
	}
	return order
}

func apiJSONNaturalPropertyOrder(graph *rdf.Graph, subject rdf.Term, element apiJSONObject) ([]string, []string) {
	return apiJSONNaturalPropertyOrderWithOrder(
		graph, subject, element, apiJSONGraphPropertyOrder(graph, subject),
	)
}

type apiJSONWriteScratch struct {
	absences            apiJSONObject
	externalForm        *bool
	memberProperties    map[string]bool
	emitted             map[string]bool
	graphProperties     map[string]bool
	seen                map[string]bool
	order               []string
	sidecarOnly         []string
	filtered            []string
	filteredSidecarOnly []string
}

func apiJSONNaturalPropertyOrderWithOrder(
	graph *rdf.Graph,
	subject rdf.Term,
	element apiJSONObject,
	graphOrder []string,
) ([]string, []string) {
	return apiJSONNaturalPropertyOrderWithScratch(
		graph, subject, element, graphOrder, &apiJSONWriteScratch{},
	)
}

func apiJSONNaturalPropertyOrderWithScratch(
	graph *rdf.Graph,
	subject rdf.Term,
	element apiJSONObject,
	graphOrder []string,
	scratch *apiJSONWriteScratch,
) ([]string, []string) {
	if scratch == nil {
		scratch = &apiJSONWriteScratch{}
	}
	if scratch.memberProperties == nil {
		scratch.memberProperties = make(map[string]bool, len(element))
	} else {
		clear(scratch.memberProperties)
	}
	if scratch.emitted == nil {
		scratch.emitted = make(map[string]bool, len(element))
	} else {
		clear(scratch.emitted)
	}
	if scratch.graphProperties == nil {
		scratch.graphProperties = make(map[string]bool, len(graphOrder))
	} else {
		clear(scratch.graphProperties)
	}
	if scratch.seen == nil {
		scratch.seen = make(map[string]bool, len(graphOrder))
	} else {
		clear(scratch.seen)
	}
	order := scratch.order[:0]
	sidecarOnly := scratch.sidecarOnly[:0]
	filtered := scratch.filtered[:0]
	filteredSidecarOnly := scratch.filteredSidecarOnly[:0]
	mappingQualifiedNameIndex := -1
	hasQualifiedName := false
	for i, member := range element {
		switch member.key {
		case "@type", "@id", apiJSONMappingValuesKey, apiJSONMappingPropertyOrderKey:
		case apiJSONMappingQualifiedNameKey:
			if mappingQualifiedNameIndex < 0 {
				mappingQualifiedNameIndex = i
			}
		case "qualifiedName":
			hasQualifiedName = true
			scratch.memberProperties[member.key] = true
		default:
			scratch.memberProperties[member.key] = true
		}
	}

	for _, member := range element {
		switch member.key {
		case "@type", "@id", apiJSONMappingPropertyOrderKey, apiJSONMappingQualifiedNameKey:
			continue
		case apiJSONMappingValuesKey:
			values, ok := member.value.(apiJSONObject)
			if !ok {
				continue
			}
			for _, value := range values {
				if scratch.memberProperties[value.key] || scratch.emitted[value.key] {
					continue
				}
				order = append(order, value.key)
				scratch.emitted[value.key] = true
				sidecarOnly = append(sidecarOnly, value.key)
			}
		default:
			if scratch.emitted[member.key] {
				continue
			}
			order = append(order, member.key)
			scratch.emitted[member.key] = true
		}
	}

	if !hasQualifiedName && mappingQualifiedNameIndex >= 0 {
		insertAt := len(order)
		positioned := false
		for i := mappingQualifiedNameIndex + 1; i < len(element); i++ {
			predicate, ok := apiJSONQualifiedNameMemberPredicate(element[i].key)
			if !ok || !graph.HasProperty(subject, predicate) {
				continue
			}
			insertAt = indexOfAPIJSONProperty(order, element[i].key)
			if insertAt < 0 {
				continue
			}
			positioned = true
			break
		}
		if !positioned {
			for i := mappingQualifiedNameIndex - 1; i >= 0; i-- {
				predicate, ok := apiJSONQualifiedNameMemberPredicate(element[i].key)
				if !ok || !graph.HasProperty(subject, predicate) {
					continue
				}
				insertAt = indexOfAPIJSONProperty(order, element[i].key) + 1
				positioned = true
				break
			}
		}
		if !positioned {
			insertAt = 0
		}
		if insertAt < 0 {
			insertAt = len(order)
		}
		order = append(order, "")
		copy(order[insertAt+1:], order[insertAt:])
		order[insertAt] = "qualifiedName"
	} else if !hasQualifiedName && graph.HasProperty(subject, rdf.SysML+pQualifiedName) {
		order = append(order, "")
		copy(order[1:], order[:len(order)-1])
		order[0] = "qualifiedName"
	}

	if hasQualifiedName || mappingQualifiedNameIndex >= 0 ||
		graph.HasProperty(subject, rdf.SysML+pQualifiedName) {
		sidecarOnly = append(sidecarOnly, "qualifiedName")
	}

	for _, property := range graphOrder {
		scratch.graphProperties[property] = true
	}
	for _, property := range order {
		if scratch.graphProperties[property] && !scratch.seen[property] {
			filtered = append(filtered, property)
			scratch.seen[property] = true
		}
	}
	for _, property := range sidecarOnly {
		if scratch.graphProperties[property] {
			filteredSidecarOnly = append(filteredSidecarOnly, property)
		}
	}
	scratch.order = order
	scratch.sidecarOnly = sidecarOnly
	scratch.filtered = filtered
	scratch.filteredSidecarOnly = filteredSidecarOnly
	return filtered, filteredSidecarOnly
}

func apiJSONGraphPropertyOrderWithFallback(order, fallback []string) []string {
	if len(order) == len(fallback) {
		equal := true
		for i := range order {
			if order[i] != fallback[i] {
				equal = false
				break
			}
		}
		if equal {
			return order
		}
	}
	inOrder := make(map[string]bool, len(order))
	for _, property := range order {
		inOrder[property] = true
	}
	before := make(map[string][]string)
	var trailing []string
	var pending []string
	for _, property := range fallback {
		if !inOrder[property] {
			pending = append(pending, property)
			continue
		}
		if len(pending) > 0 {
			before[property] = append(before[property], pending...)
			pending = nil
		}
	}
	trailing = append(trailing, pending...)
	combined := make([]string, 0, len(order)+len(trailing))
	for _, property := range order {
		combined = append(combined, before[property]...)
		combined = append(combined, property)
	}
	return append(combined, trailing...)
}

func apiJSONHasMisplacedSidecarProperty(graphOrder, naturalOrder, sidecarOnly []string) bool {
	for _, property := range sidecarOnly {
		graphPosition := indexOfAPIJSONProperty(graphOrder, property)
		if graphPosition < 0 {
			continue
		}
		naturalPosition := indexOfAPIJSONProperty(naturalOrder, property)
		if naturalPosition < 0 {
			return true
		}
		for otherGraphPosition, other := range graphOrder {
			if other == property {
				continue
			}
			otherNaturalPosition := indexOfAPIJSONProperty(naturalOrder, other)
			if otherNaturalPosition < 0 ||
				(graphPosition < otherGraphPosition) != (naturalPosition < otherNaturalPosition) {
				return true
			}
		}
	}
	return false
}

func indexOfAPIJSONProperty(order []string, property string) int {
	for i, key := range order {
		if key == property {
			return i
		}
	}
	return -1
}

type apiJSONPropertyComparison struct {
	equal         bool
	members       []rdf.CollectionMember
	hasAnnotation bool
}

type apiJSONPropertyComparisonState struct {
	subject    rdf.Term
	predicate  string
	key        string
	metaclass  string
	objects    []rdf.Term
	members    []rdf.CollectionMember
	exactCount int
	position   int
	equal      bool
	nameIndex  *apiJSONNameIndex
	cache      *apiJSONTermCache
}

func (state *apiJSONPropertyComparisonState) compareTerm(term rdf.Term) error {
	exact, err := state.exactTermAt(state.position)
	if err != nil {
		return err
	}
	state.equal = state.equal && exact.Equal(term)
	state.position++
	return nil
}

func (state *apiJSONPropertyComparisonState) compareReference(id string) error {
	exact, err := state.exactTermAt(state.position)
	if err != nil {
		return err
	}
	if target, ok := state.nameIndex.subjectsByID[id]; ok {
		state.equal = state.equal && exact.Equal(target)
	} else {
		state.equal = state.equal && exact.IsIRI() && rdf.ReferenceID(state.subject, exact) == id
	}
	state.position++
	return nil
}

func (state *apiJSONPropertyComparisonState) exactTermAt(position int) (rdf.Term, error) {
	if position >= state.exactCount {
		state.equal = false
		return rdf.Term{}, nil
	}
	exact := rdf.Term{}
	if state.members == nil {
		exact = state.objects[position]
	} else if member := state.members[position]; member.ID != "" {
		exact = rdf.ReferenceIRI(state.subject, member.ID)
		if candidate, ok := state.nameIndex.subjectsByID[member.ID]; ok {
			exact = candidate
		}
	} else {
		found := false
		for _, object := range state.objects {
			matches, err := apiJSONCollectionMemberMatches(state.subject, object, member)
			if err != nil {
				return rdf.Term{}, err
			}
			if matches {
				exact = object
				found = true
				break
			}
		}
		if !found {
			return rdf.Term{}, &UnsupportedError{
				What: fmt.Sprintf("the annotation json member %s of <%s>", member.Literal, state.subject.Value),
				Note: "it states no triple of the collection",
			}
		}
	}
	return exact, nil
}

func (state *apiJSONPropertyComparisonState) compareValue(value any) error {
	switch v := value.(type) {
	case apiJSONReference:
		return state.compareReference(v.ID)
	case apiJSONNameReference:
		return state.compareTerm(rdf.String(v.name))
	case []any:
		for _, member := range v {
			if err := state.compareValue(member); err != nil {
				return err
			}
		}
	case nil:
		return nil
	default:
		term, err := apiJSONScalarOf(
			state.subject, rdf.IRI(state.predicate), state.key, value, state.nameIndex.expressionIDs, state.cache,
		)
		if err != nil {
			return err
		}
		return state.compareTerm(term)
	}
	return nil
}

func apiJSONPropertyValuesEqual(
	graph *rdf.Graph,
	nameIndex *apiJSONNameIndex,
	subject rdf.Term,
	predicate string,
	key, metaclass string,
	value any,
	keep bool,
	cache *apiJSONTermCache,
) (apiJSONPropertyComparison, error) {
	objects := graph.ObjectsView(subject, predicate)
	var members []rdf.CollectionMember
	hasAnnotation := false
	if annotation, ok := graph.Object(subject, rdf.AnnotationJSON+key); ok && annotation.IsLiteral() {
		var err error
		members, err = cache.collectionMembers(graph, subject, key)
		if err != nil {
			return apiJSONPropertyComparison{}, err
		}
		hasAnnotation = true
	}
	exactCount := len(objects)
	if hasAnnotation {
		exactCount = len(members)
	}
	state := apiJSONPropertyComparisonState{
		subject:    subject,
		predicate:  predicate,
		key:        key,
		metaclass:  metaclass,
		objects:    objects,
		members:    members,
		exactCount: exactCount,
		equal:      true,
		nameIndex:  nameIndex,
		cache:      cache,
	}
	if keep && value != nil {
		if err := state.compareValue(value); err != nil {
			return apiJSONPropertyComparison{}, fmt.Errorf("cannot read back %s.%s: %w", metaclass, key, err)
		}
	}
	return apiJSONPropertyComparison{
		equal:         state.equal && state.position == exactCount,
		members:       members,
		hasAnnotation: hasAnnotation,
	}, nil
}

func apiJSONCollectionMemberMatches(
	subject rdf.Term,
	object rdf.Term,
	member rdf.CollectionMember,
) (bool, error) {
	if member.ID != "" {
		return object.IsIRI() && rdf.ReferenceID(subject, object) == member.ID, nil
	}
	if object.IsIRI() {
		return false, nil
	}
	literal := member.Literal
	switch object.Datatype {
	case rdf.XSD + "boolean":
		if literal.Datatype != rdf.XSD+"boolean" {
			return false, nil
		}
		objectValue := object.Value == "true" || object.Value == "1"
		if objectValue {
			return literal.Value == "true" || literal.Value == "1", nil
		}
		return literal.Value == "false" || literal.Value == "0", nil
	case rdf.XSD + "integer", rdf.XSD + "decimal", rdf.XSD + "double", rdf.XSD + "float":
		if !apiJSONNumericDatatype(literal.Datatype) {
			return false, nil
		}
		return object.Value == literal.Value, nil
	default:
		if apiJSONNumericDatatype(literal.Datatype) || literal.Datatype == rdf.XSD+"boolean" {
			return false, nil
		}
		return object.Value == literal.Value, nil
	}
}

func apiJSONNumericDatatype(datatype string) bool {
	switch datatype {
	case rdf.XSD + "integer", rdf.XSD + "decimal", rdf.XSD + "double", rdf.XSD + "float":
		return true
	default:
		return false
	}
}

func apiJSONMappingValuesForProperty(
	graph *rdf.Graph,
	nameIndex *apiJSONNameIndex,
	subject rdf.Term,
	predicate, key string,
	members []rdf.CollectionMember,
	hasAnnotation bool,
) ([]any, error) {
	objects := graph.ObjectsView(subject, predicate)
	if !hasAnnotation {
		values := make([]any, 0, len(objects))
		for _, term := range objects {
			value, err := apiJSONMappingValueForTerm(subject, term)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	}
	values := make([]any, 0, len(members))
	for _, member := range members {
		var term rdf.Term
		if member.ID != "" {
			term = rdf.ReferenceIRI(subject, member.ID)
			if candidate, ok := nameIndex.subjectsByID[member.ID]; ok {
				term = candidate
			}
		} else {
			found := false
			for _, object := range objects {
				matches, err := apiJSONCollectionMemberMatches(subject, object, member)
				if err != nil {
					return nil, err
				}
				if matches {
					term = object
					found = true
					break
				}
			}
			if !found {
				return nil, &UnsupportedError{
					What: fmt.Sprintf("the annotation json member %s of <%s>", member.Literal, subject.Value),
					Note: "it states no triple of the collection",
				}
			}
		}
		value, err := apiJSONMappingValueForTerm(subject, term)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func apiJSONMappingValueForTerm(subject, term rdf.Term) (any, error) {
	if term.IsIRI() {
		id := rdf.ReferenceID(subject, term)
		if id == "" {
			return nil, fmt.Errorf("reference %s has no API id", term)
		}
		return apiJSONReference{ID: id}, nil
	}
	if term.IsLiteral() {
		return term.Value, nil
	}
	return nil, fmt.Errorf("unsupported graph object %s", term)
}

func apiJSONMappingQualifiedName(
	graph *rdf.Graph,
	subject rdf.Term,
	defaults *apiJSONMappingNameIndex,
	specName any,
) (any, bool, error) {
	objects := graph.ObjectsView(subject, rdf.SysML+"qualifiedName")
	if len(objects) > 1 {
		return nil, false, fmt.Errorf("subject <%s> has %d sysml:qualifiedName values", subject.Value, len(objects))
	}
	if len(objects) == 1 {
		if !objects[0].IsLiteral() {
			return nil, false, fmt.Errorf("subject <%s> has a non-literal sysml:qualifiedName", subject.Value)
		}
		_, ok := rdf.SubjectID(subject)
		if !ok {
			return nil, false, fmt.Errorf("subject <%s> has no API element id", subject.Value)
		}
		graphName := objects[0].Value
		if defaultName, hasDefault := defaults.name(subject); hasDefault && defaultName == graphName {
			if specName, hasSpecName := specName.(string); !hasSpecName || specName == graphName {
				return nil, false, nil
			}
		}
		if graph.BoolValue(subject, rdf.SysML+"isLibraryElement") && specName == graphName {
			return nil, false, nil
		}
		return graphName, true, nil
	}
	return nil, false, nil
}

// collectionObject is the triple a collection member states: the object whose
// JSON spelling matches, so its datatype decides the scalar the member emits.
func collectionObject(subject rdf.Term, objects []rdf.Term, member rdf.CollectionMember) (rdf.Term, error) {
	spelling, err := member.JSON()
	if err != nil {
		return rdf.Term{}, err
	}
	for _, object := range objects {
		if have, err := rdf.ValueJSON(subject, object); err == nil && have == spelling {
			return object, nil
		}
	}
	return rdf.Term{}, &UnsupportedError{
		What: fmt.Sprintf("the annotation json member %s of <%s>", spelling, subject.Value),
		Note: "it states no triple of the collection",
	}
}

// apiJSONType is the "@type" spelling of a metaclass IRI: the bare name in
// the sysml: vocabulary, the sysx: CURIE in the extension namespace.
func apiJSONType(typ rdf.Term) string {
	if strings.HasPrefix(typ.Value, rdf.OpenSysML) {
		return "sysx:" + strings.TrimPrefix(typ.Value, rdf.OpenSysML)
	}
	return strings.TrimPrefix(typ.Value, rdf.SysML)
}

// apiJSONSysMLValue is the value a sysml: key carries: the collection its
// json: annotation states, an array when the metamodel declares the property
// multi-valued, or its single object as a scalar.
func apiJSONSysMLValue(
	graph *rdf.Graph,
	subject rdf.Term,
	predicate, key, metaclass string,
	property ontology.Property,
	propertyKnown bool,
	objectProperty bool,
	nameIndex *apiJSONNameIndex,
	cache *apiJSONTermCache,
) (any, error) {
	if key == "chainingFeature" {
		ordered := apiJSONOrderedFeatureChain(graph, nameIndex, subject)
		if len(ordered) > 0 {
			return apiJSONValues(subject, key, ordered, objectProperty)
		}
	}
	objects := graph.ObjectsView(subject, predicate)
	nonunique := propertyKnown && property.Nonunique
	if _, ok := graph.Object(subject, rdf.AnnotationJSON+key); ok && graph.CollectionsSettled() && !nonunique {
		// The typed triples state the collection in the annotation's order (see
		// rdf.Graph.MarkCollectionsSettled), so its members are read from them
		// rather than from the annotation's JSON, which spells the same members.
		values := make([]any, 0, len(objects))
		for _, object := range objects {
			if object.IsIRI() {
				values = append(values, apiJSONReference{ID: rdf.ReferenceID(subject, object)})
				continue
			}
			value, err := apiJSONScalar(subject, key, object, objectProperty)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	}
	if annotation, ok := graph.Object(subject, rdf.AnnotationJSON+key); ok {
		if !annotation.IsLiteral() {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the annotation json:%s of <%s>", key, subject.Value),
				Note: fmt.Sprintf("its object %s is not the JSON literal a collection is stated by", annotation),
			}
		}
		members, err := cache.collectionMembers(graph, subject, key)
		if err != nil {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the annotation json:%s of <%s>", key, subject.Value),
				Note: err.Error(),
			}
		}
		// The members emit in the annotation's order: a reference its {"@id"},
		// a literal its triple's scalar, which is resolved or omitted below.
		values := make([]any, 0, len(members))
		for _, member := range members {
			if member.ID != "" {
				values = append(values, apiJSONReference{ID: member.ID})
				continue
			}
			object, err := collectionObject(subject, objects, member)
			if err != nil {
				return nil, err
			}
			value, err := apiJSONScalar(subject, key, object, objectProperty)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	}
	// An unbounded property is always an array, in triple order; a sysx:
	// element has no metaclass, so the name's declarations must agree.
	many := false
	if metaclass != "" {
		many = propertyKnown && property.Many
	} else {
		many, _ = ontology.ManyAgreed(key)
	}
	if many {
		values := make([]any, 0, len(objects))
		for _, object := range objects {
			value, err := apiJSONScalar(subject, key, object, objectProperty)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	}
	if len(objects) > 1 {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("sysml:%s of <%s>", key, subject.Value),
			Note: "a sysml: collection states its members in the json: annotation, which is absent",
		}
	}
	return apiJSONValues(subject, key, objects, objectProperty)
}

func apiJSONResolveUnresolvedProperty(
	graph *rdf.Graph,
	nameIndex *apiJSONNameIndex,
	subject rdf.Term,
	metaclass string,
	key string,
	value any,
	options APIJSONOptions,
) (any, bool) {
	value, names := apiJSONResolveNameReference(graph, nameIndex, subject, key, value)
	if len(names) == 0 {
		return value, true
	}
	property, known := ontology.PropertyOf(metaclass, key)
	subjectID, _ := rdf.SubjectID(subject)
	if options.Omitted != nil || options.Diagnostic != nil {
		for _, name := range names {
			omission := APIJSONOmission{
				Subject:   subjectID,
				Metaclass: metaclass,
				Property:  key,
				Reason: metamodel.Reason{
					Category: metamodel.ReasonUnresolvedReference,
					Detail:   name,
				},
			}
			if options.Omitted != nil {
				options.Omitted(omission)
			}
			if options.Diagnostic != nil {
				options.Diagnostic(fmt.Sprintf("unresolved API JSON reference %s.%s: %s", metaclass, key, name))
			}
		}
	}
	if known && property.Kind == ontology.ObjectProperty && property.Many {
		return nil, false
	}
	if known && property.Kind == ontology.ObjectProperty {
		return nil, property.Lower == 0
	}
	return nil, false
}

// apiJSONValues spells a property's objects as the key's value: one object a
// scalar, several an array. The key classifies expression text; sysx: keys
// pass "".
func apiJSONValues(subject rdf.Term, key string, objects []rdf.Term, objectProperty bool) (any, error) {
	if len(objects) == 1 {
		return apiJSONScalar(subject, key, objects[0], objectProperty)
	}
	values := make([]any, 0, len(objects))
	for _, object := range objects {
		value, err := apiJSONScalar(subject, key, object, objectProperty)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

// apiJSONScalar is the JSON spelling of one object: an IRI a {"@id": …}
// reference, a plain literal on an object property a name to resolve, a boolean
// or number its JSON primitive, anything else a string. A literal the reader
// would restore in another datatype is refused: the element form carries no
// datatype to spell it in.
func apiJSONScalar(subject rdf.Term, key string, object rdf.Term, objectProperty bool) (any, error) {
	if object.IsIRI() {
		return apiJSONReference{ID: rdf.ReferenceID(subject, object)}, nil
	}
	if object.Lang != "" {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the language-tagged literal %s of <%s>", object, subject.Value),
			Note: "the API element form has no language tags",
		}
	}
	refuse := func(note string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("the literal %s of <%s>", object, subject.Value),
			Note: note,
		}
	}
	retyped := "the API element form carries no datatype, and the reader would read " +
		"this value back as " + apiJSONRestoredType(object)
	unspellable := "its lexical form is not the JSON spelling the reader restores it from"
	switch object.Datatype {
	case rdf.XSD + "boolean":
		if object.Value != "true" && object.Value != "false" {
			return nil, refuse(unspellable)
		}
		return object.Value == "true", nil
	case rdf.XSD + "integer":
		if !apiJSONInteger.MatchString(object.Value) {
			return nil, refuse(unspellable)
		}
		return json.Number(object.Value), nil
	case rdf.XSD + "decimal", rdf.XSD + "double":
		number := json.Number(apiJSONRealLexical(object.Value))
		if _, err := json.Marshal(number); err != nil {
			return nil, refuse(unspellable)
		}
		if realDatatype(object.Value) != object.Datatype {
			return nil, refuse(retyped)
		}
		return number, nil
	case "":
		if objectProperty {
			return apiJSONNameReference{name: object.Value}, nil
		}
		return object.Value, nil
	case rdf.OpenSysML + dtExpression:
		if !apiJSONIsExpressionText(key, object.Value) {
			return nil, refuse(retyped)
		}
		return object.Value, nil
	}
	return nil, refuse(retyped)
}

// apiJSONRestoredType names the datatype apiJSONScalarOf would give a
// literal's JSON spelling, for the refusal note.
func apiJSONRestoredType(object rdf.Term) string {
	switch {
	case object.Datatype == rdf.XSD+"boolean":
		return rdf.XSD + "boolean"
	case object.Datatype == rdf.XSD+"integer", slices.Contains(integerLiterals, object.Datatype):
		return rdf.XSD + "integer"
	case slices.Contains(realLiterals, object.Datatype):
		return realDatatype(object.Value)
	}
	return "a plain literal"
}

// apiJSONRealLexical respells an XSD real's lexical form as a JSON number
// without changing its value: a fraction needs a digit on both sides of the
// point and no '+' sign. Lexicals like INF and NaN stay unspellable.
func apiJSONRealLexical(lexical string) string {
	lexical = strings.TrimPrefix(lexical, "+")
	switch {
	case strings.HasPrefix(lexical, "-."):
		lexical = "-0" + lexical[1:]
	case strings.HasPrefix(lexical, "."):
		lexical = "0" + lexical
	}
	if i := strings.IndexAny(lexical, "eE"); i >= 0 {
		mantissa := lexical[:i]
		if strings.HasSuffix(mantissa, ".") {
			mantissa += "0"
		}
		return mantissa + lexical[i:]
	}
	if strings.HasSuffix(lexical, ".") {
		return lexical + "0"
	}
	return lexical
}
