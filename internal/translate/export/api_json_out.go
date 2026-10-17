package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
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
// keys, the shape GET /projects/{p}/commits/{c}/elements serves. It is the same
// mapping the Turtle writer spells: the subjects of the graph in order, their
// sysml: and sysx: properties as values, each collection as the array its
// json: annotation states, and each multi-valued metamodel property as an
// array even when no annotation states it. The document's top-level elements
// are owned by the unnamed root Namespace the pilot's documents carry, which
// the Turtle form leaves out (see docs/reference/rdf-mapping.md).
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
	settled, err := rdf.ReconcileCollections(wrapped)
	if err != nil {
		return nil, err
	}
	subjects := settled.SubjectsView()
	elements := make([]apiJSONObject, 0, len(subjects))
	evaluation := newAPIJSONEvaluation(settled)
	mappingNames := mappingQualifiedNames(settled)
	nameIndex := newAPIJSONNameIndex(settled)
	for _, subject := range subjects {
		element, err := apiJSONElementWithEvaluationOptions(
			settled, subject, evaluation, mappingNames, nameIndex, options,
		)
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

// apiJSONElement builds the element object for one subject: "@type", "@id",
// then each property in statement order.
func apiJSONElement(graph *rdf.Graph, subject rdf.Term) (apiJSONObject, error) {
	mappingNames := mappingQualifiedNames(graph)
	evaluation := newAPIJSONEvaluation(graph)
	return apiJSONElementWithEvaluation(graph, subject, evaluation, mappingNames)
}

func apiJSONElementWithEvaluation(
	graph *rdf.Graph,
	subject rdf.Term,
	evaluation *apiJSONEvaluation,
	mappingNames map[string]string,
) (apiJSONObject, error) {
	return apiJSONElementWithEvaluationOptions(
		graph, subject, evaluation, mappingNames, newAPIJSONNameIndex(graph), APIJSONOptions{},
	)
}

func apiJSONElementWithEvaluationOptions(
	graph *rdf.Graph,
	subject rdf.Term,
	evaluation *apiJSONEvaluation,
	mappingNames map[string]string,
	nameIndex *apiJSONNameIndex,
	options APIJSONOptions,
) (apiJSONObject, error) {
	types := graph.ObjectsView(subject, rdf.RDFType)
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
	element := apiJSONObject{
		{key: "@type", value: apiJSONType(types[0])},
		{key: "@id", value: id},
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
	mappingValues := make(map[string][]any)
	deferredProperties := make(map[string]any)
	for _, predicate := range graph.PredicatesView(subject) {
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
				continue
			}
			objectProperty := false
			if metaclass != "" {
				property, known := ontology.PropertyOf(metaclass, key)
				objectProperty = known && property.Kind == ontology.ObjectProperty
			}
			value, err := apiJSONSysMLValue(
				graph, subject, predicate, key, metaclass, objectProperty, nameIndex,
			)
			if err != nil {
				return nil, err
			}
			value, keep := apiJSONResolveUnresolvedProperty(
				graph, nameIndex, subject, metaclass, key, value, options,
			)
			exact, err := apiJSONExactPropertyValues(graph, nameIndex, subject, predicate, key)
			if err != nil {
				return nil, err
			}
			readback, err := apiJSONSpecPropertyValues(
				nameIndex, subject, predicate, key, metaclass, value, keep,
			)
			if err != nil {
				return nil, err
			}
			if !slices.EqualFunc(exact, readback, func(a, b rdf.Term) bool { return a.Equal(b) }) {
				mapped, err := apiJSONMappingValuesForTerms(subject, exact)
				if err != nil {
					return nil, fmt.Errorf("sysml:%s of <%s>: %w", key, subject.Value, err)
				}
				mappingValues[key] = mapped
				if keep {
					deferredProperties[key] = value
				}
				continue
			}
			if keep {
				element = append(element, apiJSONMember{key: key, value: value})
			}
		case strings.HasPrefix(predicate, rdf.OpenSysML):
			key := "sysx:" + strings.TrimPrefix(predicate, rdf.OpenSysML)
			objects := graph.ObjectsView(subject, predicate)
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
	var specName any
	if evaluation != nil && evaluation.evaluator != nil && evaluation.structure != nil {
		if semanticElement, ok := evaluation.structure.element(subject); ok {
			value, supported := evaluation.evaluator.Property(semanticElement, "Element", "qualifiedName")
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
	if specName != nil {
		element = append(element, apiJSONMember{key: "qualifiedName", value: specName})
	}
	mappingName, hasMappingName, err := apiJSONMappingQualifiedName(graph, subject, mappingNames, specName)
	if err != nil {
		return nil, err
	}
	if hasMappingName {
		element = append(element, apiJSONMember{key: "sysx:mappingQualifiedName", value: mappingName})
	}
	if len(mappingValues) > 0 {
		keys := make([]string, 0, len(mappingValues))
		for key := range mappingValues {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if value, ok := deferredProperties[key]; ok {
				element = append(element, apiJSONMember{key: key, value: value})
			}
		}
		sidecar := make(map[string]any, len(mappingValues))
		for key, values := range mappingValues {
			sidecar[key] = values
		}
		element = append(element, apiJSONMember{key: "sysx:mappingValues", value: sidecar})
	}
	element = appendAPIJSONOwnedAbsences(graph, subject, element, types[0])
	return element, nil
}

func apiJSONExactPropertyValues(
	graph *rdf.Graph,
	nameIndex *apiJSONNameIndex,
	subject rdf.Term,
	predicate string,
	key string,
) ([]rdf.Term, error) {
	objects := graph.ObjectsView(subject, predicate)
	annotation, ok := graph.Object(subject, rdf.AnnotationJSON+key)
	if !ok || !annotation.IsLiteral() {
		return objects, nil
	}
	members, err := rdf.ParseCollectionJSON(annotation.Value)
	if err != nil {
		return nil, err
	}
	ordered := make([]rdf.Term, 0, len(members))
	for _, member := range members {
		if member.ID != "" {
			target := rdf.ReferenceIRI(subject, member.ID)
			if candidate, ok := nameIndex.subjectsByID[member.ID]; ok {
				target = candidate
			}
			ordered = append(ordered, target)
			continue
		}
		object, err := collectionObject(subject, objects, member)
		if err != nil {
			return nil, err
		}
		ordered = append(ordered, object)
	}
	return ordered, nil
}

func apiJSONSpecPropertyValues(
	nameIndex *apiJSONNameIndex,
	subject rdf.Term,
	predicate string,
	key, metaclass string,
	value any,
	keep bool,
) ([]rdf.Term, error) {
	if !keep || value == nil {
		return nil, nil
	}
	cache := newAPIJSONTermCache()
	predicateTerm := rdf.IRI(predicate)
	var terms []rdf.Term
	var appendValue func(any) error
	appendValue = func(value any) error {
		switch v := value.(type) {
		case apiJSONReference:
			terms = append(terms, apiJSONReferenceTerm(subject, v.ID, nameIndex.expressionIDs))
		case apiJSONNameReference:
			terms = append(terms, rdf.String(v.name))
		case []any:
			for _, member := range v {
				if err := appendValue(member); err != nil {
					return err
				}
			}
		case nil:
			return nil
		default:
			term, err := apiJSONScalarOf(
				subject, predicateTerm, key, value, nameIndex.expressionIDs, cache,
			)
			if err != nil {
				return err
			}
			terms = append(terms, term)
		}
		return nil
	}
	if err := appendValue(value); err != nil {
		return nil, fmt.Errorf("cannot read back %s.%s: %w", metaclass, key, err)
	}
	return terms, nil
}

func apiJSONMappingValuesForTerms(subject rdf.Term, terms []rdf.Term) ([]any, error) {
	values := make([]any, 0, len(terms))
	for _, term := range terms {
		if term.IsIRI() {
			id := rdf.ReferenceID(subject, term)
			if id == "" {
				return nil, fmt.Errorf("reference %s has no API id", term)
			}
			values = append(values, apiJSONReference{ID: id})
			continue
		}
		if term.IsLiteral() {
			values = append(values, term.Value)
			continue
		}
		return nil, fmt.Errorf("unsupported graph object %s", term)
	}
	return values, nil
}

func apiJSONMappingQualifiedName(
	graph *rdf.Graph,
	subject rdf.Term,
	defaults map[string]string,
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
		id, ok := rdf.SubjectID(subject)
		if !ok {
			return nil, false, fmt.Errorf("subject <%s> has no API element id", subject.Value)
		}
		graphName := objects[0].Value
		if defaultName, hasDefault := defaults[id]; hasDefault && defaultName == graphName {
			return nil, false, nil
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
	objectProperty bool,
	nameIndex *apiJSONNameIndex,
) (any, error) {
	objects := graph.ObjectsView(subject, predicate)
	if _, ok := graph.Object(subject, rdf.AnnotationJSON+key); ok && graph.CollectionsSettled() {
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
		members, err := rdf.ParseCollectionJSON(annotation.Value)
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
		property, ok := ontology.PropertyOf(metaclass, key)
		many = ok && property.Many
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
	if known && property.Kind == ontology.ObjectProperty && !property.Many {
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
