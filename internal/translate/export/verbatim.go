package export

import (
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"regexp"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Source text is printed only while it states what the graph states: the
// candidate notation is converted back and each disagreement demotes its text.

// render prints the roots from source text where it agrees with the graph and
// canonically where it does not; every pass demotes more text or is the last.
// A normative id is left implied only while the notation reads as the library
// document that implies it; otherwise the pass is redone with the ids written.
func (d *decoder) render(roots []*element) (string, error) {
	for {
		d.printed = map[*element]bool{}
		d.rebuilt = map[*element]bool{}
		d.prefixed = map[*element]bool{}
		d.bodyAt = map[*element]region{}
		d.usedExpr = map[string]bool{}
		d.written = nil
		d.implied = 0
		if err := d.resolveExpressions(); err != nil {
			return "", err
		}
		var b strings.Builder
		for _, root := range roots {
			if err := d.print(&b, root, 0); err != nil {
				return "", err
			}
		}
		if d.implied > 0 && !d.libraryNotation(b.String(), roots) {
			d.explicit = true
			continue
		}
		if len(d.printed)+len(d.usedExpr) == 0 {
			return b.String(), nil
		}
		before := len(d.demoted) + len(d.demotedExpr)
		d.demoteStale(b.String(), roots)
		if len(d.demoted)+len(d.demotedExpr) == before {
			return b.String(), nil
		}
	}
}

// libraryNotation reports whether the encoder reads notation as the bundled
// library document the graph is a version of, which derives the norm's ids on its own.
func (d *decoder) libraryNotation(notation string, roots []*element) bool {
	name, ok := d.candidateName(roots)
	if !ok || d.library == "" {
		return false
	}
	file := source.New(name, []byte(notation))
	p := parser.New(file)
	root := p.ParseFile()
	return len(p.Diagnostics) == 0 && documentLibrary(file, root) == d.library
}

// verbatim returns the source text an element is printed as, if it carries
// one it has not been demoted from.
func (d *decoder) verbatim(el *element) (string, bool) {
	if d.demoted[el] {
		return "", false
	}
	return d.graph.Lexical(rdf.IRI(el.iri), rdf.OpenSysML+xSourceText)
}

// verbatimWithNotes is verbatim for an element whose text holds the notes the
// graph states; one stating others is demoted, since the graph is authoritative.
func (d *decoder) verbatimWithNotes(el *element) (string, bool) {
	text, ok := d.verbatim(el)
	if !ok {
		return "", false
	}
	if d.statesNotes() && !d.notesAgree(el, text) {
		d.demoted[el] = true
		return "", false
	}
	return text, true
}

// expressionText returns the source text an expression node is written as, if
// it carries one it has not been demoted from; an Expression element's text is
// its lines, which verbatim prints, so its structure is rebuilt here.
func (d *decoder) expressionText(node rdf.Term) (string, bool) {
	if _, isElement := d.byIRI[node.Value]; isElement || d.demotedExpr[node.Value] {
		return "", false
	}
	text, ok := d.graph.Lexical(node, rdf.OpenSysML+xSourceText)
	if ok && text != "" {
		d.usedExpr[node.Value] = true
	}
	return text, ok && text != ""
}

// uuidElementID is the spelling of an element id the uuid form derives: a
// v5 name uuid in plain hex form.
var uuidElementID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// isReferentMembership reports whether term is a plain Membership element —
// the referent relation this mapping mints beside an expression's referent
// edge — whatever namespace its IRI is spelled under.
func isReferentMembership(graph *rdf.Graph, term rdf.Term) bool {
	if !term.IsIRI() {
		return false
	}
	for _, object := range graph.Objects(term, rdf.RDFType) {
		if rdf.LocalName(object.Value) == mMembership {
			return true
		}
	}
	return false
}

// demoteStale demotes whatever verbatim text contradicts the graph; notation
// that does not convert, or whose disagreement cannot be placed, demotes all.
func (d *decoder) demoteStale(notation string, roots []*element) {
	name, ok := d.candidateName(roots)
	if !ok {
		d.demoteAll()
		return
	}
	file := source.New(name, []byte(notation))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		d.demoteAll()
		return
	}
	check, err := d.rebuildCandidate(file, root, roots)
	if err != nil {
		d.demoteAll()
		return
	}
	d.holders = holderIndex{}
	for _, t := range disagreeingTriples(d.graph, check.graph) {
		if d.restatesAnotherDisagreement(t, check) {
			continue
		}
		// A reference to an element only one graph has disagrees about that
		// element's identity, not about the element referring to it.
		var el *element
		var expr string
		if t.Object.IsIRI() && d.graph.HasProperty(t.Object, rdf.RDFType) != check.graph.HasProperty(t.Object, rdf.RDFType) {
			el, expr = d.blame(t.Object.Value, check)
		}
		if el == nil && expr == "" {
			el, expr = d.blame(t.Subject.Value, check)
		}
		if el == nil && expr == "" && t.Object.IsIRI() {
			el, expr = d.blame(t.Object.Value, check)
		}
		// One landing on notation already rebuilt is the graph's own, not its text's.
		switch {
		case el != nil && d.printed[el]:
			d.demoted[el] = true
		case expr != "":
			d.demotedExpr[expr] = true
		case el == nil:
			d.demoteAll()
			return
		}
	}
}

