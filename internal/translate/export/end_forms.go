package export

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// endNotation writes the ends of an end-binding head — `connect a to b`,
// `bind a = b`, `first a then b`, `of P from a to b` — from the form the graph
// states. Both halves of the mapping build the text here, so what the encoder
// checks it can rebuild is exactly what the decoder rebuilds.
type endNotation struct {
	form string
	// keyword is a verb written ahead of the ends by a head whose own keyword
	// is the noun form (`connection c connect a to b`), or "" when the head
	// keyword introduces the ends itself.
	keyword string
	ends    []string
	payload string
}

func (n endNotation) text() (string, error) {
	bad := func(why string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("an end-binding head written as sysx:%s %q", xEndForm, n.form),
			Note: why,
		}
	}
	var words []string
	if n.keyword != "" {
		words = append(words, n.keyword)
	}
	switch n.form {
	case formTo:
		if len(n.ends) < 2 {
			return "", bad("it states fewer than the two ends this form connects")
		}
		words = append(words, n.ends[0], "to", strings.Join(n.ends[1:], ", "))
	case formNary:
		if len(n.ends) < 2 {
			return "", bad("it states fewer than the two ends this form connects")
		}
		words = append(words, "("+strings.Join(n.ends, ", ")+")")
	case formSatisfy:
		if len(n.ends) != 1 {
			return "", bad("it states other than the one requirement this form names")
		}
		words = append(words, n.ends[0])
	case formEquals, formFirstThen, formFromTo, formFlowTo:
		if len(n.ends) != 2 {
			return "", bad("it states other than the two ends this form binds")
		}
		switch n.form {
		case formEquals:
			words = append(words, n.ends[0], "=", n.ends[1])
		case formFirstThen:
			words = append(words, n.ends[0], "then", n.ends[1])
		default:
			if n.payload != "" {
				words = append(words, "of", n.payload)
			}
			if n.form == formFromTo {
				words = append(words, "from")
			}
			words = append(words, n.ends[0], "to", n.ends[1])
		}
	default:
		return "", bad("this mapping writes no such form; see docs/reference/rdf-mapping.md § End-binding heads")
	}
	return strings.Join(words, " "), nil
}

// endVerbs are the verbs a head writes ahead of its ends when its own keyword
// is a noun (`allocation a allocate x to y`, `connector c from x to y`), by the
// form they introduce.
var endVerbs = map[string][]string{
	formTo:        {"connect", "allocate", "from"},
	formNary:      {"connect", "allocate"},
	formEquals:    {"bind", "of"},
	formFirstThen: {"first"},
}

// endForm states the form an end-binding head writes its ends in, so a graph
// without the head's source text can be written back from its structure. The
// form is only claimed when rebuilding it reproduces the head as written: a
// head this mapping cannot rebuild exactly stays readable as text alone.
func (e *encoder) endForm(subject rdf.Term, n *ast.Usage) {
	if n.Kind == ast.UsageSatisfy {
		e.satisfyForm(subject, n)
		return
	}
	form, ends, payload := e.endShape(n)
	if form == "" {
		return
	}
	keyword, from := e.endVerb(n, form)
	written, ok := e.headTail(n, from)
	if !ok {
		return
	}
	rebuilt, err := (endNotation{form: form, keyword: keyword, ends: ends, payload: payload}).text()
	if err != nil || !sameSpelling(rebuilt, written) {
		return
	}
	e.graph.Add(subject, e.sysx(xEndForm), rdf.String(form))
	if keyword != "" {
		e.graph.Add(subject, e.sysx(xEndVerb), rdf.String(keyword))
	}
	if n.Keyword != "" && n.Keyword != usageKeyword(n.Kind) {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
	}
}

// satisfyForm states that a satisfy head writes the requirement it subsets
// bare, right after its keyword (`satisfy R by v`), which is what tells that
// clause from a `subsets` one written out. The keyword goes with it, since
// `verify` is the same kind spelled differently.
func (e *encoder) satisfyForm(subject rdf.Term, n *ast.Usage) {
	requirement := relationshipTarget(n, ast.RelSubsets)
	if requirement == nil || n.Ident.Name != "" || n.Value != nil {
		return
	}
	for _, rel := range n.Relationships {
		if rel != nil && rel.Kind != ast.RelSubsets && rel.Kind != ast.RelSubject {
			return
		}
	}
	head, ok := e.headTail(n, n.Span().Offset)
	if !ok || !sameSpelling(head, n.Keyword+" "+e.text(requirement)+" "+e.subjectClause(n)) {
		return
	}
	e.graph.Add(subject, e.sysx(xEndForm), rdf.String(formSatisfy))
	if n.Keyword != "" && n.Keyword != usageKeyword(n.Kind) {
		e.graph.Add(subject, e.sysx(xDeclaredKeyword), rdf.String(n.Keyword))
	}
}

