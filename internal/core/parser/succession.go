package parser

// Member-attached `then` (SysML.xtext EmptySuccessionMember): the keyword
// sequences the member before it with the member after it, so it describes
// neither and is desugared into the same edge representation used by
// `succession first a then b;`.
//
// The grammar admits it only before an occurrence usage, so anything else after
// it is an error. A member with no name has no name for an end to reference, so
// the edge binds that end to the member itself — which is how the notation binds
// it: by position beside the keyword.

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// atChainedFirstSuccession reports whether the `first` at the cursor opens a
// succession whose source end is a '.'-chained or '::'-qualified name. Only
// SuccessionAsUsage's ConnectorEnd reaches a feature chain (KerML.xtext:699);
// InitialNodeMember's memberElement is a plain QualifiedName (SysML.xtext:1385).
func (p *Parser) atChainedFirstSuccession() bool {
	if !p.atKeyword("first") || !p.peekIsName(1) {
		return false
	}
	chained, i := false, 2
	for k := p.peekN(i).Kind; k == lexer.Dot || k == lexer.ColonColon; k = p.peekN(i).Kind {
		if !p.peekIsName(i + 1) {
			return false
		}
		chained, i = true, i+2
	}
	t := p.peekN(i)
	return chained && t.Kind == lexer.Keyword && t.KeywordID == "then"
}

// atTwoEndedFirst reports whether the `first` at the cursor names both ends —
// `first <source> then <target>` — before its member ends, as opposed to the
// one-ended `first <node>;` form that opens an InitialNodeMember.
func (p *Parser) atTwoEndedFirst() bool {
	if !p.atKeyword("first") {
		return false
	}
	for depth, i := 0, 1; i < 60; i++ {
		tok := p.peekN(i)
		switch tok.Kind {
		case lexer.EOF, lexer.Semicolon, lexer.LBrace, lexer.RBrace:
			return false
		case lexer.LParen, lexer.LBracket:
			depth++
		case lexer.RParen, lexer.RBracket:
			depth--
		case lexer.Keyword:
			if depth == 0 && tok.KeywordID == "then" {
				return true
			}
		}
	}
	return false
}

// nonOccurrenceUsageKeywords are the usage keywords a `then` may not precede:
// SysML.xtext lists them under NonOccurrenceUsageElement rather than under
// StructureUsageElement or BehaviorUsageElement.
var nonOccurrenceUsageKeywords = map[string]string{
	"attribute":  "an attribute usage",
	"datatype":   "an attribute usage",
	"feature":    "a feature",
	"ref":        "a reference usage",
	"enum":       "an enumeration usage",
	"binding":    "a binding",
	"bind":       "a binding",
	"succession": "a succession",
}

// namespaceMemberKeywords are the members a `then` may not precede because they
// are namespace members rather than usages.
var namespaceMemberKeywords = map[string]string{
	"package":   "a package",
	"namespace": "a namespace",
	"import":    "an import",
	"alias":     "an alias",
	"doc":       "a documentation comment",
	"comment":   "a comment",
	"rep":       "a textual representation",
	"filter":    "a filter",
}

// bodyBuilder accumulates one body's member list, desugaring a member-attached
// `then` as the two members it joins are parsed. A body loop takes the keyword
// (atSuccession/takeSuccession) and parses the member after it as it normally
// would, so the desugaring is independent of the members a body admits.
type bodyBuilder struct {
	p       *Parser
	members []ast.Node

	// last names the last member a succession can reference, the source a `then`
	// taken next sequences from (ast.IsSuccessionSource). An unnamed member clears it.
	last      string
	lastSpan  source.Span
	lastNode  ast.Node // that member itself, the end a `then` beside an unnamed member binds
	hasMember bool     // whether a member a `then` sequences from precedes, named or not

	// pending is set between taking a `then` and adding the member it prefixes;
	// valid is false once it has been diagnosed, so a bad succession is reported
	// rather than also synthesised.
	pending    bool
	pendingAt  source.Span
	valid      bool
	source     string
	sourceSpan source.Span
	sourceNode ast.Node
}

func (p *Parser) newBodyBuilder() *bodyBuilder {
	return &bodyBuilder{p: p}
}