// rebuildCandidate encodes the candidate in the id form the document follows, so both
// sides of the compare share identities; a uuid root tries both forms and keeps the closer.
func (d *decoder) rebuildCandidate(file *source.SourceFile, root *ast.RootNamespace, roots []*element) (*encoder, error) {
	first, second := IDQualifiedName, IDUUID
	for _, root := range roots {
		if uuidElementID.MatchString(root.elementID) {
			first, second = IDUUID, IDQualifiedName
			break
		}
	}
	check, err := encodeDocument(file, root, d.library, first)
	if err != nil {
		return nil, err
	}
	if len(disagreeingTriples(d.graph, check.graph)) > 0 {
		if other, oerr := encodeDocument(file, root, d.library, second); oerr == nil &&
			len(disagreeingTriples(d.graph, other.graph)) < len(disagreeingTriples(d.graph, check.graph)) {
			check = other
		}
	}
	return check, nil
}

// restatesAnotherDisagreement reports whether a disagreeing triple only restates
// a disagreement that counts elsewhere, so it demotes no text of its own.
func (d *decoder) restatesAnotherDisagreement(t rdf.Triple, check *encoder) bool {
	// A referent membership restates the referent edge beside it; the
	// edge's own disagreement is the one that counts.
	if isReferentMembership(d.graph, t.Subject) || isReferentMembership(check.graph, t.Subject) ||
		isReferentMembership(d.graph, t.Object) || isReferentMembership(check.graph, t.Object) {
		return true
	}
	// A library reference only names what a normative id stands for; the
	// element form carries none, so one graph naming it states no more model.
	if LibraryReference(d.graph, t.Subject) || LibraryReference(check.graph, t.Subject) {
		return true
	}
	// A bare comment the stored graph predates is stated by the text alone.
	if d.legacyBareComment(t.Subject, check) || d.legacyBareComment(t.Object, check) {
		return true
	}
	// An implied relationship element's ends restate the owner's collapsed
	// edge, whose disagreement already falls on the target's text.
	el, ok := d.byIRI[t.Subject.Value]
	return ok && el.implied && impliedRelationshipMetaclasses[el.metaclass]
}

// legacyBareComment reports whether term is an anonymous Comment, or the membership owning one,
// that only the re-parsed text states: a bare `/* … */` a graph stored before such comments were elements.
func (d *decoder) legacyBareComment(term rdf.Term, check *encoder) bool {
	if !term.IsIRI() || d.graph.HasProperty(term, rdf.RDFType) {
		return false
	}
	for _, object := range check.graph.Objects(term, rdf.RDFType) {
		switch rdf.LocalName(object.Value) {
		case "Comment":
			return !check.graph.HasProperty(term, rdf.SysML+pDeclaredName) &&
				!check.graph.HasProperty(term, rdf.SysML+pDeclaredShortName)
		case mOwningMembership:
			if member, ok := check.graph.Object(term, rdf.SysML+pMemberElement); ok {
				return d.legacyBareComment(member, check)
			}
		}
	}
	return false
}

