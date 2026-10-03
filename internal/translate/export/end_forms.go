package export

import (
	"fmt"
	"slices"
	"strconv"
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
		case form == formTo && el.metaclass == usageMetaclass[ast.UsageFlow]:
			verb = "from"
		case (form == formTo || form == formNary) && el.metaclass == usageMetaclass[ast.UsageAllocation]:
			verb = "allocate"
		case form == formTo || form == formNary:
			verb = "connect"
		}
	}
	if len(ends) == 0 {
		return "", d.missing(el, "sysx:"+xRelatedFeature, "a head that states sysx:"+xEndForm+" relates the ends it binds")
	}
	return endNotation{form: form, keyword: verb, ends: ends, payload: payload}.text()
}

// payloadText writes the payload a flow's head states after `of`, from the
// PayloadFeature the flow owns (SysML-textual-bnf FlowPayloadFeatureMember):
// `p : T[1] = v` for a declared one, `T[1]` for one that states only its type.
// A graph written before the payload was a feature states it as the
// expression sysx:payload instead, which is read as written; one stating both
// is refused rather than one chosen.
func (d *decoder) payloadText(el *element) (string, error) {
	legacy, hasLegacy := d.stringOf(el, rdf.OpenSysML+xPayload)
	payload := d.flowPayload(el)
	if payload == nil {
		for _, child := range el.children {
			if child.metaclass == mPayloadFeature {
				return "", &UnsupportedError{
					What: fmt.Sprintf("the payload features of <%s>", el.iri),
					Note: "a flow writes one payload after `of`, and only a flow has one",
				}
			}
		}
		return legacy, nil
	}
	if hasLegacy {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the payload of <%s>", el.iri),
			Note: "it states both a PayloadFeature and the earlier sysx:payload expression, and the head writes one payload",
		}
	}
	// The payload is written inside the flow's head, where it has no body.
	if len(d.bodyChildren(payload)) > 0 || d.boolOf(payload, rdf.OpenSysML+xHasBody) {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the payload <%s>", payload.iri),
			Note: "it owns members or states a body, and a payload written after `of` has no place for a body",
		}
	}
	mult := d.multiplicityText(payload)
	// `ordered` and `nonunique` follow the multiplicity of a declared payload
	// (PayloadFeatureSpecializationPart's MultiplicityPart); `of T[1]` has none.
	flags := ""
	if d.boolOf(payload, rdf.SysML+"isOrdered") {
		flags += " ordered"
	}
	if d.nonunique(payload) {
		flags += " nonunique"
	}
	words := d.identWords(payload)
	typed, err := d.referenceList(payload, rdf.SysML+relationshipProperty[ast.RelTyping])
	if err != nil {
		return "", err
	}
	if len(words) == 0 {
		rest, err := d.relationshipWords(payload, "", ast.RelTyping)
		if err != nil {
			return "", err
		}
		if _, valued := d.stringOf(payload, rdf.SysML+pValue); len(typed) != 1 || len(rest) > 0 || valued || flags != "" {
			return "", &UnsupportedError{
				What: fmt.Sprintf("the payload <%s>", payload.iri),
				Note: "a payload with no name is written by its one type alone (SysML-textual-bnf PayloadFeature), so one that states anything else has no notation",
			}
		}
		return typed[0] + mult, nil
	}
	// A named payload is written `p : T`, the one declared form the notation
	// reads back as a declaration: `of p` alone, or `of p[1]`, reads as a
	// payload typed by p (SysML-textual-bnf PayloadFeature).
	if rest, err := d.relationshipWords(payload, "", ast.RelTyping); err != nil {
		return "", err
	} else if len(typed) != 1 || len(rest) > 0 {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the payload <%s>", payload.iri),
			Note: "a named payload is written `of <name> : <type>`, so one stating no single typing, or another specialization, has no notation that reads back as it",
		}
	}
	mult += flags
	relationships, err := d.relationshipWords(payload, mult)
	if err != nil {
		return "", err
	}
	words = append(words, relationships...)
	if value, ok := d.stringOf(payload, rdf.SysML+pValue); ok {
		words = append(words, d.valueOperator(payload), value)
	}
	return strings.Join(words, " "), nil
}