// atSuccession reports whether `then` prefixes a body member, rather than
// starting a succession edge (`succession first a then b;`) or inline statement.
func (b *bodyBuilder) atSuccession() bool {
	p := b.p
	if !p.atKeyword("then") {
		return false
	}
	next := p.peekN(1)
	// `succession first a then b;` and `then a;` name members; the edge parser
	// reads them. Unreserved node words such as `then done;` are identified by
	// notation shape (see notation.go).
	kw := ""
	if next.Kind == lexer.Keyword {
		kw = next.KeywordID
	} else if w, ok := p.actionNodeWordAt(1); ok {
		kw = w
	} else {
		return false
	}
	if b.namesEdgeEnd(kw) {
		// `then done;`: a keyword this body declares a member with names an edge
		// end, not the kind of a member being declared.
		return false
	}
	if _, ok := namespaceMemberKeywords[kw]; ok {
		// Diagnosed as an illegal target rather than left to parse as a
		// namespace member with the keyword dropped.
		return true
	}
	if featureModifierKeywords[kw] {
		// A modifier cannot be the name of a member, so `then private action
		// whileLoop …` can only be a prefixed declaration.
		return true
	}
	if actionNodeKeywords[kw] {
		// An action node member (SysML.xtext ActionNodeMember): `succession first fork then f;`,
		// `then merge;`, `then send x via p;`, `then loop action { … } until c;`,
		// `then done;`. Taken here so the `then` sequences the node the keyword
		// declares, rather than being read as an edge naming the keyword itself.
		return true
	}
	if kw == "use" {
		// A use case is the only kind spelled in two words.
		if word := p.peekN(2); word.Kind != lexer.Keyword || word.KeywordID != "case" {
			return false
		}
	} else {
		_, isUsage := p.usageKind(kw)
		_, isDef := p.definitionKind(kw)
		if !isUsage && !isDef {
			// `first x;` and the guarded forms: edges with their own parsers.
			return false
		}
		// A `perform` declares an occurrence usage, so a `then` between it and a
		// named member before it is a succession over the two; with no named
		// member before it the keyword chains a statement of one node's body.
		if kw == "perform" && b.last != "" {
			return true
		}
		if startsInlineSuccessionStatement(next) && kw != "action" {
			// `then assign …`, `then perform …`, `then while …`, `then if …`
			// sequence statements inside one node body, where the lowered block
			// already runs its statements in order.
			return false
		}
	}
	// Anything else after the keyword declares a member the `then` prefixes,
	// anonymous (`then part;`, `then action { … }`) or named (`then action b;`).
	return true
}

// actionNodeKeywords are the words that begin an action node member
// (SysML.xtext ActionNodeMember, plus the final node `done` reaches): a node in
// the token flow that declares no name unless the author writes one, so a `then`
// before it sequences to the node itself. `if` and `else` are not among them:
// after a decision they begin a guarded or default target succession, not a node.
var actionNodeKeywords = map[string]bool{
	"send":      true,
	"accept":    true,
	"assign":    true,
	"terminate": true,
	"while":     true,
	"loop":      true,
	"for":       true,
	"fork":      true,
	"join":      true,
	"merge":     true,
	"decide":    true,
	"done":      true,
}

// namesEdgeEnd reports whether a keyword after `then` names an edge end rather
// than the kind of a member being declared: `then <kw>;` and `then <kw> <name>;`
// are ambiguous, and only a name this body already declares can be an end.
func (b *bodyBuilder) namesEdgeEnd(kw string) bool {
	if !b.declares(kw) {
		return false
	}
	p := b.p
	switch p.peekN(2).Kind {
	case lexer.Semicolon:
		return true
	case lexer.Identifier, lexer.Keyword, lexer.UnrestrictedName:
		return p.peekN(3).Kind == lexer.Semicolon
	}
	return false
}

// declares reports whether a member of this body was declared with this name.
func (b *bodyBuilder) declares(name string) bool {
	for _, m := range b.members {
		if memberDeclaredName(m) == name {
			return true
		}
	}
	return false
}

// takeSuccession consumes a member-attached `then`, reporting the forms the
// grammar does not allow. The member it prefixes is parsed by the caller.
func (b *bodyBuilder) takeSuccession() {
	p := b.p
	tok := p.advance() // consume 'then'
	if b.pending {
		p.error(tok.Span, "`then` cannot follow another `then`: a succession sequences two members, so each keyword needs a member between it and the next")
		return
	}
	b.pending, b.pendingAt, b.valid = true, tok.Span, true
	b.source, b.sourceSpan, b.sourceNode = b.last, b.lastSpan, b.lastNode

	// One diagnostic per keyword: the first thing wrong with it is enough to
	// say why no succession was built.
	switch what, illegal := b.illegalTarget(); {
	case illegal:
		p.error(tok.Span, fmt.Sprintf("`then` cannot sequence %s: it sequences the members either side of it, which the notation allows only before an occurrence usage such as a part, item, action or state", what))
	case !b.hasMember:
		p.error(tok.Span, "`then` has no member before it to sequence from: it sequences the member after it with the nearest feature before it, and none precedes it")
	default:
		return
	}
	b.valid = false
}