// candidateName names the candidate notation for the grammar the roots record
// their text was written in; a root with text recording none was read as a
// buffer with no extension, and the candidate is named so it reads the same
// way. Roots recording different grammars cannot be read as one document; one
// with neither text nor grammar was added to the graph and says nothing, and a
// graph whose roots all say nothing is read in the grammar of the library
// document it is a version of, if it is one.
func (d *decoder) candidateName(roots []*element) (string, bool) {
	language, seen := "", false
	for _, root := range roots {
		recorded, ok := d.stringOf(root, rdf.OpenSysML+xSourceLanguage)
		if _, hasText := d.graph.Lexical(rdf.IRI(root.iri), rdf.OpenSysML+xSourceText); !ok && !hasText {
			continue
		}
		if seen && recorded != language {
			return "", false
		}
		language, seen = recorded, true
	}
	if !seen {
		language = d.libraryLanguage()
	}
	if language == "" {
		return "<converted>", true
	}
	return "<converted>." + language, true
}

// libraryLanguage is the grammar the library document the graph is a version of
// is written in, as languageName records it; "" for a graph that is no library.
func (d *decoder) libraryLanguage() string {
	if d.library == "" {
		return ""
	}
	return languageName(source.KindOf(d.library))
}

// demoteAll demotes every verbatim text and expression printed in this pass.
func (d *decoder) demoteAll() {
	for el := range d.printed {
		d.demoted[el] = true
	}
	for iri := range d.usedExpr {
		d.demotedExpr[iri] = true
	}
}

// disagreeingTriples lists the structural triples only one of the two graphs
// states. Source text differs from canonical notation by design, a member
// index states an order the notation keeps whatever the numbers, and a note
// is layout like the text it is read from, so all of those skip.
func disagreeingTriples(graph, check *rdf.Graph) []rdf.Triple {
	var out []rdf.Triple
	noteNode := func(term rdf.Term) bool {
		return graph.Type(term) == rdf.OpenSysML+mNote || check.Type(term) == rdf.OpenSysML+mNote
	}
	structural := func(t rdf.Triple) bool {
		switch t.Predicate.Value {
		case rdf.OpenSysML + xSourceText, rdf.OpenSysML + xSourceTail, rdf.OpenSysML + xSourceLanguage,
			rdf.OpenSysML + xMemberIndex:
			return false
		}
		return !IsSourceRangeProperty(t.Predicate.Value) && !IsNoteProperty(t.Predicate.Value) && !noteNode(t.Subject)
	}
	for _, t := range graph.Triples() {
		if structural(t) && !check.Has(t) {
			out = append(out, t)
		}
	}
	for _, t := range check.Triples() {
		if structural(t) && !graph.Has(t) && !derivedNameOmitted(graph, t) && !derivedVariabilityOmitted(graph, check, t) &&
			!derivedMembershipOmitted(graph, check, t) && !derivedMultiplicityOmitted(graph, check, t) &&
			!derivedAnnotatingRelationshipOmitted(graph, check, t) && !derivedConnectorFeatureOmitted(graph, check, t) {
			out = append(out, t)
		}
	}
	return out
}

// derivedVariabilityOmitted reports a usage's derived mayTimeVary or isVariable the
// candidate states and the graph omits; a publish without derived properties states no less.
func derivedVariabilityOmitted(graph, check *rdf.Graph, t rdf.Triple) bool {
	local := strings.TrimPrefix(t.Predicate.Value, rdf.SysML)
	if local != "mayTimeVary" && local != "isVariable" {
		return false
	}
	return ontology.IsAncestorOrSelf(rdf.LocalName(check.Type(t.Subject)), "Usage") &&
		!graph.HasProperty(t.Subject, t.Predicate.Value)
}

// derivedConnectorFeatureOmitted reports a connector's derived relatedFeature, sourceFeature
// or targetFeature the candidate states and the graph omits; the ends' subsettings restate it.
func derivedConnectorFeatureOmitted(graph, check *rdf.Graph, t rdf.Triple) bool {
	var key string
	switch {
	case strings.HasPrefix(t.Predicate.Value, rdf.SysML):
		key = strings.TrimPrefix(t.Predicate.Value, rdf.SysML)
	case rdf.IsAnnotationJSON(t.Predicate.Value):
		key = strings.TrimPrefix(t.Predicate.Value, rdf.AnnotationJSON)
	default:
		return false
	}
	switch key {
	case pRelatedFeature, pSourceFeature, pTargetFeature:
	default:
		return false
	}
	return ontology.IsAncestorOrSelf(rdf.LocalName(check.Type(t.Subject)), "Connector") &&
		!graph.HasProperty(t.Subject, t.Predicate.Value)
}

