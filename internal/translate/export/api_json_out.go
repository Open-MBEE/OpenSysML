package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
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
	elements, err := apiJSONElements(graph)
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
	evaluation.qualifiedNames = apiJSONEvaluatedQualifiedNames(evaluation)
	mappingNames := mappingQualifiedNames(settled)
	for _, subject := range subjects {
		element, err := apiJSONElementWithEvaluation(settled, subject, evaluation, mappingNames)
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

// apiJSONNameReference is the {"@ref": <name>} spelling of a reference the
// graph could not link: an object property naming an element by its text.
type apiJSONNameReference struct {
	Ref string `json:"@ref"`
}

// apiJSONElement builds the element object for one subject: "@type", "@id",
// then each property in statement order.
func apiJSONElement(graph *rdf.Graph, subject rdf.Term) (apiJSONObject, error) {
	mappingNames := mappingQualifiedNames(graph)
	evaluation := newAPIJSONEvaluation(graph)
	evaluation.qualifiedNames = apiJSONEvaluatedQualifiedNames(evaluation)
	return apiJSONElementWithEvaluation(graph, subject, evaluation, mappingNames)
}

func apiJSONElementWithEvaluation(
	graph *rdf.Graph,
	subject rdf.Term,
	evaluation *apiJSONEvaluation,
	mappingNames map[string]string,
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
			value, err := apiJSONSysMLValue(graph, subject, predicate, key, metaclass, objectProperty)
			if err != nil {
				return nil, err
			}
			element = append(element, apiJSONMember{key: key, value: value})
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
	value, ok, err := apiJSONEvaluatedQualifiedName(subject, evaluation)
	if err != nil {
		return nil, err
	}
	if ok && value != nil {
		element = append(element, apiJSONMember{key: "qualifiedName", value: value})
	}
	mappingName, hasMappingName, err := apiJSONMappingQualifiedName(graph, subject, mappingNames, value)
	if err != nil {
		return nil, err
	}
	if hasMappingName {
		element = append(element, apiJSONMember{key: "sysx:mappingQualifiedName", value: mappingName})
	}
	element = appendAPIJSONOwnedAbsences(graph, subject, element, types[0])
	return element, nil
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

type apiJSONQualifiedNameResult struct {
	value any
	ok    bool
	err   error
}

type apiJSONElementNameResult struct {
	value metamodel.Value
	ok    bool
}

type apiJSONSiblingKey struct {
	namespace  metamodel.ElementKey
	membership bool
}

type apiJSONSiblingNames struct {
	first     map[string]metamodel.ElementKey
	supported bool
}

type apiJSONQualifiedNameFact struct {
	result    apiJSONQualifiedNameResult
	parent    string
	segment   string
	hasParent bool
}

func apiJSONEvaluatedQualifiedNames(evaluation *apiJSONEvaluation) map[string]apiJSONQualifiedNameResult {
	results := make(map[string]apiJSONQualifiedNameResult)
	if evaluation == nil || evaluation.structure == nil || evaluation.evaluator == nil {
		return results
	}
	nameResults := make(map[metamodel.ElementKey]apiJSONElementNameResult, len(evaluation.structure.bySubject))
	getName := func(element metamodel.Element) apiJSONElementNameResult {
		key := element.Key()
		if result, ok := nameResults[key]; ok {
			return result
		}
		value, ok := evaluation.evaluator.Property(element, "Element", "name")
		result := apiJSONElementNameResult{value: value, ok: ok}
		nameResults[key] = result
		return result
	}
	siblingNames := make(map[apiJSONSiblingKey]apiJSONSiblingNames)
	facts := make(map[string]apiJSONQualifiedNameFact, len(evaluation.structure.bySubject))
	children := make(map[string][]string)
	for subject, element := range evaluation.structure.bySubject {
		class, ok := evaluation.structure.Metaclass(element)
		if !ok {
			facts[subject] = apiJSONQualifiedNameFact{}
			continue
		}
		if _, ok := ontology.PropertyOf(class, "qualifiedName"); !ok {
			facts[subject] = apiJSONQualifiedNameFact{}
			continue
		}
		if name, known := evaluation.structure.QualifiedName(element); known {
			facts[subject] = apiJSONQualifiedNameFact{
				result: apiJSONQualifiedNameResult{value: name, ok: true},
			}
			continue
		}

		name := getName(element)
		if !name.ok {
			facts[subject] = apiJSONQualifiedNameFact{}
			continue
		}
		if effectiveName, ok := evaluation.structure.EffectiveName(element); ok && effectiveName != "" {
			name.value = metamodel.Value{Kind: metamodel.StringValue, String: effectiveName}
		}
		segment := ""
		hasSegment := false
		nameIsString := false
		switch name.value.Kind {
		case metamodel.StringValue:
			segment = identity.EscapeName(name.value.String)
			hasSegment = true
			nameIsString = true
		case metamodel.NullValue:
			shortName, ok := evaluation.evaluator.Property(element, "Element", "shortName")
			if !ok {
				facts[subject] = apiJSONQualifiedNameFact{}
				continue
			}
			switch shortName.Kind {
			case metamodel.NullValue:
				facts[subject] = apiJSONQualifiedNameFact{
					result: apiJSONQualifiedNameResult{ok: true},
				}
				continue
			case metamodel.StringValue:
				segment = identity.EscapeName(shortName.String)
				hasSegment = true
			default:
				facts[subject] = apiJSONQualifiedNameFact{
					result: apiJSONQualifiedNameResult{
						err: fmt.Errorf("qualifiedName has unsupported evaluator value kind %d", shortName.Kind),
					},
				}
				continue
			}
		default:
			facts[subject] = apiJSONQualifiedNameFact{
				result: apiJSONQualifiedNameResult{
					err: fmt.Errorf("qualifiedName has unsupported evaluator value kind %d", name.value.Kind),
				},
			}
			continue
		}
		if !hasSegment {
			facts[subject] = apiJSONQualifiedNameFact{
				result: apiJSONQualifiedNameResult{ok: true},
			}
			continue
		}

		namespace, hasNamespace, known := apiJSONQualifiedNameNamespace(evaluation.structure, element)
		if !known {
			facts[subject] = apiJSONQualifiedNameFact{}
			continue
		}
		if !hasNamespace {
			facts[subject] = apiJSONQualifiedNameFact{
				result: apiJSONQualifiedNameResult{ok: true},
			}
			continue
		}
		if nameIsString {
			siblingKey := apiJSONSiblingKey{namespace: namespace.Key(), membership: element.IsMembership}
			siblings, ok := siblingNames[siblingKey]
			if !ok {
				property := "ownedMember"
				if element.IsMembership {
					property = "ownedMembership"
				}
				value, supported := evaluation.evaluator.Property(namespace, "Namespace", property)
				siblings = apiJSONSiblingNames{first: make(map[string]metamodel.ElementKey), supported: supported && value.Kind == metamodel.SequenceValue}
				if siblings.supported {
					for _, child := range value.Values {
						var member metamodel.Element
						switch child.Kind {
						case metamodel.ElementValue:
							member = child.Element
						case metamodel.MembershipValue:
							member = metamodel.MembershipElement(child.Membership)
						default:
							siblings.supported = false
						}
						if !siblings.supported {
							break
						}
						memberName := getName(member)
						if !memberName.ok {
							siblings.supported = false
							break
						}
						switch memberName.value.Kind {
						case metamodel.NullValue:
							continue
						case metamodel.StringValue:
							if _, exists := siblings.first[memberName.value.String]; !exists {
								siblings.first[memberName.value.String] = member.Key()
							}
						default:
							siblings.supported = false
						}
						if !siblings.supported {
							break
						}
					}
				}
				siblingNames[siblingKey] = siblings
			}
			if !siblings.supported {
				facts[subject] = apiJSONQualifiedNameFact{}
				continue
			}
			if siblings.first[name.value.String] != element.Key() {
				facts[subject] = apiJSONQualifiedNameFact{
					result: apiJSONQualifiedNameResult{ok: true},
				}
				continue
			}
		}

		hasOwner, known := apiJSONQualifiedNameHasOwner(evaluation.structure, namespace)
		if !known {
			facts[subject] = apiJSONQualifiedNameFact{}
			continue
		}
		if !hasOwner {
			facts[subject] = apiJSONQualifiedNameFact{
				result: apiJSONQualifiedNameResult{value: segment, ok: true},
			}
			continue
		}
		parent, ok := evaluation.structure.subject(namespace)
		if !ok {
			facts[subject] = apiJSONQualifiedNameFact{}
			continue
		}
		facts[subject] = apiJSONQualifiedNameFact{
			parent:    parent.Value,
			segment:   segment,
			hasParent: true,
		}
		children[parent.Value] = append(children[parent.Value], subject)
	}

	queue := make([]string, 0, len(facts))
	for subject, fact := range facts {
		if !fact.hasParent {
			results[subject] = fact.result
			queue = append(queue, subject)
		}
	}
	for head := 0; head < len(queue); head++ {
		parent := results[queue[head]]
		for _, child := range children[queue[head]] {
			fact := facts[child]
			result := apiJSONQualifiedNameResult{}
			switch {
			case parent.err != nil:
				result = parent
			case !parent.ok:
			case parent.value == nil:
				result.ok = true
			default:
				parentName, ok := parent.value.(string)
				if ok {
					result = apiJSONQualifiedNameResult{
						value: parentName + "::" + fact.segment,
						ok:    true,
					}
				} else {
					result.ok = true
				}
			}
			results[child] = result
			queue = append(queue, child)
		}
	}

	implicitOwners := make(map[string]bool)
	qualifiedNames := make(map[string]apiJSONQualifiedNameResult, len(results))
	for subject, result := range results {
		qualifiedNames[subject] = result
	}
	for subject := range facts {
		subjectTerm := rdf.IRI(subject)
		if !evaluation.structure.graph.BoolValue(subjectTerm, rdf.OpenSysML+"isKindImplicit") {
			continue
		}
		result := qualifiedNames[subject]
		clear(implicitOwners)
		owner := firstIRI(evaluation.structure.graph, subjectTerm, pOwner, pOwningNamespace)
		for owner.IsIRI() && !implicitOwners[owner.Value] {
			implicitOwners[owner.Value] = true
			if _, ok := evaluation.structure.element(owner); ok {
				if value := qualifiedNames[owner.Value]; value.ok {
					if name, ok := value.value.(string); ok && name != "" {
						result = apiJSONQualifiedNameResult{value: name + "::kind", ok: true}
						break
					}
				}
			}
			owner = firstIRI(evaluation.structure.graph, owner, pOwner, pOwningNamespace)
		}
		results[subject] = result
	}
	return results
}

func apiJSONQualifiedNameNamespace(
	structure *apiJSONGraphStructure,
	element metamodel.Element,
) (metamodel.Element, bool, bool) {
	if element.IsMembership {
		return metamodel.Element{}, false, true
	}
	relationship, present, known := structure.OwningRelationship(element)
	if !known || !present {
		return metamodel.Element{}, false, known
	}
	class, known := structure.Metaclass(relationship)
	if !known {
		return metamodel.Element{}, false, false
	}
	if !ontology.IsAncestorOrSelf(class, "OwningMembership") {
		return metamodel.Element{}, false, true
	}
	namespace, present, known := structure.OwningRelatedElement(relationship)
	if !known || !present {
		return metamodel.Element{}, false, known
	}
	class, known = structure.Metaclass(namespace)
	if !known {
		return metamodel.Element{}, false, false
	}
	return namespace, ontology.IsAncestorOrSelf(class, "Namespace"), true
}

func apiJSONQualifiedNameHasOwner(
	structure *apiJSONGraphStructure,
	element metamodel.Element,
) (bool, bool) {
	relationship, present, known := structure.OwningRelationship(element)
	if !known {
		return false, false
	}
	if present {
		_, ownerPresent, ownerKnown := structure.OwningRelatedElement(relationship)
		return ownerPresent, ownerKnown
	}
	class, known := structure.Metaclass(element)
	if !known {
		return false, false
	}
	if !ontology.IsAncestorOrSelf(class, "Relationship") {
		return false, true
	}
	_, ownerPresent, ownerKnown := structure.OwningRelatedElement(element)
	return ownerPresent, ownerKnown
}

func apiJSONEvaluatedQualifiedName(
	subject rdf.Term,
	evaluation *apiJSONEvaluation,
) (any, bool, error) {
	if evaluation == nil {
		return nil, false, nil
	}
	result, ok := evaluation.qualifiedNames[subject.Value]
	if !ok {
		return nil, false, nil
	}
	return result.value, result.ok, result.err
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
func apiJSONSysMLValue(graph *rdf.Graph, subject rdf.Term, predicate, key, metaclass string, objectProperty bool) (any, error) {
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
		// a literal its triple's scalar — the datatype the reader restores it
		// to, so an unresolved name on an object property becomes {"@ref": n}.
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
