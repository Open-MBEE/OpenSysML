package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

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
	wrapped, err := withRootNamespace(graph)
	if err != nil {
		return nil, err
	}
	settled, err := rdf.ReconcileCollections(wrapped)
	if err != nil {
		return nil, err
	}
	elements := make([]apiJSONObject, 0, len(settled.Subjects()))
	for _, subject := range settled.Subjects() {
		element, err := apiJSONElement(settled, subject)
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(elements); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
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

// apiJSONNameReference is the {"@ref": <name>} spelling of a reference the
// graph could not link: an object property naming an element by its text.
type apiJSONNameReference struct {
	Ref string `json:"@ref"`
}

// apiJSONElement builds the element object for one subject: "@type", "@id",
// then each property in statement order.
func apiJSONElement(graph *rdf.Graph, subject rdf.Term) (apiJSONObject, error) {
	types := graph.Objects(subject, rdf.RDFType)
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
	for _, predicate := range graph.Predicates(subject) {
		switch {
		case predicate == rdf.RDFType:
			continue
		case rdf.IsAnnotationJSON(predicate):
			// The collection is written at its sysml: key, as the array the
			// annotation states; the annotation itself is no key of its own.
			continue
		case strings.HasPrefix(predicate, rdf.SysML):
			key := strings.TrimPrefix(predicate, rdf.SysML)
			objectProperty := false
			if metaclass != "" {
				property, known := ontology.PropertyOf(metaclass, key)
				objectProperty = known && property.Kind == ontology.ObjectProperty
			}
			value, err := apiJSONSysMLValue(graph, subject, predicate, key, metaclass, objectProperty)
			if err != nil {
				return nil, err
			}
			element = append(element, apiJSONMember{key: key, value: value})
		case strings.HasPrefix(predicate, rdf.OpenSysML):
			key := "sysx:" + strings.TrimPrefix(predicate, rdf.OpenSysML)
			objects := graph.Objects(subject, predicate)
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
	return element, nil
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
func apiJSONSysMLValue(graph *rdf.Graph, subject rdf.Term, predicate, key, metaclass string, objectProperty bool) (any, error) {
	objects := graph.Objects(subject, predicate)
	if annotation, ok := graph.Object(subject, rdf.AnnotationJSON+key); ok {
		if !annotation.IsLiteral() {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the annotation json:%s of <%s>", key, subject.Value),
				Note: fmt.Sprintf("its object %s is not the JSON literal a collection is stated by", annotation),
			}
		}
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(annotation.Value), &raw); err != nil {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the annotation json:%s of <%s>", key, subject.Value),
				Note: fmt.Sprintf("its literal is not JSON: %v", err),
			}
		}
		// The members spell the way a scalar does: an unresolved name on an
		// object property as {"@ref": <name>}, not the bare string stored.
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
// reference, a plain literal on an object property the {"@ref": …} of a name
// the graph could not link, a boolean or number its JSON primitive, anything
// else a string. A literal the reader would restore in another datatype is
// refused: the element form carries no datatype to spell it in.
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
			return apiJSONNameReference{Ref: object.Value}, nil
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
