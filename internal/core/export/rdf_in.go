package export

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// sysmlPrefix qualifies a SysML vocabulary property as a diagnostic names it.
const sysmlPrefix = "sysml:"

// annotationMetaclasses are the elements whose notation is a comment body,
// which terminates the declaration by itself.
var annotationMetaclasses = map[string]bool{
	"Comment":               true,
	"Documentation":         true,
	"TextualRepresentation": true,
}

// element is one subject of the graph, read into the shape the printer needs.
type element struct {
	iri         string
	metaclass   string
	memberIndex int
	// trailing marks a result expression the graph gives no index: it follows
	// every indexed member, as the grammar has it.
	trailing bool
	// qname is the element's sysml:qualifiedName: the mutable label a
	// reference is written back as. Identity is the element id, not the name.
	// An element stating none is named by its position, as the encoder names it.
	qname string
	// elementID is the element's sysml:elementId — its identity, which an
	// ElementId annotation may have declared independently of the name.
	elementID string
	// declaredID marks an id that came from an explicit ElementId annotation,
	// which the notation must state again: it may equal the derived id.
	declaredID bool
	// ProjectRef provenance of a scope root, written back as an annotation.
	projectID, branch, org string
	// scope is the qualified name of the namespace this element is declared
	// in, which is what a reference written inside it is relative to.
	scope    string
	owner    *element
	children []*element
	// prefix is written ahead of the declaration, for a member a succession
	// attached itself to (`then send Show(x) to screen;`).
	prefix string
	// expressions holds the notation of each expression-valued property, keyed
	// by predicate, resolved from the expression graph the property points at.
	expressions map[string]string
}

// ToSysML converts an RDF graph back into SysML v2 source text. An element
// comes back as its sysx:sourceText, byte for byte, while that still states
// what the graph states, and in canonical notation otherwise.
//
// A subject whose metaclass this mapping does not know, or which lacks the
// properties needed to rebuild its declaration, is reported as an
// UnsupportedError: a converted file that dropped an element would be worse
// than a failed conversion.
func ToSysML(graph *rdf.Graph) ([]byte, error) {
	if graph == nil || graph.Len() == 0 {
		return nil, &UnsupportedError{What: "an empty graph", Note: "nothing to convert"}
	}
	if err := checkExtensionNamespace(graph); err != nil {
		return nil, err
	}
	if err := checkSupersededPredicates(graph); err != nil {
		return nil, err
	}
	metaclasses, err := checkTypes(graph)
	if err != nil {
		return nil, err
	}
	graph, err = rdf.ReconcileCollections(graph)
	if err != nil {
		var malformed *rdf.AnnotationError
		if errors.As(err, &malformed) {
			return nil, &UnsupportedError{What: fmt.Sprintf("the annotation json:%s of <%s>", malformed.Key, malformed.Subject), Note: malformed.Note}
		}
		return nil, err
	}
	if err := checkLiterals(graph, metaclasses); err != nil {
		return nil, err
	}
	if err := checkCardinality(graph); err != nil {
		return nil, err
	}
	if err := checkValueFlags(graph); err != nil {
		return nil, err
	}
	// The first rendering writes every reference fully qualified; reading it
	// chooses each the shortest spelling that reaches its element. Later
	// renderings are re-read the same way until every spelling still does.
	first := newDecoder(graph, metaclasses, nil)
	text, roots, err := first.notation()
	if err != nil {
		return nil, err
	}
	name, ok := first.candidateName(roots)
	if !ok {
		name = "<converted>"
	}
	names, _, err := chooseNames(name, text, first.wanted, nil)
	if err != nil {
		return nil, err
	}
	for {
		d := newDecoder(graph, metaclasses, names)
		if text, _, err = d.notation(); err != nil {
			return nil, err
		}
		revised, changed, err := chooseNames(name, text, d.wanted, names)
		if err != nil {
			return nil, err
		}
		if !changed {
			return text, nil
		}
		names = revised
	}
}

func newDecoder(graph *rdf.Graph, metaclasses map[rdf.Term]string, names *nameChoices) *decoder {
	d := &decoder{
		graph:            graph,
		metaclasses:      metaclasses,
		byIRI:            map[string]*element{},
		byID:             map[string]*element{},
		dupID:            map[string]bool{},
		memberships:      map[string]membership{},
		owningMembership: map[string]membership{},
		prefixed:         map[*element]bool{},
		names:            names,
		wanted:           newWanted(),
		demoted:          map[*element]bool{},
		demotedExpr:      map[string]bool{},
		folded:           map[*element]*element{},
	}
	return d
}

// notation writes the graph and returns its roots with the text.
func (d *decoder) notation() ([]byte, []*element, error) {
	roots, err := d.build()
	if err != nil {
		return nil, nil, err
	}
	d.nl = d.newline()
	text, err := d.render(roots)
	if err != nil {
		return nil, nil, err
	}
	return []byte(text), roots, nil
}

// checkExtensionNamespace refuses a graph written with the pre-rename extension
// namespace. Its properties would otherwise read as absent and the elements they
// describe would be written back without them.
func checkExtensionNamespace(graph *rdf.Graph) error {
	for _, triple := range graph.Triples() {
		for _, term := range []rdf.Term{triple.Subject, triple.Predicate, triple.Object} {
			if term.Kind == rdf.TermIRI && strings.HasPrefix(term.Value, rdf.LegacyExtension) {
				return legacyNamespaceError(term.Value)
			}
		}
		if strings.HasPrefix(triple.Object.Datatype, rdf.LegacyExtension) {
			return legacyNamespaceError(triple.Object.Datatype)
		}
	}
	return nil
}

func legacyNamespaceError(iri string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the term <%s>", iri),
		Note: fmt.Sprintf("it is in the pre-rename extension namespace %s, which this version does not read; convert the model from source again to write %s", rdf.LegacyExtension, rdf.OpenSysML),
	}
}

// supersededPredicates are properties an earlier version wrote for metadata
// annotations and portions, each with the term that carries the same fact now.
var supersededPredicates = map[string]string{
	rdf.OpenSysML + "prefixMetadata": "an owned sysml:MetadataUsage with sysx:declaredKeyword \"#\"",
	rdf.SysML + "annotates":          sysmlPrefix + pAnnotatedElement,
	rdf.SysML + "isSnapshot":         sysmlPrefix + pPortionKind + " \"snapshot\"",
	rdf.SysML + "isTimeslice":        sysmlPrefix + pPortionKind + " \"timeslice\"",
}

// checkSupersededPredicates refuses a graph stating a fact with a predicate
// this version no longer reads, which would otherwise be dropped.
func checkSupersededPredicates(graph *rdf.Graph) error {
	for _, triple := range graph.Triples() {
		now, superseded := supersededPredicates[triple.Predicate.Value]
		if !superseded || triple.Predicate.Kind != rdf.TermIRI {
			continue
		}
		return &UnsupportedError{
			What: fmt.Sprintf("the property <%s> of <%s>", triple.Predicate.Value, triple.Subject.Value),
			Note: fmt.Sprintf("an earlier version wrote this fact this way; it is now %s, which this version reads, so convert the model from source again", now),
		}
	}
	return nil
}

// checkTypes settles the one metaclass each subject is written as, keyed by
// subject. An rdf:type that is no term of the SysML vocabulary or of this
// mapping's extension is refused: a class of another vocabulary names no
// metaclass, whatever its local name. A subject stating several classes is
// the one of them that is a subclass of every other; rdf:type statements are
// unordered, so the classes are gathered before any is judged.
func checkTypes(graph *rdf.Graph) (map[rdf.Term]string, error) {
	stated := map[rdf.Term][]string{}
	var subjects []rdf.Term
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.RDFType {
			continue
		}
		class := triple.Object
		if !class.IsIRI() || !isVocabularyTerm(class.Value) {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the subject <%s>", triple.Subject.Value),
				Note: fmt.Sprintf("its rdf:type %s is not a class of the SysML vocabulary (%s) or of this mapping's extension (%s), so it names no metaclass to write", class.String(), rdf.SysML, rdf.OpenSysML),
			}
		}
		if _, seen := stated[triple.Subject]; !seen {
			subjects = append(subjects, triple.Subject)
		}
		stated[triple.Subject] = append(stated[triple.Subject], class.Value)
	}
	metaclasses := make(map[rdf.Term]string, len(stated))
	for _, subject := range subjects {
		class, ok := mostSpecific(stated[subject])
		if !ok {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the subject <%s>", subject.Value),
				Note: fmt.Sprintf("its rdf:types %s include none that is a subclass of all the others, so they name no single metaclass to write", classList(stated[subject])),
			}
		}
		metaclasses[subject] = class
	}
	return metaclasses, nil
}

// mostSpecific picks the class among those stated that every other is a
// superclass of, reporting false when there is none.
func mostSpecific(classes []string) (string, bool) {
	for _, class := range classes {
		specific := true
		for _, other := range classes {
			if !subclassOf(class, other) {
				specific = false
				break
			}
		}
		if specific {
			return class, true
		}
	}
	return "", false
}

// subclassOf reports whether the class iri is ancestor or a subclass of it in
// the SysML ontology; a class of this mapping's extension has no superclass.
func subclassOf(class, ancestor string) bool {
	return class == ancestor || strings.HasPrefix(class, rdf.SysML) && strings.HasPrefix(ancestor, rdf.SysML) &&
		ontology.IsAncestorOrSelf(rdf.LocalName(class), rdf.LocalName(ancestor))
}

// metaclass returns the local name of the class subject is written as, or ""
// when it states none.
func (d *decoder) metaclass(subject rdf.Term) string {
	return rdf.LocalName(d.metaclasses[subject])
}

// isVocabularyTerm reports whether iri is a namespace this mapping reads followed
// by a bare local name, so that the name the decoder classifies by is the term.
func isVocabularyTerm(iri string) bool {
	local := rdf.LocalName(iri)
	return local != "" && (iri == rdf.SysML+local || iri == rdf.OpenSysML+local)
}

// checkLiterals refuses a literal whose datatype its property does not take
// ("3"^^xsd:integer as a name) or whose text is outside it ("false"^^xsd:int).
func checkLiterals(graph *rdf.Graph, metaclasses map[rdf.Term]string) error {
	for _, triple := range graph.Triples() {
		object := triple.Object
		if !object.IsLiteral() || !mappingPredicate(triple.Predicate.Value) {
			continue
		}
		if object.Lang != "" {
			return literalError(triple, "a language-tagged literal is an rdf:langString, and no property this mapping reads takes one")
		}
		allowed := literalDatatypes(rdf.LocalName(metaclasses[triple.Subject]), triple.Predicate.Value)
		if !slices.Contains(allowed, object.Datatype) {
			return literalError(triple, fmt.Sprintf("%s takes %s", curie(triple.Predicate.Value), datatypeList(allowed)))
		}
		if !inLexicalSpace(object.Datatype, object.Value) {
			return literalError(triple, fmt.Sprintf("%q is not in the lexical space of %s", object.Value, curie(object.Datatype)))
		}
		if object.Datatype == rdf.XSD+"int" {
			if _, err := strconv.ParseInt(object.Value, 10, 32); err != nil {
				return literalError(triple, fmt.Sprintf("%q is outside the value space of xsd:int, -2147483648 to 2147483647", object.Value))
			}
		}
		if isIndexProperty(triple.Predicate.Value) {
			if n, err := strconv.ParseInt(object.Value, 10, strconv.IntSize); err != nil || n < 0 {
				return literalError(triple, fmt.Sprintf("an index is a position counted from 0 up to %d, and %s is not one this tool can order by", math.MaxInt, object.Value))
			}
		}
	}
	return nil
}

// multiValuedProperties are the sysx: properties the decoder reads every value
// of. Every other sysx: property is read once, so a second value is dropped.
var multiValuedProperties = map[string]bool{
	xBodyMember:     true,
	xBodyParameter:  true,
	xDeferredEvent:  true,
	xEffectMember:   true,
	xRelatedFeature: true,
}

// singleValued reports a property the decoder reads one value of: every sysx:
// property not listed above, the ontology's boolean `is…` flags and its portion kind.
func singleValued(predicate string) bool {
	name := rdf.LocalName(predicate)
	switch {
	case strings.HasPrefix(predicate, rdf.OpenSysML):
		return !multiValuedProperties[name]
	case strings.HasPrefix(predicate, rdf.SysML):
		return strings.HasPrefix(name, "is") || name == pPortionKind
	}
	return false
}