// derivedMembershipOmitted reports a Membership end or name, or an Import's end (sysml: statement or
// its collection annotation), the candidate derives (KerML 8.3.2.4.3, 8.3.2.4.2) and the graph omits.
func derivedMembershipOmitted(graph, check *rdf.Graph, t rdf.Triple) bool {
	var key string
	switch {
	case strings.HasPrefix(t.Predicate.Value, rdf.SysML):
		key = strings.TrimPrefix(t.Predicate.Value, rdf.SysML)
	case rdf.IsAnnotationJSON(t.Predicate.Value):
		key = strings.TrimPrefix(t.Predicate.Value, rdf.AnnotationJSON)
	default:
		return false
	}
	switch key {
	case pSource, pTarget, pRelatedElement, pMemberName, pMemberShortName:
	default:
		return false
	}
	metaclass := rdf.LocalName(check.Type(t.Subject))
	derives := ontology.IsAncestorOrSelf(metaclass, mMembership) ||
		(ontology.IsAncestorOrSelf(metaclass, mImport) && (key == pSource || key == pTarget))
	return derives &&
		!graph.HasProperty(t.Subject, rdf.SysML+key) && !graph.HasProperty(t.Subject, rdf.AnnotationJSON+key)
}

// derivedAnnotatingRelationshipOmitted reports a derived ownedAnnotatingRelationship the candidate
// states for an Annotation the graph owns through ownedRelationship alone (a publish without derived).
func derivedAnnotatingRelationshipOmitted(graph, check *rdf.Graph, t rdf.Triple) bool {
	var key string
	switch {
	case strings.HasPrefix(t.Predicate.Value, rdf.SysML):
		key = strings.TrimPrefix(t.Predicate.Value, rdf.SysML)
	case rdf.IsAnnotationJSON(t.Predicate.Value):
		key = strings.TrimPrefix(t.Predicate.Value, rdf.AnnotationJSON)
	default:
		return false
	}
	if key != pOwnedAnnotatingRelationship || graph.HasProperty(t.Subject, rdf.SysML+key) {
		return false
	}
	for _, owned := range graph.Objects(t.Subject, rdf.SysML+pOwnedRelationship) {
		if rdf.LocalName(check.Type(owned)) == mAnnotation && owned.IsIRI() && !graph.HasProperty(owned, rdf.SysML+pOwnedRelatedElement) {
			return true
		}
	}
	return false
}

// derivedMultiplicityOmitted reports a statement of a usage's implied default Multiplicity
// (SysML v2 7.6.3) the candidate derives and the graph omits — the Multiplicity, its
// OwningMembership and the usage's edges to them — or the ownedMember restating a declared
// range the graph states as the usage's multiplicity; an older graph states no less.
func derivedMultiplicityOmitted(graph, check *rdf.Graph, t rdf.Triple) bool {
	defaultMultiplicity := func(term rdf.Term) bool {
		return term.IsIRI() && !graph.HasProperty(term, rdf.RDFType) &&
			rdf.LocalName(check.Type(term)) == mMultiplicityClass
	}
	owningMembership := func(term rdf.Term) bool {
		if !term.IsIRI() || graph.HasProperty(term, rdf.RDFType) {
			return false
		}
		for _, member := range check.Objects(term, rdf.SysML+pOwnedMemberElement) {
			if defaultMultiplicity(member) {
				return true
			}
		}
		return false
	}
	for _, term := range []rdf.Term{t.Subject, t.Object} {
		if defaultMultiplicity(term) || owningMembership(term) {
			return true
		}
	}
	if t.Predicate.Value == rdf.AnnotationJSON+pOwnedRelationship && t.Object.IsLiteral() {
		return defaultMembershipListOmitted(graph, t)
	}
	return t.Predicate.Value == rdf.SysML+pOwnedMember && t.Object.IsIRI() &&
		rdf.LocalName(check.Type(t.Object)) == mMultiplicityRange &&
		graph.Has(rdf.Triple{Subject: t.Subject, Predicate: rdf.IRI(rdf.SysML + pMultiplicity), Object: t.Object})
}