// flowPayload returns the PayloadFeature a flow's head writes after `of`: the
// one a flow owns, or nil when el is no flow or owns none or several.
func (d *decoder) flowPayload(el *element) *element {
	if !flowMetaclasses[el.metaclass] {
		return nil
	}
	var payload *element
	for _, child := range el.children {
		if child.metaclass != mPayloadFeature {
			continue
		}
		if payload != nil {
			return nil
		}
		payload = child
	}
	// A flow owns its payload through a FeatureMembership
	// (FlowPayloadFeatureMember); one owned otherwise has no `of` notation.
	if payload != nil {
		if m, ok := d.owningMembership[payload.iri]; !ok || d.metaclass(rdf.IRI(m.iri)) != mFeatureMembership {
			return nil
		}
	}
	return payload
}

// flowMetaclasses are the usages whose head takes an `of` clause
// (SysML-textual-bnf FlowDeclaration, MessageDeclaration).
var flowMetaclasses = map[string]bool{"FlowUsage": true, "SuccessionFlowUsage": true}

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
		payload, err = d.payloadText(el)
		return standard, payload, err
	}
	// Legacy ends may state the payload as an end role, and the flow may state
	// it again as sysx:payload or a PayloadFeature: each shape is read, and two
	// that disagree are refused rather than one dropped.
	stated, err := d.payloadText(el)
	if err != nil {
		return nil, "", err
	}
	switch {
	case legacyPayload == "":
		legacyPayload = stated
	case stated != "" && stated != legacyPayload:
		return nil, "", &UnsupportedError{
			What: fmt.Sprintf("the payload of <%s>", el.iri),
			Note: fmt.Sprintf("its legacy payload end says %q and the flow says %q, and the head writes one payload", legacyPayload, stated),
		}
	}
	return legacy, legacyPayload, nil
}

// standardEnds reads connectorEnd features in graph order, with ownership fallbacks.
func (d *decoder) standardEnds(el *element) ([]string, bool, error) {
	terms, err := d.standardEndFeatures(el)
	if err != nil {
		return nil, false, err
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

// standardEndText renders one owned connector end from its structural target.
func (d *decoder) standardEndText(end rdf.Term, in *element) (string, error) {
	if d.metaclass(end) == mFlowEnd {
		return d.flowEndText(end, in)
	}
	target, ok, err := d.standardEndTarget(end, in)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
			Note: "it has no ReferenceSubsetting or sysml:references target",
		}
	}
	var text string
	isChain, err := d.chainFeatureTerm(target)
	if err != nil {
		return "", err
	}
	if isChain {
		parts, err := d.standardChainText(target, in)
		if err != nil {
			return "", err
		}
		text = strings.Join(parts, ".")
	} else if target.IsLiteral() {
		text = target.Value
	} else {
		var err error
		text, err = d.referenceName(target, in)
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

// standardEndFeatures discovers connector ends through each standard ownership
// representation, including the ownership-only shape accepted by standardEnds.
func (d *decoder) standardEndFeatures(el *element) ([]rdf.Term, error) {
	var terms []rdf.Term
	for _, candidates := range []func(*element) ([]rdf.Term, error){
		d.connectorEndTerms, d.endMembershipMembers, d.messageEnds, d.endMembershipFeatures, d.headEndChildren,
	} {
		ends, err := candidates(el)
		if err != nil {
			return nil, err
		}
		for _, end := range ends {
			terms = d.appendEnd(el, terms, end)
		}
		if len(terms) > 0 {
			break
		}
	}
	return terms, nil
}

// appendEnd appends a connector end once; an end the connector declares as a
// member is written in its body as `end`, not in its head: the abstract
// syntax is the same.
func (d *decoder) appendEnd(el *element, terms []rdf.Term, term rdf.Term) []rdf.Term {
	flowEnd := d.byIRI[term.Value]
	headFlowEnd := flowEnd != nil && flowEnd.metaclass == mFlowEnd && d.headEnd(flowEnd, el)
	if (d.declaredChild(el, term) && !headFlowEnd) || slices.Contains(terms, term) {
		return terms
	}
	return append(terms, term)
}

// connectorEndTerms is the ends the connector states as sysml:connectorEnd.
func (d *decoder) connectorEndTerms(el *element) ([]rdf.Term, error) {
	return d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pConnectorEnd), nil
}

// endMembershipMembers is the members of the connector's EndFeatureMemberships.
func (d *decoder) endMembershipMembers(el *element) ([]rdf.Term, error) {
	var ends []rdf.Term
	for _, membership := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pOwnedFeatureMembership) {
		if d.metaclass(membership) != mEndFeatureMembership {
			continue
		}
		if member, ok := d.graph.Object(membership, rdf.SysML+pMemberElement); ok {
			ends = append(ends, member)
		}
	}
	return ends, nil
}