// illegalTarget describes the member beginning at the parser's position, just
// past a taken `then`, when the grammar does not allow a succession before it.
func (b *bodyBuilder) illegalTarget() (string, bool) {
	p := b.p
	for i := 0; ; i++ {
		tok := p.peekN(i)
		kw := ""
		if tok.Kind == lexer.Keyword {
			kw = tok.KeywordID
		} else if _, ok := p.actionNodeWordAt(i); ok {
			// An action node whose word the lexer does not reserve is a legal
			// target, as the keyword-spelled ones are.
			return "", false
		} else if tok.Kind == lexer.Identifier {
			// A bare name is a DefaultReferenceUsage.
			return "a reference usage", true
		} else {
			return "", false
		}
		if what, ok := namespaceMemberKeywords[kw]; ok {
			return what, true
		}
		if _, isDef := p.definitionKind(kw); isDef && p.peekN(i+1).Kind == lexer.Keyword && p.peekN(i+1).KeywordID == "def" {
			return "a definition", true
		}
		if _, isUsage := p.usageKind(kw); isUsage {
			// `ref` is both a modifier and a usage keyword: `then ref part x;`
			// is a part usage, `then ref x;` a reference usage.
			if what, isNon := nonOccurrenceUsageKeywords[kw]; isNon && !(kw == "ref" && b.kindFollows(i+1)) {
				return what, true
			}
			return "", false
		}
		if !featureModifierKeywords[kw] {
			return "", false
		}
	}
}

// kindFollows reports whether the token at offset i is a usage kind keyword,
// which tells a modifier apart from the kind it qualifies.
func (b *bodyBuilder) kindFollows(i int) bool {
	tok := b.p.peekN(i)
	if tok.Kind != lexer.Keyword {
		return false
	}
	_, ok := b.p.usageKind(tok.KeywordID)
	return ok
}

// add appends a member, synthesising the succession edge of a pending `then`
// after it. A succession that cannot be built is reported rather than dropped,
// since dropping it would silently change execution order.
func (b *bodyBuilder) add(m ast.Node) {
	if m == nil {
		return
	}
	pending, at, valid := b.pending, b.pendingAt, b.valid
	b.pending = false

	// `then <target>;` leaves its source to the member before it, the same member
	// a member-attached `then` sequences from, rather than to a consumer's guess.
	// An unnamed member before it is bound by position, as it is on the other end.
	if b.last != "" {
		if source := unnamedEdgeSource(m); source != nil {
			*source = memberReference(b.last, b.lastSpan)
			markImpliedSource(m)
		}
	} else if b.lastNode != nil {
		bindPositionalSource(memberNode(m), b.lastNode)
	} else if !b.hasMember {
		b.reportSourceless(memberNode(m))
	}

	target := memberDeclaredName(m)
	b.members = append(b.members, m)
	if ast.IsSuccessionSource(m) {
		// An unnamed member clears the source name too: keeping an older name would
		// sequence from a member other than the one before the keyword.
		b.hasMember = true
		b.last, b.lastSpan, b.lastNode = target, m.Span(), memberNode(m)
	}

	if !pending || !valid {
		return
	}
	edge := synthesizeSuccession(b.source, b.sourceSpan, target, m.Span(), at)
	if b.source == "" {
		edge.Source, edge.SourceMember = nil, b.sourceNode
	}
	if target == "" {
		edge.Target, edge.TargetMember = nil, memberNode(m)
	}
	b.members = append(b.members, edge)
}

// reportSourceless diagnoses a one-name edge (`then b;`, `if x then b;`, `else b;`)
// that no member a succession can sequence from precedes, as takeSuccession does
// for a member-attached `then`.
func (b *bodyBuilder) reportSourceless(m ast.Node) {
	if unnamedEdgeSource(m) == nil {
		return
	}
	word := "then"
	if edge, ok := m.(*ast.ControlFlowEdge); ok && edge.IsElse {
		word = "else"
	}
	b.p.error(m.Span(), "`"+word+"` has no member before it to sequence from: it sequences the member it names with the nearest feature before it, and none precedes it")
}