// checkCardinality refuses a subject stating a single-valued property twice
// with different objects, since only one of them could be written.
func checkCardinality(graph *rdf.Graph) error {
	type statement struct{ subject, predicate rdf.Term }
	first := map[statement]rdf.Term{}
	for _, triple := range graph.Triples() {
		predicate := triple.Predicate.Value
		if !singleValued(predicate) {
			continue
		}
		key := statement{triple.Subject, triple.Predicate}
		if seen, ok := first[key]; !ok {
			first[key] = triple.Object
		} else if seen != triple.Object {
			return &UnsupportedError{
				What: fmt.Sprintf("the subject <%s>", triple.Subject.Value),
				Note: fmt.Sprintf("it states %s twice, as %s and %s, and the property holds one value, so one of them would be dropped",
					curie(predicate), termText(seen), termText(triple.Object)),
			}
		}
	}
	return nil
}

// checkValueFlags refuses sysml:isDefault or sysml:isInitial, whatever its value,
// on a subject with no sysml:value, since the flags spell a feature value's operator.
func checkValueFlags(graph *rdf.Graph) error {
	for _, triple := range graph.Triples() {
		predicate := triple.Predicate.Value
		if predicate != rdf.SysML+pIsDefault && predicate != rdf.SysML+pIsInitial {
			continue
		}
		if graph.HasProperty(triple.Subject, rdf.SysML+pValue) {
			continue
		}
		return &UnsupportedError{
			What: fmt.Sprintf("the subject <%s>", triple.Subject.Value),
			Note: fmt.Sprintf("it states %s without a sysml:value, and the flag is the operator of a feature value, so there is nothing to write it on", curie(predicate)),
		}
	}
	return nil
}

func isIndexProperty(iri string) bool {
	if !strings.HasPrefix(iri, rdf.OpenSysML) {
		return false
	}
	switch rdf.LocalName(iri) {
	case xMemberIndex, xArgumentIndex, xEndIndex:
		return true
	}
	return false
}

// termText spells a term as Turtle does, for a diagnostic.
func termText(term rdf.Term) string {
	if term.IsIRI() {
		return "<" + term.Value + ">"
	}
	literal := rdf.String(term.Value).String()
	switch {
	case term.Lang != "":
		return literal + "@" + term.Lang
	case term.Datatype != "":
		return literal + "^^" + curie(term.Datatype)
	}
	return literal
}

// Lexical spaces per XML Schema Part 2 §3.3; owl:real, which defines none,
// takes a finite xsd:double's.
var (
	booleanLexical = regexp.MustCompile(`^(true|false|1|0)$`)
	integerLexical = regexp.MustCompile(`^[+-]?[0-9]+$`)
	decimalLexical = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)$`)
	realLexical    = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)
	doubleLexical  = regexp.MustCompile(`^([+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?|[+-]?INF|NaN)$`)
)

func inLexicalSpace(datatype, value string) bool {
	switch datatype {
	case rdf.XSD + "boolean":
		return booleanLexical.MatchString(value)
	case rdf.XSD + "integer", rdf.XSD + "int":
		return integerLexical.MatchString(value)
	case rdf.XSD + "decimal":
		return decimalLexical.MatchString(value)
	case rdf.OWL + "real":
		return realLexical.MatchString(value)
	case rdf.XSD + "double", rdf.XSD + "float":
		return doubleLexical.MatchString(value)
	}
	return true
}

func literalError(triple rdf.Triple, why string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the literal %s stated by <%s> %s", termText(triple.Object), triple.Subject.Value, curie(triple.Predicate.Value)),
		Note: why,
	}
}

func mappingPredicate(iri string) bool {
	return strings.HasPrefix(iri, rdf.SysML) || strings.HasPrefix(iri, rdf.OpenSysML)
}

// The datatypes a literal may carry, by what its property holds: "" is a plain
// literal, notation a name or expression text standing in for an element.
var (
	stringLiterals   = []string{"", rdf.XSD + "string"}
	notationLiterals = []string{"", rdf.XSD + "string", rdf.OpenSysML + dtExpression}
	booleanLiterals  = []string{rdf.XSD + "boolean"}
	integerLiterals  = []string{rdf.XSD + "integer", rdf.XSD + "int"}
	realLiterals     = []string{rdf.XSD + "decimal", rdf.OWL + "real", rdf.XSD + "double", rdf.XSD + "float"}
	boundLiterals    = append(append([]string{}, notationLiterals...), integerLiterals...)
)

// literalDatatypes lists the datatypes a literal of the property may carry on a
// subject of the metaclass: the ontology's range where the metaclass declares
// the property, else what this mapping writes there.
func literalDatatypes(metaclass, predicate string) []string {
	name := rdf.LocalName(predicate)
	if strings.HasPrefix(predicate, rdf.SysML) {
		if declared := declaredLiteralDatatypes(metaclass, name); declared != nil {
			return declared
		}
	}
	switch {
	case isIndexProperty(predicate):
		return integerLiterals
	case strings.HasPrefix(name, "is"), name == xHasBody, name == xDeclaredID, name == xBracedEffect:
		return booleanLiterals
	case strings.HasPrefix(predicate, rdf.SysML) && (name == pLowerBound || name == pUpperBound):
		// A feature's bound is an Expression the notation also states as a bare number.
		return boundLiterals
	case strings.HasPrefix(predicate, rdf.SysML):
		return notationLiterals
	}
	return stringLiterals
}

// declaredLiteralDatatypes reads the ontology's range for a property on the
// metaclass, or on every metaclass declaring it when the subject's class is
// none the ontology knows. A known class that does not declare the property
// carries it as this mapping writes it, so nil is returned for the caller's default.
func declaredLiteralDatatypes(metaclass, name string) []string {
	_, known := ontology.LookupClass(metaclass)
	var allowed []string
	for _, property := range ontology.LookupProperty(name) {
		if known && !ontology.IsAncestorOrSelf(metaclass, property.DefiningClass) {
			continue
		}
		allowed = append(allowed, rangeLiterals(property)...)
	}
	return slices.Compact(slices.Sorted(slices.Values(allowed)))
}

func rangeLiterals(property ontology.Property) []string {
	if property.Kind == ontology.ObjectProperty {
		return notationLiterals
	}
	switch property.Range {
	case rdf.XSD + "boolean":
		return booleanLiterals
	case rdf.XSD + "int":
		return integerLiterals
	case rdf.OWL + "real":
		return realLiterals
	}
	return stringLiterals
}

// datatypeList words the datatypes literalDatatypes lists: a plain literal and
// xsd:string as one.
func datatypeList(datatypes []string) string {
	var names []string
	if slices.Contains(datatypes, "") {
		names = append(names, "a string")
	}
	for _, datatype := range datatypes {
		if datatype != "" && datatype != rdf.XSD+"string" {
			names = append(names, curie(datatype))
		}
	}
	return strings.Join(names, " or ")
}

// curie abbreviates an IRI with the prefix the mapping writes it under.
func curie(iri string) string {
	for _, ns := range [...]struct{ prefix, iri string }{
		{"sysml", rdf.SysML}, {"sysx", rdf.OpenSysML}, {"xsd", rdf.XSD}, {"owl", rdf.OWL},
	} {
		if strings.HasPrefix(iri, ns.iri) {
			return ns.prefix + ":" + strings.TrimPrefix(iri, ns.iri)
		}
	}
	return "<" + iri + ">"
}

// membership is one materialized membership of the graph: the namespace it
// belongs to and the member it owns. A membership is not written back as a
// declaration — the notation states it by nesting the member in its owner — so
// it is read as the ownership edge it stands for rather than as an element.
type membership struct {
	iri    string
	owner  string
	member string
}

type decoder struct {
	graph *rdf.Graph
	// metaclasses is the class each subject is written as, settled by checkTypes.
	metaclasses map[rdf.Term]string
	byIRI       map[string]*element
	// byID keys the subjects on their element id, which is their identity; a
	// scoped graph may repeat an id across scopes, and dupID marks those.
	byID  map[string]*element
	dupID map[string]bool
	// memberships is keyed by membership IRI, and owningMembership by the IRI of
	// the member each one owns.
	memberships      map[string]membership
	owningMembership map[string]membership
	// prefixed marks the elements whose head wrote their `#M` annotations.
	prefixed map[*element]bool
	// names is the spelling chosen for each reference; while nil, references are
	// written fully qualified. wanted notes what was written for chooseNames.
	names  *nameChoices
	wanted *wanted
	// printed and usedExpr are written as source text in this pass and rebuilt
	// canonically; demoted and demotedExpr had their text proved stale earlier.
	printed     map[*element]bool
	rebuilt     map[*element]bool
	demoted     map[*element]bool
	usedExpr    map[string]bool
	demotedExpr map[string]bool
	// folded maps a succession written as the `then` ahead of its target to
	// that target, whose notation states it.
	folded map[*element]*element
	// written records where each element landed in this pass's notation, the
	// members of one ahead of it.
	written []writing
	// nl is the line ending rebuilt notation is written with.
	nl string
}

// newline is the line ending most of the stored element text uses, as the
// formatter decides it; an expression's text lies inside its element's.
func (d *decoder) newline() string {
	crlf, lf := 0, 0
	for _, subject := range d.graph.Subjects() {
		if d.isExpressionNode(subject) {
			continue
		}
		for _, property := range []string{xSourceText, xSourceTail} {
			if text, ok := d.graph.Lexical(subject, rdf.OpenSysML+property); ok {
				crlf += strings.Count(text, "\r\n")
				lf += strings.Count(text, "\n")
			}
		}
	}
	if crlf > lf-crlf {
		return "\r\n"
	}
	return "\n"
}

// writing is the range of the notation one element was written over.
type writing struct {
	el    *element
	where region
}

// build reads every subject into an element and links it to its owner,
// returning the elements that have no owner in the graph. Memberships are read
// first: they are what tells an owned Expression from an expression node, and
// what owns an element whose graph states ownership from the membership alone.
func (d *decoder) build() ([]*element, error) {
	var (
		order []*element
		roots []*element
	)
	for _, subject := range d.graph.Subjects() {
		if d.isMembership(subject) {
			if err := d.readMembership(subject); err != nil {
				return nil, err
			}
		}
	}
	for _, subject := range d.graph.Subjects() {
		if d.isMembership(subject) || d.isExpressionNode(subject) {
			// A node of an expression graph belongs to the declaration that holds
			// the expression, not to an element of its own.
			continue
		}
		metaclass := d.metaclass(subject)
		if metaclass == "" {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the subject <%s>", subject.Value),
				Note: "it has no rdf:type, so there is no way to tell what to write",
			}
		}
		el := &element{
			iri:         subject.Value,
			metaclass:   metaclass,
			memberIndex: intOf(d.graph, subject, rdf.OpenSysML+xMemberIndex),
		}
		el.trailing = !d.graph.HasProperty(subject, rdf.OpenSysML+xMemberIndex) && d.isResultExpression(el)
		el.qname, _ = d.stringOf(el, rdf.SysML+pQualifiedName)
		// The identity key. An old graph without sysml:elementId is keyed on
		// the encoding of its name, which is what its IRIs carry.
		if id, ok := d.stringOf(el, rdf.SysML+pElementID); ok {
			el.elementID = id
		} else {
			el.elementID = rdf.EncodeElementID(el.qname)
		}
		el.declaredID = d.boolOf(el, rdf.OpenSysML+xDeclaredID)
		el.projectID, _ = d.stringOf(el, rdf.OpenSysML+xProjectID)
		el.branch, _ = d.stringOf(el, rdf.OpenSysML+xBranch)
		el.org, _ = d.stringOf(el, rdf.OpenSysML+xOrg)
		d.byIRI[el.iri] = el
		if prior, seen := d.byID[el.elementID]; seen && prior != el {
			d.dupID[el.elementID] = true
		} else {
			d.byID[el.elementID] = el
		}
		order = append(order, el)
	}
	if err := d.checkMembershipEnds(); err != nil {
		return nil, err
	}
	for _, el := range order {
		parent, err := d.ownerOf(el)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			roots = append(roots, el)
			continue
		}
		el.owner = parent
		parent.children = append(parent.children, el)
	}
	if err := d.checkReferences(); err != nil {
		return nil, err
	}
	sortByIndex(roots)
	for _, el := range order {
		sortByIndex(el.children)
	}
	nameMembers(roots, "")
	if err := d.checkReachable(roots, order); err != nil {
		return nil, err
	}
	return roots, nil
}

// isMembership reports whether a subject states ownership rather than a
// declaration of its own: an OwningMembership with no qualified name. One with
// a name, such as a state's entry membership, is written as the member it is.
func (d *decoder) isMembership(subject rdf.Term) bool {
	metaclass := d.metaclass(subject)
	return metaclass != "" && ontology.IsAncestorOrSelf(metaclass, mOwningMembership) &&
		!d.graph.HasProperty(subject, rdf.SysML+pQualifiedName)
}