// defaultMembershipListOmitted reports a json:ownedRelationship list the graph states
// without the default multiplicity's membership the candidate appends (or prepends).
func defaultMembershipListOmitted(graph *rdf.Graph, t rdf.Triple) bool {
	list := t.Object.Value
	entry := `{"@id":"` + rdf.LocalName(rdf.ExpressionIRI(t.Subject, pMultiplicity).Value) + rdf.OwningMembershipSuffix + `"}`
	at := strings.Index(list, entry)
	if at < 0 {
		return false
	}
	switch {
	case strings.HasPrefix(list[at+len(entry):], ","):
		list = list[:at] + list[at+len(entry)+1:]
	case at > 0 && list[at-1] == ',':
		list = list[:at-1] + list[at+len(entry):]
	default:
		list = list[:at] + list[at+len(entry):]
	}
	stated := graph.Objects(t.Subject, t.Predicate.Value)
	if len(stated) == 0 {
		// The list mirrors the sysml:ownedRelationship statements, checked on their own.
		return true
	}
	for _, prior := range stated {
		if prior.Value == list {
			return true
		}
	}
	return false
}

// derivedNameOmitted reports a sysml:name the candidate derives for an element
// the graph states none for: the name is derived (KerML Element::name), so the graph states no less.
func derivedNameOmitted(graph *rdf.Graph, t rdf.Triple) bool {
	return t.Predicate.Value == rdf.SysML+pName && !graph.HasProperty(t.Subject, rdf.SysML+pName)
}

// blame returns the verbatim element a disagreement over iri falls in, or else
// the outermost expression node written from its text. A subject only the
// candidate has is traced to the element written over where it was parsed.
func (d *decoder) blame(iri string, check *encoder) (*element, string) {
	var expr string
	visited := map[string]bool{}
	for iri != "" && !visited[iri] {
		visited[iri] = true
		if el, ok := d.byIRI[iri]; ok {
			if target, ok := d.folded[el]; ok {
				el = target
			}
			return d.nearestVerbatim(el), expr
		}
		if d.usedExpr[iri] {
			expr = iri
		}
		if offset, ok := check.offsets[iri]; ok {
			return d.writerAt(offset), ""
		}
		holder := holderOf(d.graph, &d.holders, iri)
		if holder == "" {
			holder = holderOf(check.graph, &check.holders, iri)
		}
		iri = holder
	}
	return nil, expr
}

// nearestVerbatim returns the element whose notation a disagreement over el
// falls in: the nearest of el and its owners that was written at all. One
// already rebuilt canonically is returned as is, so the blame stops there.
func (d *decoder) nearestVerbatim(el *element) *element {
	for x := el; x != nil; x = x.owner {
		if d.printed[x] || d.rebuilt[x] {
			return x
		}
	}
	return nil
}

// writerAt returns the innermost element written over an offset of the
// candidate notation: the one whose text the notation parsed there came from.
func (d *decoder) writerAt(offset int) *element {
	var best *writing
	for i := range d.written {
		w := &d.written[i]
		if offset < w.where.start || offset >= w.where.end {
			continue
		}
		if best == nil || w.where.end-w.where.start < best.where.end-best.where.start {
			best = w
		}
	}
	if best == nil {
		return nil
	}
	return best.el
}

// holderOf returns the subject a node hangs off: the namespace or relationship
// owning an element, or the subject that points at a membership or expression node.
func holderOf(g *rdf.Graph, holders *holderIndex, iri string) string {
	subject := rdf.IRI(iri)
	if hasAddress(g, subject) {
		for _, property := range []string{pOwningNamespace, pOwner} {
			if owner, ok := g.Object(subject, rdf.SysML+property); ok && owner.IsIRI() {
				return owner.Value
			}
		}
		return ""
	}
	if !g.HasProperty(subject, rdf.RDFType) {
		return ""
	}
	return holders.pointingAt(g, iri)
}

// holderIndex maps each node to the first subject pointing at it, from one walk of
// the graph instead of one per blamed node; a pass over a grown graph starts anew.
type holderIndex struct {
	first map[string]string
}

// pointingAt returns the first subject of g with a statement whose object is iri,
// other than iri itself.
func (h *holderIndex) pointingAt(g *rdf.Graph, iri string) string {
	if h.first == nil {
		h.first = make(map[string]string)
		for _, t := range g.Triples() {
			if !t.Object.IsIRI() || t.Subject.Value == t.Object.Value {
				continue
			}
			if _, seen := h.first[t.Object.Value]; !seen {
				h.first[t.Object.Value] = t.Subject.Value
			}
		}
	}
	return h.first[iri]
}