// endMembershipFeatures is the connector's owned features that name an
// EndFeatureMembership as their owning membership.
func (d *decoder) endMembershipFeatures(el *element) ([]rdf.Term, error) {
	var ends []rdf.Term
	for _, feature := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pOwnedFeature) {
		for _, membership := range d.graph.Objects(feature, rdf.SysML+pOwningMembership) {
			if d.metaclass(membership) == mEndFeatureMembership {
				ends = append(ends, feature)
				break
			}
		}
	}
	return ends, nil
}

// headEndChildren is the connector's children written as head ends.
func (d *decoder) headEndChildren(el *element) ([]rdf.Term, error) {
	var ends []rdf.Term
	for _, child := range el.children {
		if d.headEnd(child, el) {
			ends = append(ends, rdf.IRI(child.iri))
		}
	}
	return ends, nil
}

// messageEnds returns the event ends a `message` owns through
// ParameterMembership (SysML.xtext MessageEventMember), in written order; a
// `flow` owns connector ends instead and returns none. Their metaclass —
// EventOccurrenceUsage — is what the `message` keyword states where the head
// recorded none. The ends come from two passes — memberships the flow links,
// then memberships naming the flow back — so the order the graph states them
// orders them only when one pass found them all; when both did, sysx:memberIndex
// or the flow's own source and target must.
func (d *decoder) messageEnds(el *element) ([]rdf.Term, error) {
	if el.metaclass != usageMetaclass[ast.UsageFlow] {
		return nil, nil
	}
	ends, memberships, linked, standalone := d.messageEndMemberships(el)
	indexes, byIndex := messageEndIndexes(d.graph, ends, memberships)
	sourceFirst, err := d.messageEndsBySource(el, ends)
	if err != nil {
		return nil, err
	}
	bySource := sourceFirst != nil
	if byIndex {
		order := slices.Clone(ends)
		slices.SortStableFunc(order, func(a, b rdf.Term) int {
			return indexes[a.Value] - indexes[b.Value]
		})
		if bySource && !slices.Equal(order, sourceFirst) {
			return nil, &UnsupportedError{
				What: fmt.Sprintf("the message ends of <%s>", el.iri),
				Note: "its sysx:memberIndex order and its sysml:sourceFeature/sysml:targetFeature disagree",
			}
		}
		return order, nil
	}
	if bySource {
		return sourceFirst, nil
	}
	if !linked || !standalone {
		return ends, nil
	}
	return nil, &UnsupportedError{
		What: fmt.Sprintf("the message ends of <%s>", el.iri),
		Note: "its ends are stated partly by the flow's own memberships and partly by memberships naming the flow alone, and neither sysx:memberIndex nor sysml:sourceFeature/sysml:targetFeature orders them",
	}
}