// readMembership records the ownership edge an OwningMembership stands for. Both
// ends are stated twice in the abstract syntax — once under the membership's own
// name for the property and once under the Relationship's — and either spelling
// is accepted, since a graph from another tool may carry only one; spellings
// that disagree, a literal end, or a second membership claiming the member are
// refused, since each would drop an edge.
func (d *decoder) readMembership(subject rdf.Term) error {
	what := fmt.Sprintf("the membership <%s>", subject.Value)
	owner, hasOwner, err := d.agreedObject(subject, what, "owning namespace", pMembershipOwningNamespace, pOwningRelatedElement)
	if err != nil {
		return err
	}
	member, hasMember, err := d.agreedObject(subject, what, "member", pMemberElement, pOwnedMemberElement, pOwnedMemberFeature, pOwnedVariantUsage, pOwnedResultExpression, pOwnedRelatedElement)
	if err != nil {
		return err
	}
	if !hasOwner || !hasMember {
		return &UnsupportedError{
			What: fmt.Sprintf("the membership <%s>", subject.Value),
			Note: "a membership states the namespace it belongs to in sysml:membershipOwningNamespace and the element it owns in sysml:memberElement, and this one states one of them or neither",
		}
	}
	m := membership{iri: subject.Value, owner: owner.Value, member: member.Value}
	if other, claimed := d.owningMembership[m.member]; claimed && other.iri != m.iri {
		return &UnsupportedError{
			What: fmt.Sprintf("the membership <%s>", subject.Value),
			Note: fmt.Sprintf("it and <%s> both own <%s>, and an element has one owning membership, so one of them would be dropped", other.iri, m.member),
		}
	}
	d.memberships[m.iri] = m
	d.owningMembership[m.member] = m
	return nil
}

// agreedObject returns the one object the subject, described by what, states
// under any of properties — spellings of the single-valued end named end — or
// an error when they differ or one is a literal.
func (d *decoder) agreedObject(subject rdf.Term, what, end string, properties ...string) (rdf.Term, bool, error) {
	var agreed rdf.Term
	found := false
	for _, property := range properties {
		for _, object := range d.graph.Objects(subject, rdf.SysML+property) {
			switch {
			case !object.IsIRI():
				return rdf.Term{}, false, &UnsupportedError{
					What: what,
					Note: fmt.Sprintf("its %s is the literal %s, and its %s is an element in the graph", curie(rdf.SysML+property), rdf.String(object.Value).String(), end),
				}
			case !found:
				agreed, found = object, true
			case object != agreed:
				return rdf.Term{}, false, &UnsupportedError{
					What: what,
					Note: fmt.Sprintf("it states both <%s> and <%s> as its %s, and every spelling of that end (%s) must name the one element, or one of them is dropped",
						agreed.Value, object.Value, end, curieList(properties)),
				}
			}
		}
	}
	return agreed, found, nil
}

func curieList(properties []string) string {
	curies := make([]string, len(properties))
	for i, property := range properties {
		curies[i] = curie(rdf.SysML + property)
	}
	return strings.Join(curies, ", ")
}

func classList(iris []string) string {
	curies := make([]string, len(iris))
	for i, iri := range iris {
		curies[i] = curie(iri)
	}
	return strings.Join(curies, ", ")
}

// ownerOf returns the element that owns el, or nil when it is a root, reading
// the element's owning membership, the membership's claim, or a bare owner
// triple; spellings that disagree are refused rather than one dropped.
func (d *decoder) ownerOf(el *element) (*element, error) {
	subject, what := rdf.IRI(el.iri), fmt.Sprintf("the element <%s>", el.iri)
	relationship, hasRelationship, err := d.agreedObject(subject, what, "owning relationship", pOwningMembership, pOwningRelationship)
	if err != nil {
		return nil, err
	}
	owner, hasOwner, err := d.agreedObject(subject, what, "owner", pOwningRelatedElement, pOwningNamespace, pOwner)
	if err != nil {
		return nil, err
	}
	ownerIRI := ""
	m, owned := d.owningMembership[el.iri]
	switch {
	case hasRelationship:
		// The owning relationship is either a membership standing between the
		// element and its owner, or the owner itself when a relationship owns
		// the element directly, as a state owns its entry action.
		if owned && m.iri != relationship.Value {
			return nil, &UnsupportedError{
				What: what,
				Note: fmt.Sprintf("it states <%s> as its owning relationship while the membership <%s> owns it, and following one would drop the other", relationship.Value, m.iri),
			}
		}
		if m, known := d.memberships[relationship.Value]; known {
			ownerIRI = m.owner
		} else {
			ownerIRI = relationship.Value
		}
	case owned:
		ownerIRI = m.owner
	case hasOwner:
		// A relationship a namespace declares — an import, a dependency, a
		// membership — states the element that owns it rather than a membership.
		ownerIRI = owner.Value
	default:
		return nil, nil
	}
	if hasOwner && owner.Value != ownerIRI {
		return nil, &UnsupportedError{
			What: what,
			Note: fmt.Sprintf("it states <%s> as its owner while its owning relationship puts it under <%s>, and following one would drop the other", owner.Value, ownerIRI),
		}
	}
	parent, known := d.byIRI[ownerIRI]
	if !known {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the element <%s>", el.iri),
			Note: fmt.Sprintf("its owning namespace <%s> is not in the graph", ownerIRI),
		}
	}
	return parent, nil
}

// referenceProperties are the predicates whose IRI objects reference elements,
// which must be graph subjects carrying sysml:qualifiedName.
var referenceProperties = func() map[string]bool {
	set := map[string]bool{
		rdf.SysML + pOwningNamespace:  true,
		rdf.SysML + pOwner:            true,
		rdf.SysML + pOwnedMember:      true,
		rdf.SysML + pSourceFeature:    true,
		rdf.SysML + pTargetFeature:    true,
		rdf.SysML + pClient:           true,
		rdf.SysML + pSupplier:         true,
		rdf.SysML + pAliasFor:         true,
		rdf.SysML + pAnnotatedElement: true,
		// The ends a succession reaches by position, which are elements of the
		// graph rather than names a reference could be written from.
		rdf.OpenSysML + xSourceMember: true,
		rdf.OpenSysML + xTargetMember: true,
	}
	for _, property := range relationshipProperty {
		set[rdf.SysML+property] = true
	}
	return set
}()

// checkReferences refuses a graph whose element references cannot be named
// from the graph itself, which would otherwise be written back mis-named.
func (d *decoder) checkReferences() error {
	for _, triple := range d.graph.Triples() {
		if triple.Object.Kind != rdf.TermIRI || !referenceProperties[triple.Predicate.Value] {
			continue
		}
		if _, err := d.referencedElement(triple.Object.Value); err != nil {
			return err
		}
	}
	return nil
}

// referencedElement resolves a referenced IRI to the graph subject whose
// sysml:qualifiedName names it: by IRI first, then by the element id the IRI
// ends in. An id the graph does not define is a dangling reference.
func (d *decoder) referencedElement(iri string) (*element, error) {
	target, ok := d.byIRI[iri]
	if !ok {
		id := rdf.LocalName(iri)
		target, ok = d.byID[id]
		if !ok || d.dupID[id] {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the reference <%s>", iri),
				Note: fmt.Sprintf("the graph defines no element with id %q, so there is no sysml:qualifiedName to write its name back from", id),
			}
		}
	}
	if !d.graph.HasProperty(rdf.IRI(target.iri), rdf.SysML+pQualifiedName) {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the element <%s>", target.iri),
			Note: "it is referenced but carries no sysml:qualifiedName, which is where a reference's name is read from",
		}
	}
	return target, nil
}

// checkMembershipEnds refuses a membership whose end is no element of the graph:
// the member would be left out of the output and the edge lost with it.
func (d *decoder) checkMembershipEnds() error {
	for _, subject := range d.graph.Subjects() {
		m, ok := d.memberships[subject.Value]
		if !ok {
			continue
		}
		for _, end := range []struct{ name, iri string }{{"owning namespace", m.owner}, {"member", m.member}} {
			if _, known := d.byIRI[end.iri]; !known {
				return &UnsupportedError{
					What: fmt.Sprintf("the membership <%s>", m.iri),
					Note: fmt.Sprintf("its %s <%s> is not an element of the graph, so the membership would be dropped", end.name, end.iri),
				}
			}
		}
	}
	return nil
}

