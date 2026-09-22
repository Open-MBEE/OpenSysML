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
// same elements back.
func ReadAPIJSON(data []byte) (*rdf.Graph, error) {
	elements, expressionIDs, err := parseAPIJSON(data)
	if err != nil {
		return nil, err
	}
	graph := rdf.NewGraph()
	// The collection annotations are stated after every element's triples, the
	// positions AnnotateCollections writes them in.
	var annotations []apiJSONAnnotation
	for _, element := range elements {
		var subject rdf.Term
		if element.expression {
			subject = rdf.IRI(rdf.Expression + element.id)
		} else {
			subject = rdf.ReferenceIRI(rdf.Term{}, element.id)
		}
		typ, err := apiJSONTypeIRI(element.typ)
		if err != nil {
			return nil, fmt.Errorf("element %q: %w", element.id, err)
		}
		graph.Add(subject, rdf.IRI(rdf.RDFType), typ)
		for _, member := range element.members {
			predicate, sysmlKey, err := apiJSONPredicate(member.key)
			if err != nil {
				return nil, fmt.Errorf("element %q: %w", element.id, err)
			}
			if err := apiJSONTriples(graph, subject, predicate, sysmlKey, member.value, expressionIDs, &annotations); err != nil {
				return nil, fmt.Errorf("element %q: %w", element.id, err)
			}
		}
	}
	for _, annotation := range annotations {
		text, err := rdf.CollectionJSON(annotation.subject, annotation.members)
		if err != nil {
			return nil, err
		}
		graph.Add(annotation.subject, rdf.AnnotationJSONTerm(annotation.key), rdf.String(text))
	}
	if len(expressionIDs) > 0 {
		graph.Prefixes[rdf.ExpressionPrefix] = rdf.Expression
	}
	return graph, nil
}

// apiJSONElementData is one parsed element object: its identity, its class as
// an expression node, and its properties in written order.
type apiJSONElementData struct {
	id            string
	typ           string
	expression    bool
	qualifiedName bool
	members       []apiJSONMember
}

var apiJSONInteger = regexp.MustCompile(`^-?[0-9]+$`)

// parseAPIJSON decodes the document into element objects, preserving member
// order, and returns the ids classified into the expression namespace, since
// references resolve against the whole document.
func parseAPIJSON(data []byte) ([]apiJSONElementData, map[string]bool, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	start, err := dec.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read the API element document: %w", err)
	}
	var objects []apiJSONElementData
	switch start {
	case json.Delim('['):
		for dec.More() {
			if token, err := dec.Token(); err != nil || token != json.Delim('{') {
				return nil, nil, fmt.Errorf("an API element array holds element objects, not %v", token)
			}
			object, err := apiJSONObjectOf(dec)
			if err != nil {
				return nil, nil, err
			}
			objects = append(objects, object)
		}
		if _, err := dec.Token(); err != nil {
			return nil, nil, fmt.Errorf("cannot read the API element array: %w", err)
		}
	case json.Delim('{'):
		object, err := apiJSONObjectOf(dec)
		if err != nil {
			return nil, nil, err
		}
		objects = append(objects, object)
	default:
		return nil, nil, fmt.Errorf("an API element document is an array of element objects or a single object, not %v", start)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, nil, fmt.Errorf("the API element document holds more than one JSON value")
	}
	ids := map[string]bool{}
	for _, object := range objects {
		if object.id == "" {
			return nil, nil, fmt.Errorf("an element object needs a non-empty string \"@id\"")
		}
		if object.typ == "" {
			return nil, nil, fmt.Errorf("element %q needs a string \"@type\"", object.id)
		}
		if ids[object.id] {
			return nil, nil, fmt.Errorf("the id %q names two element objects", object.id)
		}
		ids[object.id] = true
	}
	expressionIDs := map[string]bool{}
	for i, object := range objects {
		if isExpressionID(object.id, ids, object.qualifiedName) {
			objects[i].expression = true
			expressionIDs[object.id] = true
		}
	}
	return objects, expressionIDs, nil
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
		return rdf.OpenSysMLTerm(name), nil
	}
	if strings.Contains(typ, ":") {
		return rdf.Term{}, fmt.Errorf("the \"@type\" %q carries a prefix this mapping does not know", typ)
	}
	return rdf.SysMLTerm(typ), nil
}

// isExpressionID classifies an element id: an expression-namespace IRI is only
// ever minted by ExpressionIRI(owner, position) from an owner that is itself a
// subject, and expression nodes carry no qualifiedName; an element id never
// ends in a lone '_', so a `_p` inside an encoded name never splits.
func isExpressionID(id string, ids map[string]bool, hasQualifiedName bool) bool {
	if hasQualifiedName {
		return false
	}
	id, _ = strings.CutSuffix(id, rdf.OwningMembershipSuffix)
	return rdf.SplitExpressionNodeID(id, func(owner string) bool { return ids[owner] })
}