// messageEndMemberships is the flow's event ends and the ParameterMemberships
// owning them, in the order the graph states them, and whether any came from
// a membership the flow links (linked) or one naming the flow alone (standalone).
func (d *decoder) messageEndMemberships(el *element) (ends, memberships []rdf.Term, linked, standalone bool) {
	seen := map[string]bool{}
	consider := func(membership rdf.Term, isLinked bool) {
		if seen[membership.Value] || d.metaclass(membership) != mParameterMembership {
			return
		}
		seen[membership.Value] = true
		member := firstIRI(d.graph, membership, pMemberElement, pOwnedMemberElement, pOwnedMemberParameter, pOwnedRelatedElement)
		if member.Value == "" || d.metaclass(member) != mEventOccurrenceUsage {
			return
		}
		ends = append(ends, member)
		memberships = append(memberships, membership)
		if isLinked {
			linked = true
		} else {
			standalone = true
		}
	}
	for _, property := range []string{pOwnedMembership, pOwnedRelationship} {
		for _, membership := range d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+property) {
			consider(membership, true)
		}
	}
	// A membership may state the flow it belongs to from its own side alone
	// (sysml:membershipOwningNamespace, sysml:owningRelatedElement), the way
	// readMembership accepts it; those follow in the order the graph states them.
	if d.parameterOwned == nil {
		d.parameterOwned = parameterOwnerIndex(d.graph, d.metaclass)
	}
	for _, membership := range d.parameterOwned[el.iri] {
		consider(membership, false)
	}
	return ends, memberships, linked, standalone
}

// messageEndIndexes is the position each membership, or else its member,
// states for its end, and whether those positions order every end. A value
// present but not an integer is no index at all, and ends tied at one index
// have no order.
func messageEndIndexes(graph *rdf.Graph, ends, memberships []rdf.Term) (map[string]int, bool) {
	indexOf := func(term rdf.Term) (int, bool) {
		value, ok := graph.Lexical(term, rdf.OpenSysML+xMemberIndex)
		if !ok {
			return 0, false
		}
		n, err := strconv.Atoi(value)
		return n, err == nil
	}
	seenIndex := map[int]bool{}
	indexes := map[string]int{}
	for i := range ends {
		index, ok := indexOf(memberships[i])
		if !ok {
			index, ok = indexOf(ends[i])
		}
		if !ok || seenIndex[index] {
			return nil, false
		}
		seenIndex[index] = true
		indexes[ends[i].Value] = index
	}
	return indexes, len(ends) > 0
}

// messageEndsBySource orders two ends by the flow's own sysml:sourceFeature
// and sysml:targetFeature, each end's referenced feature matching one; nil
// when they do not order them.
func (d *decoder) messageEndsBySource(el *element, ends []rdf.Term) ([]rdf.Term, error) {
	if len(ends) != 2 {
		return nil, nil
	}
	sources := d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pSourceFeature)
	targets := d.graph.Objects(rdf.IRI(el.iri), rdf.SysML+pTargetFeature)
	if len(sources) != 1 || len(targets) != 1 || sources[0] == targets[0] {
		return nil, nil
	}
	var first rdf.Term
	matched := true
	for _, end := range ends {
		target, ok, err := d.standardEndTarget(end, el)
		if err != nil {
			return nil, err
		}
		switch {
		case !ok:
			matched = false
		case target == sources[0] && first.Value == "":
			first = end
		case target == targets[0]:
		default:
			matched = false
		}
	}
	if !matched || first.Value == "" {
		return nil, nil
	}
	sourceFirst := []rdf.Term{first}
	for _, end := range ends {
		if end != first {
			sourceFirst = append(sourceFirst, end)
		}
	}
	return sourceFirst, nil
}