// checkReachable reports an element that no root owns, which happens when
// ownership forms a cycle. Printing walks down from the roots, so such an
// element would be left out of the output without this check.
func (d *decoder) checkReachable(roots, all []*element) error {
	seen := make(map[string]bool, len(all))
	var walk func(el *element)
	walk = func(el *element) {
		if seen[el.iri] {
			return
		}
		seen[el.iri] = true
		for _, child := range el.children {
			walk(child)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	for _, el := range all {
		if !seen[el.iri] {
			return &UnsupportedError{
				What: fmt.Sprintf("the element <%s>", el.iri),
				Note: "no root owns it, so its sysml:owningNamespace chain forms a cycle",
			}
		}
	}
	return nil
}

// nameMembers scopes each member in its owner and names one the graph leaves
// unnamed by its position, as the encoder names it when the notation is read
// back, so that a reference it writes is keyed as it reads.
func nameMembers(members []*element, owner string) {
	for i, el := range members {
		el.scope = owner
		if el.qname == "" {
			el.qname = qualify(owner, "", i)
		}
		nameMembers(el.children, el.qname)
	}
}

// sortByIndex orders members by sysx:memberIndex, a result expression stated
// without one after them all; members alike in both keep the graph's order.
func sortByIndex(elements []*element) {
	sort.SliceStable(elements, func(i, j int) bool {
		a, b := elements[i], elements[j]
		if a.trailing != b.trailing {
			return b.trailing
		}
		return a.memberIndex < b.memberIndex
	})
}

// print writes one element and, recursively, its members, recording where in
// the notation it was written.
func (d *decoder) print(b *strings.Builder, el *element, depth int) error {
	start := b.Len()
	err := d.printElement(b, el, depth)
	d.written = append(d.written, writing{el, region{start, b.Len()}})
	return err
}

func (d *decoder) printElement(b *strings.Builder, el *element, depth int) error {
	indent := strings.Repeat("    ", depth)
	lead := indent + el.prefix
	if text, ok := d.verbatim(el); ok {
		// The text writes the `#` prefixes itself, so they are only checked here;
		// one disagreeing with the graph demotes the text like any other triple.
		if _, err := d.prefixWords(el); err != nil {
			return err
		}
		// The member's lines as written, members, notes and prefix included.
		b.WriteString(text)
		d.printed[el] = true
		// The members carry their own text; the tail closes the body after them.
		if tail, ok := d.graph.Lexical(rdf.IRI(el.iri), rdf.OpenSysML+xSourceTail); ok {
			children, err := d.bodyMembers(el)
			if err != nil {
				return err
			}
			for _, child := range children {
				if err := d.print(b, child, depth+1); err != nil {
					return err
				}
			}
			b.WriteString(tail)
		}
		return nil
	}
	d.rebuilt[el] = true
	if handled, err := d.printBehavior(b, el, lead, depth); handled {
		if err != nil {
			return err
		}
		// The behavioral writer prints a whole declaration with no place for `member`.
		if d.typeFeatureMember(el) {
			return d.typeFeatureUnwritable(el, "its notation is written whole by the behavioral mapping")
		}
		return d.unwrittenPrefix(el)
	}
	head, err := d.head(el)
	if err != nil {
		return err
	}
	b.WriteString(lead + head)
	if err := d.unwrittenPrefix(el); err != nil {
		return err
	}
	if annotationMetaclasses[el.metaclass] || d.isResultExpression(el) {
		// A comment, doc or rep declaration ends with its comment body, and a
		// result expression is bare: neither takes a terminator.
		b.WriteString(d.nl)
		return nil
	}
	children, err := d.bodyMembers(el)
	if err != nil {
		return err
	}
	// `parallel` marks a state's substates orthogonal, and only a body may
	// follow it, so a parallel state with none has no notation.
	parallel := d.boolOf(el, rdf.SysML+"isParallel")
	annotations := identityAnnotations(el)
	if len(children) == 0 && len(annotations) == 0 && !d.boolOf(el, rdf.OpenSysML+xHasBody) {
		if parallel {
			return d.missing(el, "sysx:"+xHasBody, "a parallel state states its regions in a body")
		}
		b.WriteString(";" + d.nl)
		return nil
	}
	if parallel {
		b.WriteString(" parallel")
	}
	b.WriteString(" {" + d.nl)
	for _, annotation := range annotations {
		b.WriteString(indent + "    " + annotation + d.nl)
	}
	for _, child := range children {
		if err := d.print(b, child, depth+1); err != nil {
			return err
		}
	}
	b.WriteString(indent + "}" + d.nl)
	return nil
}

// bodyMembers lists the members written in an element's body, in order: a `#`
// prefix and an accept parameter are written into its head, and a succession
// stating no source of its own folds into the member it introduces as `then`.
func (d *decoder) bodyMembers(el *element) ([]*element, error) {
	children := d.bodyChildren(el)
	if accept := d.acceptParam(el); accept != nil {
		children = slices.DeleteFunc(children, func(child *element) bool { return child == accept })
	}
	return d.positionalSuccessions(children)
}

// identityAnnotations re-materializes the identity the graph states as the
// annotations the notation declares it with: a ProjectRef on a scope root,
// and an ElementId wherever the id is explicit or differs from the encoding
// of the qualified name — a rename must not turn into a new element.
func identityAnnotations(el *element) []string {
	var out []string
	if el.projectID != "" || el.branch != "" || el.org != "" {
		var fields []string
		for _, f := range []struct{ name, value string }{
			{"projectId", el.projectID}, {"branch", el.branch}, {"org", el.org},
		} {
			if f.value != "" {
				fields = append(fields, fmt.Sprintf("%s = %s;", f.name, lexer.StringText(f.value)))
			}
		}
		out = append(out, "@IdentityMetadata::ProjectRef { "+strings.Join(fields, " ")+" }")
	}
	if el.declaredID || (el.elementID != "" && el.elementID != rdf.EncodeElementID(el.qname)) {
		out = append(out, fmt.Sprintf("@IdentityMetadata::ElementId { id = %s; }", lexer.StringText(el.elementID)))
	}
	return out
}

// head builds the declaration text up to the body or terminator, with the
// `member` of a KerML TypeFeatureMember ahead of it where the membership states one.
func (d *decoder) head(el *element) (string, error) {
	head, err := d.declarationHead(el)
	if err != nil || !d.typeFeatureMember(el) {
		return head, err
	}
	return d.memberPrefixed(el, head)
}

// typeFeatureMember reports whether a type owns el, a feature, through a plain
// OwningMembership rather than a FeatureMembership (KerML.xtext TypeFeatureMember).
func (d *decoder) typeFeatureMember(el *element) bool {
	if el.owner == nil || el.metaclass == usageMetaclass[ast.UsageMetadata] ||
		!ontology.IsAncestorOrSelf(el.metaclass, "Feature") || !isType(el.owner.metaclass) {
		return false
	}
	// A variant is flagged as one whatever its membership is typed; an end's
	// cross feature is written in the end's head (ownedCrossFeature).
	if d.boolOf(el, rdf.SysML+"isVariant") || d.enumeratedValue(el) || d.ownedCrossFeature(el.owner) == el {
		return false
	}
	m, owned := d.owningMembership[el.iri]
	return owned && d.metaclass(rdf.IRI(m.iri)) == mOwningMembership
}

// memberPrefixed writes `member` between a head's visibility and its declaration
// (KerML.xtext TypeFeatureMember); SysML has no such keyword, so a SysML root refuses.
func (d *decoder) memberPrefixed(el *element, head string) (string, error) {
	if !d.kerml(el) {
		return "", d.typeFeatureUnwritable(el, "SysML has no `member` keyword, and writing it as a feature of the type would be a different model")
	}
	visibility := d.visibility(el)
	if visibility == "" {
		return "member " + head, nil
	}
	rest, ok := strings.CutPrefix(head, visibility+" ")
	if !ok {
		return "", d.typeFeatureUnwritable(el, "its head does not open with the visibility `member` follows")
	}
	return visibility + " member " + rest, nil
}

// typeFeatureUnwritable refuses a feature its type owns through a plain
// OwningMembership that the notation cannot state as such.
func (d *decoder) typeFeatureUnwritable(el *element, why string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the feature <%s>", el.iri),
		Note: fmt.Sprintf("its type owns it through a plain sysml:OwningMembership, which KerML writes `member`, but %s", why),
	}
}

// declarationHead builds the declaration text up to the body or terminator.
func (d *decoder) declarationHead(el *element) (string, error) {
	if d.isResultExpression(el) {
		return d.expressionNodeText(rdf.IRI(el.iri), el)
	}
	switch el.metaclass {
	case "Package", "Namespace":
		return d.namespaceHead(el)
	case "Import":
		return d.importHead(el)
	case mAlias:
		return d.aliasHead(el)
	case "Dependency":
		return d.dependencyHead(el)
	case "Specialization", "FeatureTyping", "Subsetting", "Redefinition",
		"FeatureInverting", "TypeFeaturing", "Conjugation", "Disjoining":
		return d.relationshipElementHead(el)
	case "Comment":
		return d.commentHead(el)
	case "Documentation":
		return d.documentationHead(el), nil
	case "TextualRepresentation":
		return d.representationHead(el)
	case mMultiplicity:
		return d.multiplicityHead(el)
	case mFilter:
		condition, ok := d.stringOf(el, rdf.OpenSysML+xFilter)
		if !ok {
			return "", d.missing(el, "sysx:"+xFilter, "a filter is its condition")
		}
		return "filter " + condition, nil
	case mConstraint:
		// A bare condition states no keyword of its own; it asserts implicitly.
		keyword, _ := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword)
		return d.conditionHead(el, keyword)
	case mAssume:
		return d.conditionHead(el, "assume")
	case mRequire:
		return d.conditionHead(el, "require")
	}
	// Every form of a metadata usage is typed by its one definition
	// (SysML.xtext MetadataUsageDeclaration), whichever keyword writes it.
	if el.metaclass == usageMetaclass[ast.UsageMetadata] {
		if _, err := d.metadataDefinition(el); err != nil {
			return "", err
		}
		keyword, err := d.metadataKeyword(el)
		if err != nil {
			return "", err
		}
		switch keyword {
		case "@":
			return d.metadataHead(el)
		case "#":
			// A `#` prefix is written into its owner's head, so one reaching
			// here has no declaration to prefix.
			return "", &UnsupportedError{
				What: fmt.Sprintf("the prefix annotation <%s>", el.iri),
				Note: "a prefix is written ahead of the declaration that owns it, and this one is owned by no declaration",
			}
		}
	}
	// A succession carrying its ends as references is the one the parser builds
	// for a succession, written back as `succession first <source> then <target>;`.
	// sequences the two members it names wherever they are declared, so the
	// order survives the round trip. A `succession` declaration whose head was
	// kept verbatim never reaches here — print() writes its source text.
	// A `succession` declaration that states the form its ends are written in
	// is a head that binds ends, not an edge between two members.
	if el.metaclass == mSuccession && !d.statesEnds(el) {
		return d.successionHead(el)
	}
	// A control node, statement, state or region: the behavioral half of the
	// mapping writes the ones whose notation is a head and a terminator.
	if head, handled, err := d.behaviorHead(el); handled {
		return head, err
	}
	if kind, ok := metaclassDefinition[el.metaclass]; ok {
		return d.definitionHead(el, kind)
	}
	if kind, ok := metaclassUsage[el.metaclass]; ok {
		return d.usageHead(el, kind)
	}
	return "", &UnsupportedError{
		What: fmt.Sprintf("the element <%s> of type sysml:%s", el.iri, el.metaclass),
		Note: "this metaclass has no SysML notation in the conversion mapping",
	}
}

func (d *decoder) namespaceHead(el *element) (string, error) {
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	prefixes, err := d.prefixWords(el)
	if err != nil {
		return "", err
	}
	words = append(words, prefixes...)
	if el.metaclass == "Package" {
		if d.boolOf(el, rdf.OpenSysML+"isStandardLibraryPackage") {
			words = append(words, "standard")
		}
		if d.boolOf(el, rdf.OpenSysML+"isLibraryPackage") {
			words = append(words, "library")
		}
		words = append(words, "package")
	} else {
		words = append(words, "namespace")
	}
	words = append(words, d.identWords(el)...)
	return strings.Join(words, " "), nil
}

func (d *decoder) definitionHead(el *element, kind ast.DefinitionKind) (string, error) {
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if d.boolOf(el, rdf.SysML+"isAbstract") {
		words = append(words, "abstract")
	}
	// An enumeration definition is a variation by what it is, not by a keyword
	// (SysML v2 EnumerationDefinition); its isVariation writes nothing back.
	if d.boolOf(el, rdf.SysML+"isVariation") && kind != ast.DefEnumeration {
		words = append(words, "variation")
	}
	if d.boolOf(el, rdf.SysML+"isConstant") {
		words = append(words, constantKeyword(d.kerml(el)))
	}
	if d.boolOf(el, rdf.SysML+"isEvent") {
		words = append(words, "event")
	}
	// Prefix metadata ends the definition prefix, ahead of the kind keyword
	// (SysML.xtext DefinitionPrefix `BasicDefinitionPrefix? DefinitionExtensionKeyword*`).
	prefixes, err := d.prefixWords(el)
	if err != nil {
		return "", err
	}
	words = append(words, prefixes...)
	words = append(words, d.keywordOr(el, definitionKeyword(kind)))
	if d.boolOf(el, rdf.SysML+"isAll") {
		words = append(words, "all")
	}
	// Every definition kind but `metaclass` shares its keyword with a usage
	// form, and is told apart from it by `def`.
	if kind != ast.DefMetaclass {
		words = append(words, "def")
	}
	words = append(words, d.identWords(el)...)
	relationships, err := d.relationshipWords(el, "")
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	return strings.Join(words, " "), nil
}

// enumeratedValue reports whether el is an enumerated value: an enumeration
// usage owned by an enumeration definition, a variant by that ownership alone.
func (d *decoder) enumeratedValue(el *element) bool {
	return el.metaclass == usageMetaclass[ast.UsageEnumeration] &&
		el.owner != nil && el.owner.metaclass == definitionMetaclass[ast.DefEnumeration]
}