// subjectClause is the `by` clause of a satisfy head, or "" where it names no
// subject.
func (e *encoder) subjectClause(n *ast.Usage) string {
	subject := relationshipTarget(n, ast.RelSubject)
	if subject == nil {
		return ""
	}
	return "by " + e.text(subject)
}

// endShape reads the form a head writes its ends in, with the end texts the
// graph carries beside it, or "" for a head whose ends the graph cannot state:
// an end that redefines, an inline payload declaration, or a transition's
// trigger, guard and effect.
func (e *encoder) endShape(n *ast.Usage) (form string, ends []string, payload string) {
	switch {
	case len(n.ConnectorEnds) > 0:
		for _, end := range n.ConnectorEnds {
			text, ok := e.connectorEndText(end)
			if !ok {
				return "", nil, ""
			}
			ends = append(ends, text)
		}
		switch {
		case n.Kind == ast.UsageSuccession:
			if len(ends) != 2 {
				return "", nil, ""
			}
			return formFirstThen, ends, ""
		case n.Kind == ast.UsageBinding:
			if len(ends) != 2 {
				return "", nil, ""
			}
			return formEquals, ends, ""
		case e.wroteBefore(n, n.ConnectorEnds[0].Target, "("):
			return formNary, ends, ""
		case len(ends) == 2:
			return formTo, ends, ""
		}
		return "", nil, ""
	case n.FlowEnds != nil:
		flow := n.FlowEnds
		if flow.From == nil || flow.To == nil || flow.PayloadDecl != nil || flow.PayloadMultiplicity != nil {
			return "", nil, ""
		}
		ends = []string{e.text(flow.From), e.text(flow.To)}
		if e.wroteBefore(n, flow.From, "from") {
			return formFromTo, ends, e.text(flow.Payload)
		}
		return formFlowTo, ends, e.text(flow.Payload)
	}
	return "", nil, ""
}

// endText is one end as its notation writes it: the multiplicity ahead of the
// feature it names, when the end states one.
func (e *encoder) endText(mult *ast.Multiplicity, target ast.Node) string {
	if mult == nil {
		return e.text(target)
	}
	return e.text(mult) + " " + e.text(target)
}

// connectorEndText is one connector end as the graph can state it: `[1] a.p`
// or `[1] bead ::> t.bead`; an end saying more than that is not stated.
func (e *encoder) connectorEndText(end *ast.ConnectorEnd) (string, bool) {
	if end == nil || end.Target == nil {
		return "", false
	}
	if _, named := end.DeclaredName(); !named {
		if end.ReferencedTarget() != nil {
			return "", false
		}
		return e.endText(end.Multiplicity, end.Target), true
	}
	reference := endReference(end)
	if reference == nil {
		return "", false
	}
	return e.endText(end.Multiplicity, end.Target) + " " + e.referencesKeyword(end) + " " + e.text(reference), true
}

// endReference is the one reference subsetting a named end states, else nil.
func endReference(end *ast.ConnectorEnd) *ast.Relationship {
	var reference *ast.Relationship
	for _, rel := range end.Relationships {
		if rel == nil || rel.Kind != ast.RelReferences || reference != nil {
			return nil
		}
		reference = rel
	}
	return reference
}

// referencesKeyword is the ReferencesKeyword written between a connector end's
// name and its target, `::>` or `references` (KerML.xtext:856).
func (e *encoder) referencesKeyword(end *ast.ConnectorEnd) string {
	reference := endReference(end)
	if reference == nil || end.Target == nil {
		return referencesSymbol
	}
	between := source.Span{Offset: end.Target.Span().End(), Len: reference.Span().Offset - end.Target.Span().End()}
	if between.Len > 0 {
		if written := words(e.src.slice(between)); len(written) == 1 && written[0] == referencesWord {
			return referencesWord
		}
	}
	return referencesSymbol
}