// parameterOwnerIndex indexes the ParameterMembership elements by the
// namespace each states as its own (sysml:membershipOwningNamespace, else
// sysml:owningRelatedElement), in the order the graph states them.
func parameterOwnerIndex(graph *rdf.Graph, meta func(rdf.Term) string) map[string][]rdf.Term {
	index := map[string][]rdf.Term{}
	for _, subject := range graph.Subjects() {
		if meta(subject) != mParameterMembership {
			continue
		}
		if owner := firstIRI(graph, subject, pMembershipOwningNamespace, pOwningRelatedElement); owner.Value != "" {
			index[owner.Value] = append(index[owner.Value], subject)
		}
	}
	return index
}

// declaredChild reports whether term is an element the graph declares under el
// rather than a node el owns by structure alone.
func (d *decoder) declaredChild(el *element, term rdf.Term) bool {
	child, declared := d.byIRI[term.Value]
	return declared && child.owner == el && !d.isExpressionNode(term) && !d.headEnd(child, el)
}

// standardEndTarget resolves an end through ReferenceSubsetting, then the
// interim sysml:references property, refusing conflicting representations.
func referenceSubsettingIndex(graph *rdf.Graph, meta func(rdf.Term) string, subjects []rdf.Term) map[string][]rdf.Term {
	index := map[string][]rdf.Term{}
	for _, subject := range subjects {
		if meta(subject) != mReferenceSubsetting {
			continue
		}
		if referencing, ok := graph.Object(subject, rdf.SysML+pReferencingFeature); ok && referencing.IsIRI() {
			index[referencing.Value] = append(index[referencing.Value], subject)
		}
	}
	return index
}

func (d *decoder) standardEndTarget(end rdf.Term, in *element) (rdf.Term, bool, error) {
	var relationships []rdf.Term
	appendUnique := func(term rdf.Term) {
		for _, prior := range relationships {
			if prior == term {
				return
			}
		}
		relationships = append(relationships, term)
	}
	for _, relationship := range d.graph.Objects(end, rdf.SysML+pOwnedReferenceSubsetting) {
		if d.metaclass(relationship) == mReferenceSubsetting {
			appendUnique(relationship)
		}
	}
	if len(relationships) == 0 {
		for _, relationship := range d.referenceSubsettings[end.Value] {
			appendUnique(relationship)
		}
	}
	var target rdf.Term
	for _, relationship := range relationships {
		candidate, ok := d.graph.Object(relationship, rdf.SysML+pReferencedFeature)
		if !ok {
			return rdf.Term{}, false, &UnsupportedError{
				What: fmt.Sprintf("the ReferenceSubsetting <%s> of <%s>", relationship.Value, in.iri),
				Note: "it has no sysml:referencedFeature target",
			}
		}
		if target.Value != "" && target != candidate {
			return rdf.Term{}, false, &UnsupportedError{
				What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
				Note: "its ReferenceSubsetting relationships disagree about sysml:referencedFeature",
			}
		}
		target = candidate
	}
	direct, hasDirect := d.graph.Object(end, rdf.SysML+pReferences)
	if hasDirect && target.Value != "" && direct != target {
		return rdf.Term{}, false, &UnsupportedError{
			What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
			Note: "its ReferenceSubsetting and sysml:references targets disagree",
		}
	}
	if target.Value != "" {
		return target, true, nil
	}
	return direct, hasDirect, nil
}

// standardChainText resolves and renders the ordered segments of a chain feature.
func (d *decoder) standardChainText(chain rdf.Term, in *element) ([]string, error) {
	segments, err := d.chainSegments(chain)
	if err != nil {
		return nil, err
	}
	return d.segmentsText(segments, in)
}

