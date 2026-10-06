package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// ReadAPIJSON parses the SysML v2 API's element form — a JSON array of element
// objects, or a single element object — into the same graph ParseTurtle yields:
// the "@type" and "@id" of each object and each metamodel key in document
// order. It is the inverse of WriteAPIJSON, so a graph it builds writes the
// same elements back; the unnamed root Namespace a document wraps its
// top-level elements in is dropped, as the Turtle form does not carry it.
func ReadAPIJSON(data []byte) (*rdf.Graph, error) {
	expressionIDs, elementCount, err := apiJSONExpressionIDs(data)
	if err != nil {
		return nil, err
	}
	builder := rdf.NewGraphBuilder(elementCount * apiJSONEstimatedTriplesPerElement)
	cache := newAPIJSONTermCache()
	// The collection annotations are stated after every element's triples, the
	// positions AnnotateCollections writes them in.
	var annotations []apiJSONAnnotation
	err = apiJSONElementsOf(data, apiJSONObjectOf, func(element apiJSONElementData) error {
		var subject rdf.Term
		if expressionIDs[element.id] {
			subject = rdf.IRI(rdf.Expression + element.id)
		} else {
			subject = rdf.ReferenceIRI(rdf.Term{}, element.id)
		}
		typ, err := cache.typeIRI(element.typ)
		if err != nil {
			return fmt.Errorf("element %q: %w", element.id, err)
		}
		builder.Add(subject, rdf.IRI(rdf.RDFType), typ)
		for _, member := range element.members {
			predicate, sysmlKey, err := cache.predicate(member.key)
			if err != nil {
				return fmt.Errorf("element %q: %w", element.id, err)
			}
			if err := apiJSONTriples(builder, subject, predicate, sysmlKey, member.value, expressionIDs, cache, &annotations); err != nil {
				return fmt.Errorf("element %q: %w", element.id, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, annotation := range annotations {
		text, err := rdf.CollectionJSON(annotation.subject, annotation.members)
		if err != nil {
			return nil, err
		}
		builder.Add(annotation.subject, rdf.AnnotationJSONTerm(annotation.key), rdf.String(text))
	}
	if len(expressionIDs) > 0 {
		builder.SetPrefix(rdf.ExpressionPrefix, rdf.Expression)
	}
	graph := builder.Build()
	return withoutRootNamespace(graph), nil
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
	id            string
	typ           string
	expression    bool
	qualifiedName bool
	members       []apiJSONMember
	indexMember   string
	indexOwner    string
}

var apiJSONInteger = regexp.MustCompile(`^-?[0-9]+$`)

const apiJSONEstimatedTriplesPerElement = 10

// apiJSONExpressionIDs indexes the element identities in a first pass, so the
// second pass can stream their properties directly into the graph.
func apiJSONExpressionIDs(data []byte) (map[string]bool, int, error) {
	var objects []apiJSONElementData
	if err := apiJSONElementsOf(data, apiJSONIndexObjectOf, func(object apiJSONElementData) error {
		objects = append(objects, object)
		return nil
	}); err != nil {
		return nil, 0, err
	}
	index, err := newAPIJSONIndex(objects)
	if err != nil {
		return nil, 0, err
	}
	index.classifyExpressions(objects)
	return index.expressionIDs, len(objects), nil
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

// classifyExpressions classifies the expression namespace to a fixpoint: a
// node is an expression when its metaclass is one the encoder mints directly
// under a declaration, or when its parent is an expr: node or an
// expression-class element — a child may be listed before its parent.
func (index *apiJSONIndex) classifyExpressions(objects []apiJSONElementData) {
	for changed := true; changed; {
		changed = false
		for i := range objects {
			object := &objects[i]
			if object.expression || object.qualifiedName || !index.expression(object) {
				continue
			}
			object.expression = true
			index.expressionIDs[object.id] = true
			changed = true
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
func apiJSONIndexObjectOf(dec *json.Decoder) (apiJSONElementData, error) {
	object := apiJSONElementData{}
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
		switch {
		case key == "@type", key == "@id":
			value, err := apiJSONValueOf(dec)
			if err != nil {
				return object, fmt.Errorf("%s: %w", key, err)
			}
			text, ok := value.(string)
			if !ok {
				return object, fmt.Errorf("%q is a string, not %v", key, value)
			}
			if key == "@type" {
				object.typ = text
			} else {
				object.id = text
			}
		case strings.HasPrefix(key, "@"):
			return object, fmt.Errorf("the key %q is not a keyword this document carries", key)
		case key == "qualifiedName":
			value, err := apiJSONValueOf(dec)
			if err != nil {
				return object, fmt.Errorf("%s: %w", key, err)
			}
			text, ok := value.(string)
			object.qualifiedName = ok && text != ""
		case membershipMemberProperty(key), membershipOwnerProperty(key):
			value, err := apiJSONValueOf(dec)
			if err != nil {
				return object, fmt.Errorf("%s: %w", key, err)
			}
			if id, ok := memberReference(value); ok {
				if membershipMemberProperty(key) {
					object.indexMember = id
				} else {
					object.indexOwner = id
				}
			}
		default:
			if err := skipAPIJSONValue(dec); err != nil {
				return object, fmt.Errorf("%s: %w", key, err)
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return object, err
	}
	return object, nil
}

func skipAPIJSONValue(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		seen := map[string]bool{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("an object member key is a string, not %v", keyToken)
			}
			if seen[key] {
				return fmt.Errorf("object states %q twice", key)
			}
			seen[key] = true
			if err := skipAPIJSONValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	case json.Delim('['):
		for dec.More() {
			if err := skipAPIJSONValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	default:
		return nil
	}
}

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
		value, err := apiJSONValueOf(dec)
		if err != nil {
			return object, fmt.Errorf("%s: %w", key, err)
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
			if key == "qualifiedName" {
				text, ok := value.(string)
				object.qualifiedName = ok && text != ""
			}
			object.members = append(object.members, apiJSONMember{key: key, value: value})
		}
	}
	if _, err := dec.Token(); err != nil {
		return object, err
	}
	return object, nil
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
	types      map[string]rdf.Term
	predicates map[string]apiJSONPredicateValue
	references map[apiJSONReferenceKey]rdf.Term
	literals   map[string]string
}

func newAPIJSONTermCache() *apiJSONTermCache {
	return &apiJSONTermCache{
		types:      map[string]rdf.Term{},
		predicates: map[string]apiJSONPredicateValue{},
		references: map[apiJSONReferenceKey]rdf.Term{},
		literals:   map[string]string{},
	}
}

func (cache *apiJSONTermCache) literal(value string) string {
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
func apiJSONTriples(graph *rdf.GraphBuilder, subject rdf.Term, predicate rdf.Term, sysmlKey string, value any, expressionIDs map[string]bool, cache *apiJSONTermCache, annotations *[]apiJSONAnnotation) error {
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
		return apiJSONCollection(graph, subject, predicate, sysmlKey, v, expressionIDs, cache, annotations)
	default:
		object, err := apiJSONScalarOf(subject, predicate, sysmlKey, value, expressionIDs, cache)
		if err != nil {
			return err
		}
		graph.Add(subject, predicate, object)
		return nil
	}
}

// nonuniqueCollections are the derived KerML collections declared {nonunique}:
// a relationship's related elements may repeat (an association of two ends on one type).
var nonuniqueCollections = map[string]bool{
	"relatedElement": true, "relatedType": true, "relatedFeature": true,
	"chainingFeature": true, "source": true, "target": true,
}

// apiJSONCollection states an array member: one triple per value, recording
// a collection of at least two on a sysml: key for its json: annotation.
func apiJSONCollection(graph *rdf.GraphBuilder, subject rdf.Term, predicate rdf.Term, sysmlKey string, values []any, expressionIDs map[string]bool, cache *apiJSONTermCache, annotations *[]apiJSONAnnotation) error {
	if len(values) == 0 {
		return nil
	}
	members := make([]rdf.Term, 0, len(values))
	for _, value := range values {
		member, err := apiJSONScalarOf(subject, predicate, sysmlKey, value, expressionIDs, cache)
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
			if nonuniqueCollections[name] {
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