func (d *decoder) usageHead(el *element, kind ast.UsageKind) (string, error) {
	// A head that binds ends is written from the form it states; one relating
	// ends without a form is refused rather than written back without them.
	endForm, hasEnds := d.stringOf(el, rdf.OpenSysML+xEndForm)
	// A satisfy head names the requirement it subsets bare rather than through
	// a relatedFeature end, so its form states no ends.
	if endForm == formSatisfy {
		hasEnds = false
	}
	if !hasEnds && d.statesEnds(el) {
		return "", d.missing(el, "sysx:"+xEndForm,
			"the ends it relates are written in the form the head states")
	}
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if d.boolOf(el, rdf.SysML+"isAbstract") {
		words = append(words, "abstract")
	}
	// A result parameter is declared with `return`, which carries its out
	// direction: writing both would not parse.
	isResult := d.boolOf(el, rdf.SysML+"isResult")
	if isResult {
		words = append(words, "return")
	} else if direction, ok := d.stringOf(el, rdf.SysML+pDirection); ok {
		words = append(words, direction)
	}
	keyword := d.keywordOr(el, usageKeyword(kind))
	// An accept written without the `action` keyword its kind states carries
	// `accept` as the keyword it was written with; the shorthand writes it.
	if keyword == "accept" {
		keyword = ""
	}
	identWords := d.identWords(el)
	references, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelReferences])
	if err != nil {
		return "", err
	}
	portion, err := d.portionKind(el)
	if err != nil {
		return "", err
	}
	event := d.boolOf(el, rdf.SysML+"isEvent") || el.metaclass == mEventOccurrenceUsage
	// A `snapshot`, `timeslice`, `event` or `assert` keyword states a typed
	// fact; a spelling the typing contradicts is refused, not respelled.
	if err := d.keywordTyped(el, keyword, portion, event); err != nil {
		return "", err
	}
	kerml := d.kerml(el)
	isPortion, err := d.portionPrefix(el, kerml, portion)
	if err != nil {
		return "", err
	}
	for _, flag := range []struct {
		keyword string
		set     bool
	}{
		{"variation", d.boolOf(el, rdf.SysML+"isVariation")},
		// An enumerated value is a variant by what it is, not by a keyword
		// (SysML.xtext EnumerationUsageMember); its isVariant writes nothing back.
		{"variant", d.boolOf(el, rdf.SysML+"isVariant") && !d.enumeratedValue(el)},
		// `portion` is composite and stands in for `composite`
		// (KerML.xtext BasicFeaturePrefix `isComposite ?= 'composite' | isPortion ?= 'portion'`).
		{"portion", isPortion},
		{"composite", d.boolOf(el, rdf.SysML+"isComposite") && !isPortion},
		{"derived", d.boolOf(el, rdf.SysML+"isDerived")},
		{constantKeyword(kerml), d.boolOf(el, rdf.SysML+"isConstant")},
		{"individual", d.boolOf(el, rdf.SysML+"isIndividual")},
		{"snapshot", portion == "snapshot"},
		{"timeslice", portion == "timeslice"},
		{"event", event},
		{"end", d.boolOf(el, rdf.SysML+"isEnd")},
		{"ref", d.boolOf(el, rdf.SysML+"isReference")},
	} {
		// A keyword such as `snapshot` is both a modifier and a kind keyword;
		// writing it here as well as below would declare it twice.
		if flag.keyword == keyword {
			continue
		}
		if flag.set {
			words = append(words, flag.keyword)
		}
		// The cross feature an end owns is written right after `end`
		// (SysML.xtext EndUsagePrefix `'end' OwnedCrossFeatureMember?`).
		if flag.keyword == "end" {
			if cross := d.ownedCrossFeature(el); cross != nil {
				crossWords, err := d.crossFeatureWords(cross)
				if err != nil {
					return "", err
				}
				words = append(words, crossWords...)
			}
		}
	}
	// Prefix metadata ends the usage prefix, ahead of the kind keyword
	// (SysML.xtext UsagePrefix `UnextendedUsagePrefix UsageExtensionKeyword*`);
	// a subject, actor, stakeholder or objective takes it after its keyword
	// instead (SubjectUsage, ActorUsage, StakeholderUsage, ObjectiveRequirementUsage).
	prefixes, err := d.prefixWords(el)
	if err != nil {
		return "", err
	}
	// A prefix qualifies the kind keyword after it, and the `not` of
	// `assert not constraint c` negates the declaration that prefix introduces.
	// Negation on its own has no notation, so it is reported rather than dropped.
	prefix, hasPrefix := d.stringOf(el, rdf.OpenSysML+xDeclaredPrefix)
	negated := d.boolOf(el, rdf.SysML+"isNegated")
	// An asserted constraint's metaclass is its `assert`, which prefixes
	// `constraint` or, written as the keyword itself, stands in for it.
	asserted := false
	if el.metaclass == mAssertConstraintUsage {
		if hasPrefix && prefix != "assert" {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the asserted constraint <%s>", el.iri),
				Note: fmt.Sprintf("its sysx:%s %q is not the `assert` its metaclass states", xDeclaredPrefix, prefix),
			}
		}
		prefix, hasPrefix = "assert", true
		if keyword == "assert" {
			keyword, asserted = "", true
		}
	}
	switch {
	case hasPrefix:
		// `#M assert not constraint c` ends the occurrence prefix ahead of the
		// qualifying keyword (SysML.xtext AssertConstraintUsage, PerformActionUsage);
		// `assume #M constraint c` takes it after (RequirementConstraintUsage).
		if !annotationsFollowPrefix[prefix] {
			words = append(words, prefixes...)
			prefixes = nil
		}
		words = append(words, prefix)
		if negated {
			words = append(words, "not")
		}
	case negated:
		return "", d.missing(el, "sysx:"+xDeclaredPrefix, "the `not` of a negated declaration qualifies the prefix keyword it follows")
	}
	keywordAt := len(words)
	switch kind {
	case ast.UsageSubject, ast.UsageActor, ast.UsageStakeholder, ast.UsageObjective:
		if keyword == "" && len(prefixes) > 0 {
			return "", d.missing(el, "sysx:"+xDeclaredKeyword, "a prefix annotation on a "+usageKeyword(kind)+" follows its keyword")
		}
		words = append(words, keyword)
		words = append(words, prefixes...)
	default:
		words = append(words, prefixes...)
		keywordAt = len(words)
		if keyword != "" {
			words = append(words, keyword)
		}
	}
	// `chain` qualifies the kind keyword it follows, unlike the modifiers above.
	if d.boolOf(el, rdf.SysML+"isChain") {
		words = append(words, "chain")
	}
	if d.boolOf(el, rdf.SysML+"isAll") {
		words = append(words, "all")
	}
	// A `render`/`frame` reference writes its target as a bare name; without one the
	// member declares a usage, spelling out the kind keyword (SysML.xtext
	// ViewRenderingUsage, FramedConcernUsage) even when it declares no name.
	var skip []ast.RelationshipKind
	if endForm == formEquals {
		// The bound feature is an end of the binding, written by the ends
		// notation rather than as a `references` clause.
		skip = append(skip, ast.RelReferences)
	}
	// A satisfy head writes the requirement it subsets bare, after the keyword;
	// without that form it declares a requirement usage of its own.
	switch {
	case endForm == formSatisfy:
		targets, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelSubsets])
		if err != nil {
			return "", err
		}
		if len(targets) == 0 {
			return "", d.missing(el, sysmlPrefix+relationshipProperty[ast.RelSubsets],
				"a satisfy head names the requirement it satisfies")
		}
		words = append(words, strings.Join(targets, ", "))
		skip = append(skip, ast.RelSubsets)
	case kind == ast.UsageSatisfy:
		words = append(words, "requirement")
	}
	if noun := memberDeclarationKeyword(kind); noun != "" {
		targets, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelReferences])
		if err != nil {
			return "", err
		}
		if len(targets) > 0 {
			words = append(words, strings.Join(targets, ", "))
			skip = append(skip, ast.RelReferences)
		} else {
			words = append(words, noun)
		}
	}
	// `include` states the use case a case performs: `include <ref>;` names an
	// existing one, `include use case <name> : T` declares one that includes T
	// (SysML.xtext PerformedUseCaseUsage). Both carry the inclusion as a
	// relationship, which the keyword itself writes.
	included, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelIncludes])
	if err != nil {
		return "", err
	}
	if len(included) > 0 {
		skip = append(skip, ast.RelIncludes)
		if len(d.identWords(el)) == 0 {
			// `include <ref>;` states no kind keyword and takes the inclusion in
			// its place; the typing the parser derives from it is that same target.
			words = append(words[:keywordAt:keywordAt], "include", strings.Join(included, ", "))
			skip = append(skip, ast.RelTyping)
		} else {
			words = append(words[:keywordAt:keywordAt], append([]string{"include"}, words[keywordAt:]...)...)
		}
	}
	// A `perform` or a state's `entry`/`do`/`exit` names the action it performs,
	// declaring no name of its own (SysML.xtext PerformActionUsageDeclaration),
	// as `event m.start` and `assert c` name an occurrence or a constraint.
	referencing := referenceMemberKeyword(keyword) || keyword == "event" || asserted
	if referencing && len(identWords) > 0 && !asserted {
		// `event e;` names the `e` it refers to; a declared `e` spells its kind
		// keyword out (`event occurrence e;`), which the graph does not state.
		return "", &UnsupportedError{
			What: fmt.Sprintf("the `%s` declaration <%s>", keyword, el.iri),
			Note: fmt.Sprintf("it declares a name (sysml:declaredName), which `%s` written as the kind keyword cannot: `%s <name>` names the feature it refers to, and a declaration is written `%s %s <name>`, so the notation would come back as a reference to a different element", keyword, keyword, keyword, usageKeyword(kind)),
		}
	}
	if referencing && len(identWords) == 0 && len(references) == 0 {
		// With neither, `perform;` would come back as a feature named `perform`.
		written := keyword
		if asserted {
			written = "assert"
		}
		return "", &UnsupportedError{
			What: fmt.Sprintf("the `%s` declaration <%s>", written, el.iri),
			Note: fmt.Sprintf("it neither declares a name nor names the feature it refers to (sysml:references), the two shapes `%s` is written in, so the notation cannot be rebuilt from the graph", written),
		}
	}
	referenced := referencing && len(identWords) == 0
	if referenced {
		words = append(words, strings.Join(references, ", "))
		skip = append(skip, ast.RelReferences)
	}
	// The multiplicity part (`[1] ordered nonunique`) qualifies the type it
	// follows, so it goes with the typing clause and ahead of any further
	// specialization; with no type it follows the name (`x[2] redefines y`) or
	// the reference (`event m.start[1] redefines e`), and with neither it closes
	// the head (`:>> y[2]`).
	multPart := d.multiplicityText(el)
	if d.boolOf(el, rdf.SysML+"isOrdered") {
		multPart += " ordered"
	}
	if d.boolOf(el, rdf.SysML+"isNonunique") {
		multPart += " nonunique"
	}
	typed, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelTyping])
	if err != nil {
		return "", err
	}
	typedPart := ""
	namedMult := false
	switch {
	case len(typed) > 0:
		typedPart, multPart = multPart, ""
	case len(identWords) > 0 && multPart != "":
		identWords[len(identWords)-1] += multPart
		namedMult, multPart = true, ""
	case referenced && multPart != "":
		words[len(words)-1] += multPart
		multPart = ""
	}
	words = append(words, identWords...)
	// The accept shorthand writes its parameter into the head, ahead of the
	// `via` clause the parent's relationships supply.
	if accept := d.acceptParam(el); accept != nil {
		words = append(words, "accept")
		words = append(words, d.identWords(accept)...)
		acceptWords, err := d.relationshipWords(accept, "")
		if err != nil {
			return "", err
		}
		words = append(words, acceptWords...)
		// A trigger (`when`/`at`/`after` …) is what the payload accepts, written
		// in place of a type rather than as a value clause.
		if trigger, ok := d.stringOf(accept, rdf.SysML+pValue); ok {
			words = append(words, trigger)
		}
	}
	// `metadata M about x;` writes its typing bare (SysML.xtext MetadataUsageDeclaration).
	if kind == ast.UsageMetadata && len(identWords) == 0 && len(typed) == 1 {
		words = append(words, typed[0]+typedPart)
		typedPart = ""
		skip = append(skip, ast.RelTyping)
	}
	relationships, err := d.relationshipWords(el, typedPart, skip...)
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	if hasEnds {
		// A connector's own multiplicity is its declaration, written ahead of
		// the ends; after them it would read as the last end's.
		declared := multPart != "" || len(words) > keywordAt+1
		if declared && keywordAt < len(words) && words[keywordAt] == "bind" {
			// SysML's `bind` shorthand declares nothing; `bind [1] a = b` gives the
			// first end the `[1]`, so the declaration takes the `binding … bind` form.
			words[keywordAt] = "binding"
		}
		ends, err := d.endWords(el, endForm, declared)
		if err != nil {
			return "", err
		}
		if multPart != "" {
			words = append(words, strings.TrimSpace(multPart))
			multPart = ""
		}
		words = append(words, ends)
		// The `= value` of a binding is one of its ends, already written above.
		if endForm == formEquals {
			return strings.Join(words, " "), nil
		}
	}
	head := strings.Join(words, " ") + multPart
	value, hasValue := d.stringOf(el, rdf.SysML+pValue)
	// `assert c;`, `assert c[1]` and `assert c { … }` name the `c` they refer to;
	// a declared `c` is read only where a typing, specialization or value follows.
	if asserted && len(identWords) > 0 && !strings.HasPrefix(identWords[0], "<") &&
		(namedMult || (len(relationships) == 0 && !hasValue)) {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the `assert` declaration <%s>", el.iri),
			Note: "it declares a name (sysml:declaredName) that nothing but a body or a multiplicity follows, the shape in which `assert <name>` names the constraint it refers to, so the notation would come back as a reference to a different element; a declaration is written `assert constraint <name>`",
		}
	}
	if hasValue {
		head += " " + d.valueOperator(el) + " " + value
	}
	return head, nil
}

// valueOperator is the operator a feature value was written with, rebuilt from
// its isDefault and isInitial flags: `=`, `:=`, `default =` or `default :=`.
func (d *decoder) valueOperator(el *element) string {
	op := "="
	if d.boolOf(el, rdf.SysML+pIsInitial) {
		op = ":="
	}
	if d.boolOf(el, rdf.SysML+pIsDefault) {
		return "default " + op
	}
	return op
}