// segmentsText spells the segments of a chain as written in in, each resolved
// from the one before it.
func (d *decoder) segmentsText(segments []rdf.Term, in *element) ([]string, error) {
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
			key := segmentKey{member: d.writtenQName(in), operand: operand, name: name, target: target.qname}
			if chosen, ok := d.names.segments[key]; ok {
				spelling = qualifiedNameText(chosen)
			}
		}
		parts = append(parts, spelling)
		operand = target.qname
	}
	return parts, nil
}

// flowEndText renders a FlowEnd (SysML.xtext FlowEnd) as the chain it was
// written as: the features its ReferenceSubsetting names, if any, then the
// feature its FlowFeature redefines. `a.p.fuel` subsets a.p and redefines fuel.
func (d *decoder) flowEndText(end rdf.Term, in *element) (string, error) {
	segments, err := d.flowEndSegments(end, in)
	if err != nil {
		return "", err
	}
	parts, err := d.segmentsText(segments, in)
	if err != nil {
		return "", err
	}
	text := strings.Join(parts, ".")
	mult, err := d.endMultiplicity(end, in)
	if err != nil {
		return "", err
	}
	if mult != "" {
		text = mult + " " + text
	}
	return text, nil
}

func (d *decoder) flowFeatureImplied(el, parent *element) (bool, error) {
	if parent == nil || parent.metaclass != mFlowEnd || el.metaclass != mReferenceUsage {
		return false, nil
	}
	membership, owned := d.owningMembership[el.iri]
	if !owned || d.metaclass(rdf.IRI(membership.iri)) != mFeatureMembership {
		return false, nil
	}
	subject := rdf.IRI(el.iri)
	if d.graph.HasProperty(subject, rdf.SysML+pDeclaredName) ||
		d.graph.HasProperty(subject, rdf.SysML+pDeclaredShortName) ||
		len(d.graph.Objects(subject, rdf.SysML+pOwnedAnnotation)) > 0 {
		return false, nil
	}
	relationships := d.graph.Objects(subject, rdf.SysML+pOwnedRelationship)
	redefinitions := 0
	for _, relation := range relationships {
		meta := d.metaclass(relation)
		if meta == mRedefinition {
			redefinitions++
			continue
		}
		if !impliedRelationshipMetaclasses[meta] || !d.graph.BoolValue(relation, rdf.SysML+pIsImplied) {
			return false, nil
		}
	}
	return redefinitions == 1, nil
}