// endVerb returns the verb written ahead of the ends and the offset the ends
// notation starts at, which is that verb's if there is one.
func (e *encoder) endVerb(n *ast.Usage, form string) (string, int) {
	first := e.firstEnd(n)
	if first == nil {
		return "", -1
	}
	start, at := n.Span().Offset, first.Span().Offset
	if form == formEquals {
		// The `of` of `binding b of a = c` names the bound feature the way
		// `bind` does, and both precede it.
		start = n.Span().Offset
	}
	if at <= start {
		return "", at
	}
	head := source.Span{Offset: start, Len: at - start}
	for _, verb := range endVerbs[form] {
		// A head whose own keyword is the verb (`connect a to b`) writes it
		// once; the keyword the graph already carries is that verb.
		if verb == n.Keyword {
			continue
		}
		if index := e.src.lastToken(head, verb); index >= 0 {
			return verb, index
		}
	}
	if form == formNary {
		if index := e.src.lastToken(head, "("); index >= 0 {
			return "", index
		}
	}
	if form == formFromTo {
		if index := e.src.lastToken(head, "of"); index >= 0 {
			return "", index
		}
		if index := e.src.lastToken(head, "from"); index >= 0 {
			return "", index
		}
	}
	if form == formFlowTo && n.FlowEnds != nil && n.FlowEnds.Payload != nil {
		if index := e.src.lastToken(head, "of"); index >= 0 {
			return "", index
		}
	}
	return "", at
}

// firstEnd returns the node the ends notation begins with: the first end's
// multiplicity when it states one, else the feature it names.
func (e *encoder) firstEnd(n *ast.Usage) ast.Node {
	switch {
	case len(n.ConnectorEnds) > 0 && n.ConnectorEnds[0] != nil:
		if n.ConnectorEnds[0].Multiplicity != nil {
			return n.ConnectorEnds[0].Multiplicity
		}
		return n.ConnectorEnds[0].Target
	case n.FlowEnds != nil:
		if n.FlowEnds.Payload != nil {
			return n.FlowEnds.Payload
		}
		return n.FlowEnds.From
	}
	return nil
}

// headTail returns the declaration text from an offset to the end of the head,
// which is what the ends notation has to reproduce.
func (e *encoder) headTail(n *ast.Usage, from int) (string, bool) {
	end := e.headEnd(n)
	if from < n.Span().Offset || from >= end {
		return "", false
	}
	text := strings.TrimSpace(e.src.code(source.Span{Offset: from, Len: end - from}))
	if strings.ContainsAny(text, "{}") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimSuffix(text, ";")), true
}

// wroteBefore reports whether word is a token of the head ahead of an end, which
// is what tells the parenthesized and `from` forms from the ones without them.
func (e *encoder) wroteBefore(n *ast.Usage, end ast.Node, word string) bool {
	start, at := n.Span().Offset, end.Span().Offset
	if at <= start {
		return false
	}
	return e.src.lastToken(source.Span{Offset: start, Len: at - start}, word) >= 0
}

// relationshipTarget returns the target of the first relationship of a kind.
func relationshipTarget(n *ast.Usage, kind ast.RelationshipKind) ast.Node {
	for _, rel := range n.Relationships {
		if rel != nil && rel.Kind == kind && rel.Target != nil {
			return rel.Target
		}
	}
	return nil
}

// endWords rebuilds the ends of an end-binding head from the graph: the form it
// states, the verb it writes them after, and the features it relates. declared
// reports that a declaration part precedes the ends, which the verb then separates.
func (d *decoder) endWords(el *element, form string, declared bool) (string, error) {
	ends, payload, err := d.relatedEnds(el)
	if err != nil {
		return "", err
	}
	verb, _ := d.stringOf(el, rdf.OpenSysML+xEndVerb)
	if verb == "" && declared {
		// A declaration is followed by the verb (KerML.xtext BindingConnectorDeclaration,
		// SysML.xtext BindingConnectorAsUsage); `binding [1] a = b` gives `[1]` to the end.
		switch {
		case form == formEquals && d.kerml(el):
			verb = "of"
		case form == formEquals:
			verb = "bind"
		case form == formFirstThen:
			verb = "first"
		}
	}
	if len(ends) == 0 {
		return "", d.missing(el, "sysx:"+xRelatedFeature, "a head that states sysx:"+xEndForm+" relates the ends it binds")
	}
	return endNotation{form: form, keyword: verb, ends: ends, payload: payload}.text()
}