// conditionHead rebuilds a condition member from its properties: an inline
// condition (`assert x > 0`), the constraint it states (`require R`), or the
// constraint it declares (`assume constraint c : C default = v`, or a nested
// `constraint { … }`).
func (d *decoder) conditionHead(el *element, keyword string) (string, error) {
	var words []string
	if keyword != "" {
		words = append(words, keyword)
	}
	if d.boolOf(el, rdf.SysML+"isNegated") {
		words = append(words, "not")
	}
	// Prefix metadata follows the member keyword and introduces a constraint
	// declaration: `assume #goal constraint c` (RequirementConstraintUsage).
	prefixes, err := d.prefixWords(el)
	if err != nil {
		return "", err
	}
	if len(prefixes) > 0 && keyword == "" {
		return "", d.missing(el, "sysx:"+xDeclaredKeyword, "a prefix annotation on a condition follows its keyword")
	}
	words = append(words, prefixes...)
	references, err := d.referenceList(el, rdf.SysML+relationshipProperty[ast.RelReferences])
	if err != nil {
		return "", err
	}
	var skip []ast.RelationshipKind
	// The declaration form states its keyword; a graph written before it did
	// is one only where a body follows and nothing is referenced.
	hasBody := d.boolOf(el, rdf.OpenSysML+xHasBody)
	declared := hasBody && len(references) == 0
	if el.metaclass == mAssume || el.metaclass == mRequire {
		if written, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok {
			if written != "constraint" {
				return "", &UnsupportedError{
					What: fmt.Sprintf("the condition member <%s>", el.iri),
					Note: fmt.Sprintf("its sysx:%s %q is not a form of a %s member, which declares a `constraint` or states one bare", xDeclaredKeyword, written, keyword),
				}
			}
			declared = true
		}
	}
	switch condition, ok := d.stringOf(el, rdf.OpenSysML+xCondition); {
	case ok:
		if len(prefixes) > 0 {
			return "", d.unprefixedCondition(el, "an inline condition")
		}
		if err := d.inlineConditionOnly(el, keyword, declared, references, hasBody); err != nil {
			return "", err
		}
		words = append(words, condition)
		return strings.Join(words, " "), nil
	case declared:
		// The nested-constraint form spells out the kind it declares, so the
		// braces that follow are read as a constraint body rather than a name.
		words = append(words, "constraint")
		words = append(words, d.identWords(el)...)
	case len(references) > 0:
		if len(prefixes) > 0 {
			return "", d.unprefixedCondition(el, "a constraint reference")
		}
		// The constraint the member states comes first; any further `::>` it
		// declares follows with the other specializations.
		words = append(words, references[0])
		if rest := references[1:]; len(rest) > 0 {
			words = append(words, relationshipSyntax[ast.RelReferences], strings.Join(rest, ", "))
		}
		skip = append(skip, ast.RelReferences)
	default:
		return "", d.missing(el, "sysx:"+xCondition, "a condition member states a condition")
	}
	// The constraint usage's own head — specializations, then `[1]`, then
	// `= value` — in the order the requirement member parser reads it.
	relationships, err := d.relationshipWords(el, "", skip...)
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	head := strings.Join(words, " ") + d.multiplicityText(el)
	if value, ok := d.stringOf(el, rdf.SysML+pValue); ok {
		head += " " + d.valueOperator(el) + " " + value
	}
	return head, nil
}

// inlineConditionOnly refuses an inline condition that also carries facts of
// the declaration or reference forms, which writing the condition alone would drop.
func (d *decoder) inlineConditionOnly(el *element, keyword string, declared bool, references []string, hasBody bool) error {
	var extra []string
	if declared {
		extra = append(extra, "declares a `constraint`")
	}
	if hasBody || len(d.bodyChildren(el)) > 0 {
		extra = append(extra, "has a body")
	}
	if len(references) > 0 {
		extra = append(extra, "states a constraint through sysml:"+relationshipProperty[ast.RelReferences])
	}
	if len(d.identWords(el)) > 0 {
		extra = append(extra, "declares a name")
	}
	if relationships, err := d.relationshipWords(el, ""); err != nil {
		return err
	} else if len(relationships) > 0 {
		extra = append(extra, "declares specializations")
	}
	if d.multiplicityText(el) != "" {
		extra = append(extra, "declares a multiplicity")
	}
	if _, ok := d.stringOf(el, rdf.SysML+pValue); ok {
		extra = append(extra, "has a value")
	}
	if len(extra) == 0 {
		return nil
	}
	form := "condition"
	if keyword != "" {
		form = keyword
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the condition member <%s>", el.iri),
		Note: fmt.Sprintf("it states an inline condition (sysx:%s) and also %s; a %s member is written in one form, and writing the condition alone would drop the rest", xCondition, strings.Join(extra, ", "), form),
	}
}

// unprefixedCondition reports a prefix annotation on a condition member whose
// form has no prefix position; only a constraint declaration takes one.
func (d *decoder) unprefixedCondition(el *element, form string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the condition member <%s>", el.iri),
		Note: fmt.Sprintf("a prefix annotation qualifies a constraint declaration (`assume #M constraint c`), and this member states %s, which has no position for one", form),
	}
}

// isResultExpression reports whether el is the result expression of a body:
// the Expression a ResultExpressionMembership owns, written back bare.
func (d *decoder) isResultExpression(el *element) bool {
	m, owned := d.owningMembership[el.iri]
	return owned && d.metaclass(rdf.IRI(m.iri)) == mResultExpressionMembership
}

// acceptParam returns the synthetic parameter of an accept shorthand, whose
// notation belongs in its parent's declaration head.
func (d *decoder) acceptParam(el *element) *element {
	for _, child := range el.children {
		if d.boolOf(child, rdf.SysML+"isAccept") {
			return child
		}
	}
	return nil
}

// missing reports a graph element that cannot be written back as notation
// because a property its declaration is built from is absent.
func (d *decoder) missing(el *element, property, why string) error {
	return &UnsupportedError{
		What: fmt.Sprintf("the element <%s>", el.iri),
		Note: fmt.Sprintf("it has no %s, and %s, so no valid declaration can be written for it", property, why),
	}
}

func (d *decoder) importHead(el *element) (string, error) {
	var words []string
	// An expose is always protected and always imports all (SysML v2 8.3.26.2),
	// so its keyword states both: writing them as well does not parse.
	expose := d.boolOf(el, rdf.OpenSysML+xExpose)
	if keyword := d.visibility(el); keyword != "" && !expose {
		words = append(words, keyword)
	}
	if expose {
		words = append(words, "expose")
	} else {
		words = append(words, "import")
		if d.boolOf(el, rdf.SysML+pIsImportAll) {
			words = append(words, "all")
		}
	}
	imported, err := d.referenceText(el, rdf.SysML+pImportedNamespace)
	if err != nil {
		return "", err
	}
	if imported == "" {
		return "", d.missing(el, sysmlPrefix+pImportedNamespace, "an import names the namespace it imports")
	}
	// `P::*::**` imports the members of P recursively; `P::**` imports P itself
	// and, recursively, its members. Both flags may hold at once.
	if d.boolOf(el, rdf.OpenSysML+xNamespaceImport) {
		imported += "::*"
	}
	if d.boolOf(el, rdf.OpenSysML+xRecursive) {
		imported += "::**"
	}
	words = append(words, imported)
	if filter, ok := d.stringOf(el, rdf.OpenSysML+xFilter); ok {
		words = append(words, "["+filter+"]")
	}
	return strings.Join(words, " "), nil
}

func (d *decoder) aliasHead(el *element) (string, error) {
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	words = append(words, "alias")
	words = append(words, d.identWords(el)...)
	forName, err := d.referenceText(el, rdf.SysML+pAliasFor)
	if err != nil {
		return "", err
	}
	if forName == "" {
		return "", d.missing(el, sysmlPrefix+pAliasFor, "an alias names the element it stands for")
	}
	words = append(words, "for", forName)
	return strings.Join(words, " "), nil
}

func (d *decoder) dependencyHead(el *element) (string, error) {
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	prefixes, err := d.prefixWords(el)
	if err != nil {
		return "", err
	}
	words = append(words, prefixes...)
	words = append(words, "dependency")
	words = append(words, d.identWords(el)...)
	clients, err := d.referenceList(el, rdf.SysML+pClient)
	if err != nil {
		return "", err
	}
	suppliers, err := d.referenceList(el, rdf.SysML+pSupplier)
	if err != nil {
		return "", err
	}
	if len(clients) == 0 {
		return "", d.missing(el, sysmlPrefix+pClient, "a dependency runs from at least one client")
	}
	if len(suppliers) == 0 {
		return "", d.missing(el, sysmlPrefix+pSupplier, "a dependency runs to at least one supplier")
	}
	// `from` is what separates the clients from a name: without it, the first
	// client would be read as the dependency's own name.
	words = append(words, "from", strings.Join(clients, ", "))
	words = append(words, "to", strings.Join(suppliers, ", "))
	return strings.Join(words, " "), nil
}

// relationshipElementHead rebuilds a keyword-first relationship member from its
// ordered ends: `specialization Gen subtype A specializes B`.
func (d *decoder) relationshipElementHead(el *element) (string, error) {
	keyword, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword)
	if !ok {
		return "", d.missing(el, "sysx:"+xDeclaredKeyword,
			"a relationship member is written keyword-first, and the keyword says which form")
	}
	form, ok := relationshipMemberSyntax[keyword]
	if !ok {
		return "", &UnsupportedError{What: fmt.Sprintf("the relationship keyword %q of %s", keyword, el.iri)}
	}
	source, err := d.relationshipEndName(el, form.source)
	if err != nil {
		return "", err
	}
	target, err := d.relationshipEndName(el, form.target)
	if err != nil {
		return "", err
	}
	var words []string
	if keyword := d.visibility(el); keyword != "" {
		words = append(words, keyword)
	}
	if prefix, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredPrefix); ok {
		words = append(words, prefix)
		words = append(words, d.identWords(el)...)
		words = append(words, keyword)
	} else {
		words = append(words, keyword)
		words = append(words, d.identWords(el)...)
	}
	if keyword == "featuring" {
		// `featuring of f by T` names the relationship before its featured end.
		if len(d.identWords(el)) > 0 {
			words = append(words, "of")
		}
		return strings.Join(append(words, source, "by", target), " "), nil
	}
	return strings.Join(append(words, source, form.separator, target), " "), nil
}

// relationshipEndName reads one end of a relationship element, which the
// notation always writes.
func (d *decoder) relationshipEndName(el *element, property string) (string, error) {
	names, err := d.referenceList(el, rdf.SysML+property)
	if err != nil {
		return "", err
	}
	if len(names) != 1 {
		return "", d.missing(el, sysmlPrefix+property, "a relationship relates exactly one element at each end")
	}
	return names[0], nil
}

func (d *decoder) commentHead(el *element) (string, error) {
	// The keyword is what makes this a declared element rather than lexical
	// trivia, so it is written even when nothing identifies the comment.
	words := []string{"comment"}
	words = append(words, d.identWords(el)...)
	about, err := d.referenceList(el, rdf.SysML+pAnnotatedElement)
	if err != nil {
		return "", err
	}
	if len(about) > 0 {
		words = append(words, "about", strings.Join(about, ", "))
	}
	words = append(words, d.localeWords(el)...)
	body, _ := d.stringOf(el, rdf.SysML+pBody)
	return strings.Join(words, " ") + " /*" + body + "*/", nil
}

func (d *decoder) documentationHead(el *element) string {
	words := []string{"doc"}
	words = append(words, d.identWords(el)...)
	words = append(words, d.localeWords(el)...)
	body, _ := d.stringOf(el, rdf.SysML+pBody)
	return strings.Join(words, " ") + " /*" + body + "*/"
}

func (d *decoder) representationHead(el *element) (string, error) {
	words := []string{"rep"}
	words = append(words, d.identWords(el)...)
	language, ok := d.stringOf(el, rdf.SysML+pLanguage)
	if !ok {
		return "", d.missing(el, sysmlPrefix+pLanguage, "a textual representation states the language it is written in")
	}
	body, _ := d.stringOf(el, rdf.SysML+pBody)
	words = append(words, "language", lexer.StringText(language))
	return strings.Join(words, " ") + " /*" + body + "*/", nil
}

func (d *decoder) multiplicityHead(el *element) (string, error) {
	words := []string{"multiplicity"}
	words = append(words, d.identWords(el)...)
	if mult := d.multiplicityText(el); mult != "" {
		words = append(words, mult)
	}
	// A MultiplicitySubset states its bounds by subsetting instead of a range.
	subsets, err := d.referenceText(el, rdf.SysML+relationshipProperty[ast.RelSubsets])
	if err != nil {
		return "", err
	}
	if subsets != "" {
		words = append(words, "subsets", subsets)
	}
	return strings.Join(words, " "), nil
}

// A comment or doc declaration whose head carries a locale needs the `locale`
// keyword written back out.
func (d *decoder) localeWords(el *element) []string {
	locale, ok := d.stringOf(el, rdf.SysML+pLocale)
	if !ok {
		return nil
	}
	return []string{"locale", lexer.StringText(locale)}
}

// keywordOr returns the kind keyword the author wrote, falling back to the
// canonical one when the graph does not record a synonym.
func (d *decoder) keywordOr(el *element, canonical string) string {
	if written, ok := d.stringOf(el, rdf.OpenSysML+xDeclaredKeyword); ok && written != "" {
		return written
	}
	// A declaration that wrote no kind keyword takes its kind from its owner.
	if d.boolOf(el, rdf.OpenSysML+xImplicitKind) {
		return ""
	}
	return canonical
}