// flowEndSegments is the chain a FlowEnd is written as: what its
// ReferenceSubsetting names, if anything, then the feature its FlowFeature
// redefines. The FlowFeature is found through ownedFeature, else through the
// FeatureMembership that owns it.
func (d *decoder) flowEndSegments(end rdf.Term, in *element) ([]rdf.Term, error) {
	refuse := func(note string) ([]rdf.Term, error) {
		return nil, &UnsupportedError{What: fmt.Sprintf("the flow end <%s> of <%s>", end.Value, in.iri), Note: note}
	}
	if d.graph.HasProperty(end, rdf.SysML+pDeclaredName) {
		return refuse("it declares a name, and the `from`/`to` form writes the end by its reference chain")
	}
	if d.graph.HasProperty(end, rdf.SysML+pDeclaredShortName) {
		return refuse("it declares a short name, and the `from`/`to` form writes the end by its reference chain")
	}
	if len(d.graph.Objects(end, rdf.SysML+pOwnedAnnotation)) > 0 {
		return refuse("it owns annotations, which the `from`/`to` form cannot carry")
	}
	var segments []rdf.Term
	target, ok, err := d.standardEndTarget(end, in)
	if err != nil {
		return nil, err
	}
	if ok {
		isChain, err := d.chainFeatureTerm(target)
		if err != nil {
			return nil, err
		}
		if isChain {
			if segments, err = d.chainSegments(target); err != nil {
				return nil, err
			}
		} else {
			segments = []rdf.Term{target}
		}
	}
	features := d.graph.Objects(end, rdf.SysML+pOwnedFeature)
	if len(features) == 0 {
		for _, membership := range d.graph.Objects(end, rdf.SysML+pOwnedFeatureMembership) {
			if member, ok := d.graph.Object(membership, rdf.SysML+pMemberElement); ok {
				features = append(features, member)
			}
		}
	}
	var redefined []rdf.Term
	for _, feature := range features {
		redefined = append(redefined, d.graph.Objects(feature, rdf.SysML+relationshipProperty[ast.RelRedefines])...)
	}
	if len(redefined) != 1 {
		return refuse(fmt.Sprintf("its FlowFeature redefines %d features, and a flow end names exactly one", len(redefined)))
	}
	toolkitFeatures := d.toolkitFlowFeatures(end)
	if len(toolkitFeatures) > 0 {
		if len(toolkitFeatures) != 1 || len(features) != 1 || features[0] != toolkitFeatures[0] {
			return refuse(fmt.Sprintf("it owns %d toolkit FlowFeatures, and a flow end writes exactly one reference chain", len(toolkitFeatures)))
		}
		feature := toolkitFeatures[0]
		flowFeature, flowEnd := d.byIRI[feature.Value], d.byIRI[end.Value]
		if flowFeature == nil || flowEnd == nil {
			return refuse("its FlowFeature is not an implied ReferenceUsage owned through a FeatureMembership")
		}
		implied, err := d.flowFeatureImplied(flowFeature, flowEnd)
		if err != nil {
			return nil, err
		}
		if !implied {
			return refuse("its FlowFeature is not an implied ReferenceUsage owned through a FeatureMembership")
		}
		if d.graph.HasProperty(feature, rdf.SysML+pDeclaredName) ||
			d.graph.HasProperty(feature, rdf.SysML+pDeclaredShortName) ||
			len(d.graph.Objects(feature, rdf.SysML+pOwnedAnnotation)) > 0 {
			return refuse("its FlowFeature declares a name or annotations that the `from`/`to` form cannot carry")
		}
		if d.graph.HasProperty(feature, rdf.SysML+pValue) {
			return refuse("its FlowFeature carries a value that the `from`/`to` form cannot carry")
		}
		for _, relation := range d.graph.Objects(feature, rdf.SysML+pOwnedRelationship) {
			meta := d.metaclass(relation)
			if meta == mRedefinition {
				continue
			}
			if !impliedRelationshipMetaclasses[meta] || !d.graph.BoolValue(relation, rdf.SysML+pIsImplied) {
				return refuse(fmt.Sprintf("its FlowFeature owns a %s relationship that the `from`/`to` form cannot carry", meta))
			}
		}
		if name, hasName := d.graph.Lexical(feature, rdf.SysML+pName); hasName {
			redefinedName, err := d.flowEndTargetName(redefined[0])
			if err != nil {
				return nil, err
			}
			if name != redefinedName {
				return refuse("its FlowFeature's derived name does not match the feature it redefines")
			}
		}
		for _, relation := range d.graph.Objects(end, rdf.SysML+pOwnedRelationship) {
			meta := d.metaclass(relation)
			switch {
			case meta == mReferenceSubsetting, meta == mFeatureMembership:
			case impliedRelationshipMetaclasses[meta] && d.graph.BoolValue(relation, rdf.SysML+pIsImplied):
			default:
				return refuse(fmt.Sprintf("it owns a %s relationship that the `from`/`to` form cannot carry", meta))
			}
		}
		if name, hasName := d.graph.Lexical(end, rdf.SysML+pName); hasName {
			var endRedefined []rdf.Term
			endRedefined = append(endRedefined, d.graph.Objects(end, rdf.SysML+relationshipProperty[ast.RelRedefines])...)
			if len(endRedefined) == 0 {
				for _, relation := range d.graph.Objects(end, rdf.SysML+pOwnedRelationship) {
					if d.metaclass(relation) != mRedefinition {
						continue
					}
					if target, ok := d.graph.Object(relation, rdf.SysML+"redefinedFeature"); ok {
						endRedefined = append(endRedefined, target)
					}
				}
			}
			if len(endRedefined) != 1 {
				return refuse(fmt.Sprintf("its derived name has %d redefined features, and a flow end names exactly one", len(endRedefined)))
			}
			redefinedName, err := d.flowEndTargetName(endRedefined[0])
			if err != nil {
				return nil, err
			}
			if name != redefinedName {
				return refuse("its derived name does not match the feature it redefines")
			}
		}
	}
	return append(segments, redefined[0]), nil
}