// apiJSONPredicate maps a member key to its predicate IRI, reporting the sysml:
// local name for the collection annotation it may carry.
func apiJSONPredicate(key string) (rdf.Term, string, error) {
	if name, ok := strings.CutPrefix(key, "sysx:"); ok {
		return rdf.OpenSysMLTerm(name), "", nil
	}
	if strings.Contains(key, ":") {
		return rdf.Term{}, "", fmt.Errorf("the key %q carries a prefix this mapping does not know", key)
	}
	return rdf.SysMLTerm(key), key, nil
}

// apiJSONAnnotation is a collection awaiting its json: literal: the members
// a sysml: array stated, settled once every triple stands.
type apiJSONAnnotation struct {
	subject rdf.Term
	key     string
	members []rdf.Term
}

// apiJSONTriples states one property of an element object as graph triples.
func apiJSONTriples(graph *rdf.Graph, subject rdf.Term, predicate rdf.Term, sysmlKey string, value any, expressionIDs map[string]bool, annotations *[]apiJSONAnnotation) error {
	switch v := value.(type) {
	case nil:
		// The API serves absent properties as null; nothing to state.
		return nil
	case map[string]any:
		target, err := apiJSONReferenceTarget(subject, v, expressionIDs)
		if err != nil {
			return err
		}
		graph.Add(subject, predicate, target)
		return nil
	case []any:
		return apiJSONCollection(graph, subject, predicate, sysmlKey, v, expressionIDs, annotations)
	default:
		object, err := apiJSONScalarOf(subject, predicate, sysmlKey, value, expressionIDs)
		if err != nil {
			return err
		}
		graph.Add(subject, predicate, object)
		return nil
	}
}

// apiJSONReferenceTarget resolves a {"@id": <id>} member object into the IRI it
// names from subject.
func apiJSONReferenceTarget(subject rdf.Term, object map[string]any, expressionIDs map[string]bool) (rdf.Term, error) {
	id, ok := object["@id"].(string)
	if !ok || len(object) != 1 || id == "" {
		return rdf.Term{}, fmt.Errorf("an object value is a reference {\"@id\": <id>}")
	}
	target := rdf.ReferenceIRI(subject, id)
	if targetID, ok := rdf.SubjectID(target); ok && expressionIDs[targetID] {
		target = rdf.IRI(rdf.Expression + targetID)
	}
	return target, nil
}

// apiJSONCollection states an array member: one triple per value, recording
// the collection on a sysml: key for its json: annotation.
func apiJSONCollection(graph *rdf.Graph, subject rdf.Term, predicate rdf.Term, sysmlKey string, values []any, expressionIDs map[string]bool, annotations *[]apiJSONAnnotation) error {
	if len(values) == 0 {
		return nil
	}
	members := make([]rdf.Term, 0, len(values))
	for _, value := range values {
		member, err := apiJSONScalarOf(subject, predicate, sysmlKey, value, expressionIDs)
		if err != nil {
			return err
		}
		members = append(members, member)
	}
	for i, member := range members {
		for _, earlier := range members[:i] {
			if member.Equal(earlier) {
				name := sysmlKey
				if name == "" {
					name = rdf.LocalName(predicate.Value)
				}
				return fmt.Errorf("the array on %s of <%s> repeats the member %s", name, subject.Value, member)
			}
		}
		graph.Add(subject, predicate, member)
	}
	if sysmlKey != "" {
		*annotations = append(*annotations, apiJSONAnnotation{subject: subject, key: sysmlKey, members: members})
	}
	return nil
}

// apiJSONScalarOf turns one JSON value into the term a triple holds: a
// reference object an IRI, a bool or number its typed literal, a string a plain
// literal — or the expression text a reference-valued property carries.
func apiJSONScalarOf(subject rdf.Term, predicate rdf.Term, sysmlKey string, value any, expressionIDs map[string]bool) (rdf.Term, error) {
	switch v := value.(type) {
	case bool:
		return rdf.Bool(v), nil
	case json.Number:
		lexical := v.String()
		if apiJSONInteger.MatchString(lexical) {
			return rdf.TypedLiteral(lexical, rdf.XSD+"integer"), nil
		}
		return rdf.TypedLiteral(lexical, realDatatype(lexical)), nil
	case string:
		if apiJSONIsExpressionText(sysmlKey, v) {
			return rdf.TypedLiteral(v, rdf.OpenSysML+dtExpression), nil
		}
		return rdf.String(v), nil
	case map[string]any:
		return apiJSONReferenceTarget(subject, v, expressionIDs)
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