// relatedEnds reads the ends of a head in the order they are written, each
// behind the multiplicity it states, with the payload of a flow kept apart: it
// is written ahead of them, after `of`.
func (d *decoder) relatedEnds(el *element) (ends []string, payload string, err error) {
	standard, hasStandard, err := d.standardEnds(el)
	if err != nil {
		return nil, "", err
	}
	legacy, legacyPayload, err := d.legacyEnds(el)
	if err != nil {
		return nil, "", err
	}
	if hasStandard {
		if len(legacy) > 0 && (!slices.Equal(standard, legacy) || legacyPayload != "") {
			return nil, "", &UnsupportedError{
				What: fmt.Sprintf("the connector ends of <%s>", el.iri),
				Note: "its standard sysml:connectorEnd and legacy sysx:relatedFeature shapes disagree",
			}
		}
		payload, _ = d.stringOf(el, rdf.OpenSysML+xPayload)
		return standard, payload, nil
	}
	return legacy, legacyPayload, nil
}

func (d *decoder) standardEnds(el *element) ([]string, bool, error) {
	terms := d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pConnectorEnd)
	if len(terms) == 0 {
		for _, membership := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pOwnedFeatureMembership) {
			if d.metaclass(membership) != mEndFeatureMembership {
				continue
			}
			if member, ok := d.graph.Object(membership, rdf.SysML+pMemberElement); ok {
				terms = append(terms, member)
			}
		}
	}
	if len(terms) == 0 {
		for _, feature := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pOwnedFeature) {
			for _, membership := range d.graph.Objects(feature, rdf.SysML+pOwningMembership) {
				if d.metaclass(membership) == mEndFeatureMembership {
					terms = append(terms, feature)
					break
				}
			}
		}
	}
	if len(terms) == 0 {
		return nil, false, nil
	}
	ends := make([]string, 0, len(terms))
	for _, term := range terms {
		text, err := d.standardEndText(term, el)
		if err != nil {
			return nil, true, err
		}
		ends = append(ends, text)
	}
	return ends, true, nil
}

func (d *decoder) standardEndText(end rdf.Term, in *element) (string, error) {
	target, ok := d.graph.Object(end, rdf.SysML+pReferences)
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
			Note: "it has no sysml:references target",
		}
	}
	var text string
	if d.graph.HasProperty(target, rdf.SysML+pChainingFeature) {
		parts, err := d.standardChainText(target, in)
		if err != nil {
			return "", err
		}
		text = strings.Join(parts, ".")
	} else if target.IsLiteral() {
		text = target.Value
	} else {
		var err error
		text, err = d.endReferenceText(target, in)
		if err != nil {
			return "", err
		}
	}
	name, err := d.standardEndName(end, in)
	if err != nil {
		return "", err
	}
	if name != "" {
		text = name + " " + text
	}
	mult, err := d.endMultiplicity(end, in)
	if err != nil {
		return "", err
	}
	if mult != "" {
		text = mult + " " + text
	}
	return text, nil
}

func (d *decoder) standardChainText(chain rdf.Term, in *element) ([]string, error) {
	segments := d.graph.Objects(chain, rdf.SysML+pChainingFeature)
	parts := make([]string, 0, len(segments))
	operand := ""
	for _, segment := range segments {
		if segment.IsLiteral() {
			parts = append(parts, qualifiedNameText(segment.Value))
			operand = ""
			continue
		}
		target, name, err := d.namedMember(segment)
		if err != nil {
			return nil, err
		}
		spelling := nameText(name)
		if d.names != nil {
			key := segmentKey{member: in.qname, operand: operand, name: name, target: target.qname}
			if chosen, ok := d.names.segments[key]; ok {
				spelling = qualifiedNameText(chosen)
			}
		}
		parts = append(parts, spelling)
		operand = target.qname
	}
	return parts, nil
}

func (d *decoder) endReferenceText(target rdf.Term, in *element) (string, error) {
	if target.IsLiteral() {
		return d.referenceName(target, in)
	}
	return d.referenceName(target, in)
}