func (d *decoder) toolkitFlowFeatures(end rdf.Term) []rdf.Term {
	var features []rdf.Term
	seen := map[string]bool{}
	for _, membership := range d.graph.Objects(end, rdf.SysML+pOwnedFeatureMembership) {
		if d.metaclass(membership) != mFeatureMembership {
			continue
		}
		feature, ok := d.graph.Object(membership, rdf.SysML+pMemberElement)
		if !ok || d.metaclass(feature) != mReferenceUsage {
			continue
		}
		owning, ok := d.owningMembership[feature.Value]
		if !ok || owning.iri != membership.Value || seen[feature.Value] {
			continue
		}
		seen[feature.Value] = true
		features = append(features, feature)
	}
	return features
}

func (d *decoder) flowEndTargetName(target rdf.Term) (string, error) {
	if name, ok := d.graph.Lexical(target, rdf.SysML+pName); ok {
		return name, nil
	}
	if !target.IsIRI() {
		return "", &UnsupportedError{What: fmt.Sprintf("the flow-end redefinition target %s", target), Note: "it has no name"}
	}
	el, err := d.referencedElement(target.Value)
	if err != nil {
		return "", err
	}
	segments := identitySegments(el.qname)
	if len(segments) == 0 {
		return "", &UnsupportedError{What: fmt.Sprintf("the flow-end redefinition target <%s>", target.Value), Note: "it has no qualified name"}
	}
	return identityName(segments[len(segments)-1]), nil
}

// standardEndName renders an end's declared name and ReferencesKeyword.
func (d *decoder) standardEndName(end rdf.Term, in *element) (string, error) {
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
	// Only a declared name is written: Element::name is derived, and an end
	// redefining `source` takes that name without declaring one.
	names := d.graph.Objects(end, rdf.SysML+pDeclaredName)
	if len(names) == 0 {
		return "", nil
	}
	if len(names) > 1 {
		return "", &UnsupportedError{
			What: fmt.Sprintf("the connector end <%s> of <%s>", end.Value, in.iri),
			Note: "it declares more than one end name",
		}
	}
	return nameText(names[0].Value) + " " + keyword, nil
}

// legacyEnds reads the positional sysx connector-end representation.
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
	// An error only arises when ends exist; the later write surfaces the refusal.
	if terms, err := d.standardEndFeatures(el); err != nil || len(terms) > 0 {
		return true
	}
	return len(d.graph.Objects(rdf.IRI(el.iri), rdf.OpenSysML+xRelatedFeature)) > 0
}

// inferredEndForm selects the notation form implied by a usage metaclass and arity.
func (d *decoder) inferredEndForm(el *element) string {
	switch el.metaclass {
	case usageMetaclass[ast.UsageBinding]:
		return formEquals
	case usageMetaclass[ast.UsageSuccession]:
		return formFirstThen
	case usageMetaclass[ast.UsageFlow]:
		return formFromTo
	case usageMetaclass[ast.UsageConnection],
		usageMetaclass[ast.UsageInterface],
		usageMetaclass[ast.UsageAllocation],
		usageMetaclass[ast.UsageConnector]:
		if ends, err := d.standardEndFeatures(el); err == nil && len(ends) > 2 {
			return formNary
		}
		return formTo
	}
	return ""
}