// keywordTyped checks that a keyword the graph types agrees with its typing:
// the portion kind, the event typing or the AssertConstraintUsage metaclass.
func (d *decoder) keywordTyped(el *element, keyword, portion string, event bool) error {
	var stated, expected string
	switch keyword {
	case "snapshot", "timeslice":
		if portion == keyword {
			return nil
		}
		stated, expected = sysmlPrefix+pPortionKind+" "+strconv.Quote(portion), strconv.Quote(keyword)
		if portion == "" {
			stated = "no " + sysmlPrefix + pPortionKind
		}
	case "event":
		if event {
			return nil
		}
		stated, expected = "the metaclass "+el.metaclass, mEventOccurrenceUsage
	case "assert":
		if el.metaclass == mAssertConstraintUsage {
			return nil
		}
		stated, expected = "the metaclass "+el.metaclass, mAssertConstraintUsage
	default:
		return nil
	}
	return &UnsupportedError{
		What: fmt.Sprintf("the `%s` declaration <%s>", keyword, el.iri),
		Note: fmt.Sprintf("it has %s, not the %s its keyword states, so the notation cannot be rebuilt without declaring something else", stated, expected),
	}
}

// kerml reports whether el is written under KerML's grammar: the one its root
// records, or SysML for a root recording none, which is how candidateName reads it.
func (d *decoder) kerml(el *element) bool {
	root := el
	for root.owner != nil {
		root = root.owner
	}
	language, _ := d.stringOf(root, rdf.OpenSysML+xSourceLanguage)
	return language == "kerml"
}

// constantKeyword spells isConstant for the grammar: KerML.xtext FeaturePrefix
// `isConstant ?= 'const'`, SysML.xtext RefPrefix `isConstant ?= 'constant'`.
func constantKeyword(kerml bool) string {
	if kerml {
		return "const"
	}
	return "constant"
}

// portionPrefix reports whether isPortion is written as KerML's `portion`; SysML has no
// such prefix, so there the flag is carried by a portion kind and refused without one.
func (d *decoder) portionPrefix(el *element, kerml bool, portion string) (bool, error) {
	if !d.boolOf(el, rdf.SysML+"isPortion") {
		return false, nil
	}
	switch {
	case kerml && !d.boolOf(el, rdf.SysML+"isComposite"):
		return false, &UnsupportedError{
			What: fmt.Sprintf("the portion <%s>", el.iri),
			Note: "it has sysml:isPortion without sysml:isComposite, and a portion is composite (KerML Feature::isPortion), so `portion` would declare more than the graph states",
		}
	case kerml:
		return true, nil
	case portion != "":
		return false, nil
	}
	return false, &UnsupportedError{
		What: fmt.Sprintf("the portion <%s>", el.iri),
		Note: "it has sysml:isPortion and no sysml:" + pPortionKind + ", and SysML declares a portion only as `snapshot` or `timeslice`, so no valid declaration can be written for it",
	}
}

// portionKind reads the portion a usage is declared as, `snapshot` or
// `timeslice`; any other value has no notation and is refused.
func (d *decoder) portionKind(el *element) (string, error) {
	portion, ok := d.stringOf(el, rdf.SysML+pPortionKind)
	if !ok {
		return "", nil
	}
	switch portion {
	case "snapshot", "timeslice":
		return portion, nil
	}
	return "", &UnsupportedError{
		What: fmt.Sprintf("the portion kind %q of <%s>", portion, el.iri),
		Note: sysmlPrefix + pPortionKind + " is `snapshot` or `timeslice`, the two portions the notation declares",
	}
}

func (d *decoder) identWords(el *element) []string {
	var words []string
	if short, ok := d.stringOf(el, rdf.SysML+pDeclaredShortName); ok {
		words = append(words, "<"+nameText(short)+">")
	}
	if name, ok := d.stringOf(el, rdf.SysML+pDeclaredName); ok {
		words = append(words, nameText(name))
	}
	return words
}

// nameText writes a name as the notation spells it: the graph carries the name
// itself, so one that is not a basic name needs its quotes back (KerML §8.2.2).
// A reserved word lexes as a keyword rather than a name, so a name spelling one
// needs the quotes too.
func nameText(name string) string {
	if lexer.IsIdentifier(name) && !lexer.IsKeyword(name) {
		return name
	}
	return "'" + escapeName(name) + "'"
}

// escapeName escapes a quote the name itself contains, which would otherwise
// close the unrestricted name early. The parser keeps the escapes a name was
// written with, so one already escaped is left alone.
func escapeName(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '\\':
			b.WriteByte(name[i])
			if i+1 < len(name) {
				i++
				b.WriteByte(name[i])
			}
		case '\'':
			b.WriteString(`\'`)
		default:
			b.WriteByte(name[i])
		}
	}
	return b.String()
}

// qualifiedNameText writes a qualified name segment by segment, since each
// segment is a name of its own and is quoted on its own.
func qualifiedNameText(qname string) string {
	global := strings.HasPrefix(qname, "$::")
	if global {
		qname = strings.TrimPrefix(qname, "$::")
	}
	segments := strings.Split(qname, "::")
	for i, segment := range segments {
		segments[i] = nameText(segment)
	}
	out := strings.Join(segments, "::")
	if global {
		return "$::" + out
	}
	return out
}

// metadataKeyword reads the keyword a metadata usage was written with: `#` for
// a prefix ahead of a declaration, `@` for a member of its own, and none for
// `metadata`. Any other keyword is refused rather than read as one of those;
// a repeated one is refused by checkCardinality before decoding starts.
func (d *decoder) metadataKeyword(el *element) (string, error) {
	keyword, ok := d.graph.Object(rdf.IRI(el.iri), rdf.OpenSysML+xDeclaredKeyword)
	if !ok {
		return "", nil
	}
	switch keyword := keyword.Value; keyword {
	case "#", "@":
		return keyword, nil
	default:
		return "", &UnsupportedError{
			What: fmt.Sprintf("the element <%s>", el.iri),
			Note: fmt.Sprintf("its sysx:%s %q is not a metadata form; a metadata usage is written as `metadata`, `@` or `#`", xDeclaredKeyword, keyword),
		}
	}
}

// metadataSigil reports the sigil a metadata usage was written with, or none
// for a `metadata` member and for a keyword its head will refuse.
func (d *decoder) metadataSigil(el *element) string {
	if el.metaclass != usageMetaclass[ast.UsageMetadata] {
		return ""
	}
	keyword, err := d.metadataKeyword(el)
	if err != nil {
		return ""
	}
	return keyword
}

// annotationsFollowPrefix lists the qualifying keywords written ahead of the
// `#M` annotations they own (RequirementConstraintUsage, KerML FeaturePrefix).
var annotationsFollowPrefix = map[string]bool{
	"assume":  true,
	"require": true,
	"var":     true,
}

// prefixWords writes the `#M` annotations a declaration owns as prefixes, which
// are written into its head rather than its body.
func (d *decoder) prefixWords(el *element) ([]string, error) {
	d.prefixed[el] = true
	var words []string
	for _, child := range el.children {
		if child.metaclass != usageMetaclass[ast.UsageMetadata] {
			continue
		}
		// A head kept as source text prints no members, so a keyword the
		// member form would refuse is refused here rather than dropped.
		keyword, err := d.metadataKeyword(child)
		if err != nil {
			return nil, err
		}
		if keyword != "#" {
			continue
		}
		definition, err := d.metadataDefinition(child)
		if err != nil {
			return nil, err
		}
		typed, err := d.referenceName(definition, child)
		if err != nil {
			return nil, err
		}
		// A prefix is spelled as its type alone (SysML.xtext PrefixMetadataUsage).
		about, err := d.referenceList(child, rdf.SysML+pAnnotatedElement)
		if err != nil {
			return nil, err
		}
		if len(d.identWords(child)) > 0 || len(about) > 0 || len(child.children) > 0 || d.boolOf(child, rdf.OpenSysML+xHasBody) {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the prefix annotation <%s>", child.iri),
				Note: "a prefix names only its metadata definition; a name, an about clause or a body is written by the `@` member form",
			}
		}
		words = append(words, "#"+typed)
	}
	return words, nil
}

// unwrittenPrefix reports a `#M` annotation on an element whose head has no
// place for one, so that it is refused rather than dropped.
func (d *decoder) unwrittenPrefix(el *element) error {
	if d.prefixed[el] {
		return nil
	}
	for _, child := range el.children {
		if d.metadataSigil(child) == "#" {
			return &UnsupportedError{
				What: fmt.Sprintf("the prefix annotation <%s>", child.iri),
				Note: fmt.Sprintf("it prefixes a sysml:%s, whose notation takes no prefix annotation", el.metaclass),
			}
		}
	}
	return nil
}

// bodyChildren returns the members written in an element's body: every child
// but the prefix annotations and the cross feature its head writes.
func (d *decoder) bodyChildren(el *element) []*element {
	cross := d.ownedCrossFeature(el)
	var out []*element
	for _, child := range el.children {
		if child != cross && d.metadataSigil(child) != "#" {
			out = append(out, child)
		}
	}
	return out
}

// ownedCrossFeature is the kindless feature an end owns through a plain OwningMembership,
// written in its head (KerML.xtext OwnedCrossingFeature); a keyworded one is a body `member`.
func (d *decoder) ownedCrossFeature(el *element) *element {
	if !d.boolOf(el, rdf.SysML+"isEnd") || !ontology.IsAncestorOrSelf(el.metaclass, "Feature") {
		return nil
	}
	for _, child := range el.children {
		if child.metaclass != crossFeatureMetaclass(d.kerml(el)) {
			continue
		}
		if m, owned := d.owningMembership[child.iri]; owned && d.metaclass(rdf.IRI(m.iri)) == mOwningMembership {
			return child
		}
	}
	return nil
}

// crossFeatureWords writes an end's cross feature after `end`: name, multiplicity
// and specializations, typing spelled `typed by` since `:` there is the end's own.
func (d *decoder) crossFeatureWords(cross *element) ([]string, error) {
	if len(cross.children) > 0 || d.boolOf(cross, rdf.OpenSysML+xHasBody) || len(identityAnnotations(cross)) > 0 {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the cross feature <%s>", cross.iri),
			Note: "it is written in the head of the end that owns it, which has no place for a body or an identity annotation",
		}
	}
	words := d.identWords(cross)
	mult := d.multiplicityText(cross)
	switch {
	case len(words) > 0:
		words[len(words)-1] += mult
	case mult != "":
		words = append(words, mult)
	}
	for _, flag := range []struct {
		property string
		keyword  string
	}{{"isOrdered", "ordered"}, {"isNonunique", "nonunique"}} {
		if d.boolOf(cross, rdf.SysML+flag.property) {
			words = append(words, flag.keyword)
		}
	}
	if len(words) == 0 {
		return nil, &UnsupportedError{
			What: fmt.Sprintf("the cross feature <%s>", cross.iri),
			Note: "it declares neither a name nor a multiplicity part, and one or the other introduces a cross feature ahead of its end",
		}
	}
	prefix, err := d.crossFeaturePrefixWords(cross)
	if err != nil {
		return nil, err
	}
	words = append(prefix, words...)
	typed, err := d.referenceList(cross, rdf.SysML+relationshipProperty[ast.RelTyping])
	if err != nil {
		return nil, err
	}
	if len(typed) > 0 {
		words = append(words, "typed by", strings.Join(typed, ", "))
	}
	relationships, err := d.relationshipWords(cross, "", ast.RelTyping)
	if err != nil {
		return nil, err
	}
	return append(words, relationships...), nil
}

// crossFeaturePrefixWords writes the prefix a cross feature owns ahead of its name
// (KerML.xtext OwnedCrossingFeature BasicFeaturePrefix, SysML.xtext BasicUsagePrefix),
// its flags spelled in the grammar of its root as a usage head's are.
func (d *decoder) crossFeaturePrefixWords(cross *element) ([]string, error) {
	kerml := d.kerml(cross)
	isPortion, err := d.portionPrefix(cross, kerml, "")
	if err != nil {
		return nil, err
	}
	var words []string
	if direction, ok := d.stringOf(cross, rdf.SysML+pDirection); ok {
		words = append(words, direction)
	}
	for _, flag := range []struct {
		keyword string
		set     bool
	}{
		{"derived", d.boolOf(cross, rdf.SysML+"isDerived")},
		{"abstract", d.boolOf(cross, rdf.SysML+"isAbstract")},
		{"variation", d.boolOf(cross, rdf.SysML+"isVariation")},
		{"portion", isPortion},
		{"composite", d.boolOf(cross, rdf.SysML+"isComposite") && !isPortion},
	} {
		if flag.set {
			words = append(words, flag.keyword)
		}
	}
	if prefix, ok := d.stringOf(cross, rdf.OpenSysML+xDeclaredPrefix); ok {
		words = append(words, prefix)
	}
	for _, flag := range []struct {
		keyword string
		set     bool
	}{
		{constantKeyword(kerml), d.boolOf(cross, rdf.SysML+"isConstant")},
		{"ref", d.boolOf(cross, rdf.SysML+"isReference")},
	} {
		if flag.set {
			words = append(words, flag.keyword)
		}
	}
	return words, nil
}