func (d *decoder) standardEndName(end rdf.Term, in *element) (string, error) {
	names := d.graph.Objects(end, rdf.SysML+pDeclaredName)
	if len(names) == 0 {
		names = d.graph.Objects(end, rdf.SysML+pName)
	}
	if len(names) == 0 {
		return "", nil
	}
	if len(names) > 1 {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
			Note: "it declares more than one end name",
		}
	}
	name := names[0].Value
	keyword := referencesSymbol
	if spelled, ok := d.graph.Lexical(end, rdf.OpenSysML+xEndReferencesKeyword); ok {
		if spelled != referencesSymbol && spelled != referencesWord {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
				Note: fmt.Sprintf("it spells its ReferencesKeyword as %q", spelled),
			}
		}
		keyword = spelled
	}
	return nameText(name) + " " + keyword, nil
}

func (d *decoder) legacyEnds(el *element) (ends []string, payload string, err error) {
	type end struct {
		index int
		text  string
	}
	var ordered []end
	for _, term := range d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xRelatedFeature) {
		named, err := d.endNameText(term, el)
		if err != nil {
			return nil, "", err
		}
		text, err := d.expressionNodeText(term, el)
		if err != nil {
			return nil, "", err
		}
		if role, ok := d.graph.Lexical(term, rdf.OpenSysML+xEndRole); ok && role == "payload" {
			payload = text
			continue
		}
		if named != "" {
			text = named + " " + text
		}
		mult, err := d.endMultiplicity(term, el)
		if err != nil {
			return nil, "", err
		}
		if mult != "" {
			text = mult + " " + text
		}
		ordered = append(ordered, end{index: intOf(d.graph, term, rdf.OpenSysML+xEndIndex), text: text})
	}
	slices.SortStableFunc(ordered, func(a, b end) int { return a.index - b.index })
	for _, end := range ordered {
		ends = append(ends, end.text)
	}
	return ends, payload, nil
}

// endNameText writes a connector end's declared name and ReferencesKeyword
// (`e1 ::>`), or "" for a bare end; a named end relating no feature is refused.
func (d *decoder) endNameText(end rdf.Term, in *element) (string, error) {
	name, ok := d.graph.Lexical(end, rdf.OpenSysML+xEndName)
	if !ok {
		return "", nil
	}
	refuse := func(note string) error {
		return &UnsupportedError{
			What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
			Note: note,
		}
	}
	if d.metaclass(end) == "" && !d.graph.HasProperty(end, rdf.OpenSysML+xSourceText) {
		return "", refuse(fmt.Sprintf("it declares a name (sysx:%s) but relates no feature, and a named connector end reference-subsets the feature it attaches to", xEndName))
	}
	keyword := referencesSymbol
	if spelled, ok := d.graph.Lexical(end, rdf.OpenSysML+xEndReferencesKeyword); ok {
		if spelled != referencesSymbol && spelled != referencesWord {
			return "", refuse(fmt.Sprintf("it spells its ReferencesKeyword as %q (sysx:%s), and the notation has only `%s` and `%s`", spelled, xEndReferencesKeyword, referencesSymbol, referencesWord))
		}
		keyword = spelled
	}
	return nameText(name) + " " + keyword, nil
}

// endMultiplicity writes the bounds an end node states (`connect [1] a to b`),
// or "" for an end written bare.
func (d *decoder) endMultiplicity(end rdf.Term, in *element) (string, error) {
	lower, hasLower, err := d.boundText(end, rdf.SysML+pLowerBound, in)
	if err != nil {
		return "", err
	}
	upper, hasUpper, err := d.boundText(end, rdf.SysML+pUpperBound, in)
	if err != nil {
		return "", err
	}
	return multiplicityNotation(lower, upper, hasLower, hasUpper), nil
}

// statesEnds reports whether an element relates ends of its own, the shape that
// needs a form to be written back.
func (d *decoder) statesEnds(el *element) bool {
	if len(d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pConnectorEnd)) > 0 {
		return true
	}
	for _, membership := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pOwnedFeatureMembership) {
		if d.metaclass(membership) == mEndFeatureMembership {
			return true
		}
	}
	return len(d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xRelatedFeature)) > 0
}

func (d *decoder) inferredEndForm(el *element) string {
	switch el.metaclass {
	case "BindingConnectorAsUsage":
		return formEquals
	case "SuccessionAsUsage":
		return formFirstThen
	case "FlowUsage":
		return formFromTo
	case "ConnectionUsage", "InterfaceUsage", "AllocationUsage", "ConnectorAsUsage":
		if terms := d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pConnectorEnd); len(terms) > 2 {
			return formNary
		}
		return formTo
	}
	return ""
}