// bindPositionalSource binds the source of a one-name edge (`then b;`, `if x
// then b;`, `else b;`) to the member before it when that member declares no name
// of its own, which the notation leaves the edge to reach by position.
func bindPositionalSource(m, source ast.Node) {
	switch edge := m.(type) {
	case *ast.SuccessionEdge:
		if unnamedEdgeSource(edge) != nil {
			edge.Source, edge.SourceMember = nil, source
		}
	case *ast.ControlFlowEdge:
		if unnamedEdgeSource(edge) != nil {
			edge.Source, edge.SourceMember = nil, source
		}
	}
}

// markImpliedSource records that the source end of an edge member is the member
// before it, which the notation supplied rather than the author naming it.
func markImpliedSource(m ast.Node) {
	switch edge := m.(type) {
	case *ast.SuccessionEdge:
		edge.SourceImplied = true
	case *ast.ControlFlowEdge:
		edge.SourceImplied = true
	}
}

// memberNode addresses the member a membership wraps, the node a succession end
// bound by position refers to.
func memberNode(m ast.Node) ast.Node {
	if ms, ok := m.(*ast.Membership); ok && ms.Member != nil {
		return ms.Member
	}
	return m
}

// unnamedEdgeSource addresses the source end of an edge member, guarded or not,
// when the one-name notation (`then b;`, `then b if x;`) left it unnamed.
func unnamedEdgeSource(m ast.Node) **ast.QualifiedName {
	var end **ast.QualifiedName
	switch n := m.(type) {
	case *ast.SuccessionEdge:
		end = &n.Source
	case *ast.ControlFlowEdge:
		end = &n.Source
	default:
		return nil
	}
	if *end != nil && len((*end).Parts) != 0 {
		return nil
	}
	return end
}

// finish returns the body's members, reporting a `then` that no member follows.
func (b *bodyBuilder) finish() []ast.Node {
	if b.pending {
		b.p.error(b.pendingAt, "`then` has no member after it to sequence to: the keyword sequences the member after it with the member before it")
		b.pending = false
	}
	return b.members
}

// synthesizeSuccession builds the edge a member-attached `then` desugars to,
// spanned at the keyword so diagnostics point at what the author wrote.
func synthesizeSuccession(source string, sourceSpan source.Span, target string, targetSpan, at source.Span) *ast.SuccessionEdge {
	edge := &ast.SuccessionEdge{
		Source:        memberReference(source, sourceSpan),
		Target:        memberReference(target, targetSpan),
		SourceImplied: true,
		TargetImplied: true,
	}
	edge.NodeSpan = at
	return edge
}

// memberReference names a member of the same body, spanned at its declaration.
func memberReference(name string, sp source.Span) *ast.QualifiedName {
	qn := &ast.QualifiedName{Parts: []ast.NameSegment{{Text: name, Span: sp}}}
	qn.NodeSpan = sp
	return qn
}

// memberDeclaredName returns the name a succession can reference a member by,
// or "" when it declares none.
func memberDeclaredName(member ast.Node) string {
	switch n := member.(type) {
	case *ast.Membership:
		if n.Member == nil {
			return ""
		}
		return memberDeclaredName(n.Member)
	case *ast.Usage:
		// A usage with no declared name answers to the feature it references or
		// redefines (`perform doIt;`), the name lowering resolves it by.
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return n.Ident.ShortName
	case *ast.Definition:
		return identifierName(n.Ident)
	case *ast.Package:
		return identifierName(n.Ident)
	case *ast.Namespace:
		return identifierName(n.Ident)
	case *ast.ActionExecutionNode:
		return n.Name
	case *ast.InitialNode:
		return n.Name()
	case *ast.FinalNode:
		return ""
	case *ast.ForkNode:
		return n.Name
	case *ast.JoinNode:
		return n.Name
	case *ast.MergeNode:
		return n.Name
	case *ast.DecisionNode:
		return n.Name
	case *ast.StateNode:
		return n.Name
	case *ast.SubstateMember:
		return n.Name
	case *ast.PseudostateNode:
		return n.Name
	}
	return ""
}

// identifierName falls back to the short name, the other spelling a reference
// can use.
func identifierName(id ast.Identification) string {
	if id.Name != "" {
		return id.Name
	}
	return id.ShortName
}