// metadataHead writes a metadata usage member: `@M`, `@ m : M`, with the
// elements it is about (SysML.xtext MetadataUsage).
func (d *decoder) metadataHead(el *element) (string, error) {
	definition, err := d.metadataDefinition(el)
	if err != nil {
		return "", err
	}
	typed, err := d.referenceName(definition, el)
	if err != nil {
		return "", err
	}
	head := "@" + typed
	if ident := d.identWords(el); len(ident) > 0 {
		head = "@ " + strings.Join(ident, " ") + " : " + typed
	}
	if keyword := d.visibility(el); keyword != "" {
		head = keyword + " " + head
	}
	about, err := d.referenceList(el, rdf.SysML+pAnnotatedElement)
	if err != nil {
		return "", err
	}
	if len(about) > 0 {
		head += " about " + strings.Join(about, ", ")
	}
	return head, nil
}

// metadataDefinition is the one metadata definition a metadata usage applies;
// a usage typed by none, by several, or by an element of another metaclass has
// no notation and is refused. A literal type is a name the graph does not
// define, so its metaclass cannot be checked; it must at least be a name.
func (d *decoder) metadataDefinition(el *element) (rdf.Term, error) {
	types := d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+relationshipProperty[ast.RelTyping])
	switch len(types) {
	case 1:
		if types[0].IsLiteral() {
			why := notAName(types[0])
			if types[0].Datatype == rdf.OpenSysML+dtExpression {
				why = "it is an expression, not a name"
			}
			if why != "" {
				return rdf.Term{}, &UnsupportedError{
					What: fmt.Sprintf("the element <%s>", el.iri),
					Note: fmt.Sprintf("its %s %s: %s, and a metadata usage names the one metadata definition it applies", sysmlPrefix+relationshipProperty[ast.RelTyping], types[0], why),
				}
			}
			return types[0], nil
		}
		target, err := d.referencedElement(types[0].Value)
		if err != nil {
			return rdf.Term{}, err
		}
		if target.metaclass != definitionMetaclass[ast.DefMetadata] && target.metaclass != definitionMetaclass[ast.DefMetaclass] {
			return rdf.Term{}, &UnsupportedError{
				What: fmt.Sprintf("the element <%s>", el.iri),
				Note: fmt.Sprintf("its %s <%s> is a sysml:%s, and a metadata usage names the one metadata definition it applies", sysmlPrefix+relationshipProperty[ast.RelTyping], target.iri, target.metaclass),
			}
		}
		return types[0], nil
	case 0:
		return rdf.Term{}, d.missing(el, sysmlPrefix+relationshipProperty[ast.RelTyping], "a metadata usage names the one metadata definition it applies")
	}
	return rdf.Term{}, &UnsupportedError{
		What: fmt.Sprintf("the element <%s>", el.iri),
		Note: fmt.Sprintf("it has %d %s, and a metadata usage names the one metadata definition it applies, so no valid declaration can be written for it", len(types), sysmlPrefix+relationshipProperty[ast.RelTyping]),
	}
}

// visibility reads the visibility a member was declared with, which the abstract
// syntax states on the membership rather than on the member — except on an
// import, which has a visibility of its own.
func (d *decoder) visibility(el *element) string {
	keyword, ok := d.stringOf(el, rdf.SysML+pVisibility)
	if !ok {
		if m, owned := d.owningMembership[el.iri]; owned {
			keyword, ok = d.graph.Lexical(rdf.IRI(m.iri), rdf.SysML+pVisibility)
		}
	}
	if !ok || keyword == "" {
		return ""
	}
	return visibilityKeyword(visibilityOf(keyword))
}

// relationshipWords renders the typing and specialization clauses of a
// declaration head, in the order the grammar expects. multPart, when given, is
// the multiplicity part the typing clause carries.
func (d *decoder) relationshipWords(el *element, multPart string, skip ...ast.RelationshipKind) ([]string, error) {
	var words []string
	for _, kind := range relationshipOrder {
		if slices.Contains(skip, kind) {
			continue
		}
		targets, err := d.referenceList(el, rdf.SysML+relationshipProperty[kind])
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			continue
		}
		// Conjugation qualifies the type a feature is typed by, not the feature
		// itself: the notation is `port p : ~P` (SysML v2 ConjugatedPortTyping).
		if kind == ast.RelTyping && d.boolOf(el, rdf.SysML+"isConjugated") {
			for i, target := range targets {
				targets[i] = "~" + target
			}
		}
		clause := strings.Join(targets, ", ")
		if kind == ast.RelTyping {
			clause += multPart
		}
		words = append(words, relationshipSyntax[kind], clause)
	}
	return words, nil
}

func (d *decoder) multiplicityText(el *element) string {
	lower, hasLower := d.stringOf(el, rdf.SysML+pLowerBound)
	upper, hasUpper := d.stringOf(el, rdf.SysML+pUpperBound)
	return multiplicityNotation(lower, upper, hasLower, hasUpper)
}

// multiplicityNotation writes the bounds a subject states, or "" for none.
func multiplicityNotation(lower, upper string, hasLower, hasUpper bool) string {
	switch {
	case hasLower && hasUpper:
		return "[" + lower + ".." + upper + "]"
	case hasUpper:
		return "[" + upper + "]"
	case hasLower:
		return "[" + lower + "..*]"
	}
	return ""
}

// boundText renders the bound expression a node states under property, if any.
func (d *decoder) boundText(node rdf.Term, property string, in *element) (string, bool, error) {
	bound, ok := d.graph.Object(node, property)
	if !ok {
		return "", false, nil
	}
	text, err := d.expressionNodeText(bound, in)
	return text, err == nil, err
}

// referenceText renders a single reference property: an element IRI becomes the
// qualified name it encodes, a literal is the name as written.
func (d *decoder) referenceText(el *element, property string) (string, error) {
	list, err := d.referenceList(el, property)
	if err != nil || len(list) == 0 {
		return "", err
	}
	return list[0], nil
}

func (d *decoder) referenceList(el *element, property string) ([]string, error) {
	var out []string
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), property) {
		name, err := d.referenceName(term, el)
		if err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, nil
}

// referenceName renders a reference term for the declaration of el: a literal
// as written (refused unless it is a name), an IRI as the spelling that
// resolves to its element there.
func (d *decoder) referenceName(term rdf.Term, el *element) (string, error) {
	if term.IsLiteral() {
		if term.Datatype == rdf.OpenSysML+dtExpression {
			return term.Value, nil
		}
		if why := notAName(term); why != "" {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the reference %s", term),
				Note: why + ", and a reference the graph does not define is carried as the plain name to write",
			}
		}
		return qualifiedNameText(term.Value), nil
	}
	target, err := d.referencedElement(term.Value)
	if err != nil {
		return "", err
	}
	spelled := d.spelledName(target)
	key := nameKey{member: el.qname, target: target.qname}
	written := spelled
	if d.names != nil {
		var ok bool
		if written, ok = d.names.references[key]; !ok {
			written = relativeName(spelled, el.scope)
		}
	}
	d.wanted.references[key] = wantedReference{
		qualified: spelled,
		written:   written,
		count:     d.wanted.references[key].count + 1,
	}
	return qualifiedNameText(written), nil
}

// memberName renders a chain segment or `first` start, looked up in its operand
// or body: a literal as written, an IRI as its element's own name.
func (d *decoder) memberName(term rdf.Term) (string, *element, error) {
	if term.IsLiteral() {
		return qualifiedNameText(term.Value), nil, nil
	}
	target, name, err := d.namedMember(term)
	if err != nil {
		return "", nil, err
	}
	return nameText(name), target, nil
}

// namedMember is the element an IRI names and the name it is looked up by.
func (d *decoder) namedMember(term rdf.Term) (*element, string, error) {
	target, err := d.referencedElement(term.Value)
	if err != nil {
		return nil, "", err
	}
	name, ok := d.effectiveName(target)
	if !ok {
		return nil, "", &UnsupportedError{
			What: fmt.Sprintf("the element <%s>", target.iri),
			Note: "a feature chain or `first` names it, but it declares no name and takes none from a feature it references or redefines",
		}
	}
	return target, name, nil
}

// spelledName is el's qualified name as notation states it: an unnamed usage's
// segment is the name it answers to, and a membership holding its member (rather
// than standing as a usage, as a subject does) has no segment of its own.
func (d *decoder) spelledName(el *element) string {
	segments := strings.Split(el.qname, "::")
	spelled := make([]string, 0, len(segments))
	for i, cur := len(segments)-1, el; i >= 0; i-- {
		segment := segments[i]
		if cur != nil && strings.HasPrefix(segment, "@") {
			if name, ok := d.effectiveName(cur); ok {
				segment = name
			} else if ontology.IsAncestorOrSelf(cur.metaclass, mOwningMembership) {
				cur = cur.owner
				continue
			}
		}
		spelled = append(spelled, segment)
		if cur != nil {
			cur = cur.owner
		}
	}
	slices.Reverse(spelled)
	return strings.Join(spelled, "::")
}

// effectiveName is the name el answers to: its declared name, else the one its
// naming feature supplies, as answersTo picks it.
func (d *decoder) effectiveName(el *element) (string, bool) {
	seen := map[string]bool{}
	for !seen[el.iri] {
		seen[el.iri] = true
		if name, ok := d.stringOf(el, rdf.SysML+pDeclaredName); ok {
			return name, true
		}
		naming, ok := d.namingFeature(el)
		if !ok {
			return "", false
		}
		if naming.IsLiteral() {
			return literalTargetName(naming)
		}
		next, err := d.referencedElement(naming.Value)
		if err != nil {
			return "", false
		}
		el = next
	}
	return "", false
}

// namingFeature is the feature an unnamed usage takes its name from: the one it
// references, else the first it redefines unless that is a chain (ast.NamingFeature).
func (d *decoder) namingFeature(el *element) (rdf.Term, bool) {
	if _, usage := metaclassUsage[el.metaclass]; !usage {
		return rdf.Term{}, false
	}
	subject := rdf.IRI(el.iri)
	if refs := d.graph.Objects(subject, rdf.SysML+relationshipProperty[ast.RelReferences]); len(refs) > 0 {
		return refs[0], true
	}
	redefs := d.graph.Objects(subject, rdf.SysML+relationshipProperty[ast.RelRedefines])
	if len(redefs) == 0 || ast.IsFeatureChain(literalTarget(redefs[0])) {
		return rdf.Term{}, false
	}
	return redefs[0], true
}

// relativeName strips from qname the longest prefix of scope it is declared
// under, the textual approximation for a reference the resolver does not read.
func relativeName(qname, scope string) string {
	for {
		if scope == "" {
			return qname
		}
		if rest, found := strings.CutPrefix(qname, scope+"::"); found {
			return rest
		}
		cut := strings.LastIndex(scope, "::")
		if cut < 0 {
			scope = ""
			continue
		}
		scope = scope[:cut]
	}
}

// notAName says why a string literal cannot be written as a qualified name —
// one whose every segment fits between unrestricted-name quotes. Its datatype
// and tag are checked by checkLiterals before decoding starts.
func notAName(term rdf.Term) string {
	if term.Value == "" {
		return "it is empty"
	}
	for _, segment := range strings.Split(strings.TrimPrefix(term.Value, "$::"), "::") {
		switch {
		case segment == "":
			return "it has an empty name segment"
		case strings.ContainsAny(segment, "\n\r"):
			return "it has a name segment holding a line break"
		case (len(segment)-len(strings.TrimRight(segment, `\`)))%2 == 1:
			return "it has a name segment ending in a backslash"
		}
	}
	return ""
}

// literalTargetName is the name a usage takes from a naming feature the graph
// keeps as text: the last segment of a name, or of the member a chain reaches.
func literalTargetName(term rdf.Term) (string, bool) {
	if term.Datatype != rdf.OpenSysML+dtExpression {
		return lastSegment(term.Value), true
	}
	name, _ := ast.TargetName(literalTarget(term))
	return name, name != ""
}

// literalTarget parses a relationship target the graph keeps as expression text.
func literalTarget(term rdf.Term) ast.Node {
	if !term.IsLiteral() || term.Datatype != rdf.OpenSysML+dtExpression {
		return nil
	}
	return parser.New(source.New("<naming>", []byte(term.Value))).ParseExpression()
}

func lastSegment(qname string) string {
	if cut := strings.LastIndex(qname, "::"); cut >= 0 {
		return qname[cut+2:]
	}
	return qname
}

func (d *decoder) stringOf(el *element, property string) (string, bool) {
	if text, ok := el.expressions[property]; ok {
		return text, text != ""
	}
	value, ok := d.graph.Lexical(rdf.IRI(el.iri), property)
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

func (d *decoder) boolOf(el *element, property string) bool {
	return d.graph.BoolValue(rdf.IRI(el.iri), property)
}

func intOf(g *rdf.Graph, subject rdf.Term, property string) int {
	value, ok := g.Lexical(subject, property)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}
