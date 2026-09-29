package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// sequenceMemberKinds are the usage kinds a `then` declares in an action body
// (SysML.xtext ActionNodeMember, plus a target usage member): the named usages
// state a type, the control nodes take their name alone.
var sequenceMemberKinds = map[string]bool{ // value: whether a type is admitted
	"action":         true,
	"perform action": true,
	"state":          true,
	"merge":          false,
	"decide":         false,
	"join":           false,
	"fork":           false,
}

func (m Model) addSequenceSplice(i int, op Operation) (splice, error) {
	if m.Source.Kind() != source.KindSysML {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("action sequencing is not legal in %s source %q",
				m.Source.Kind(), m.Source.Name()),
		}
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if !actionBodyOwner(owner) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("first and then are only admitted in an action body, which %s does not open",
				ownerName(op.Owner)),
		}
	}
	if op.SequenceKeyword != "first" && op.SequenceKeyword != "then" {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("sequence keyword %q is not first or then", op.SequenceKeyword),
		}
	}
	if op.SequenceKeyword == "first" {
		if op.SequenceRef == "" || op.MemberKind != "" || op.MemberName != "" || op.Type != "" {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: "first takes a node reference alone",
			}
		}
	} else if op.SequenceRef != "" {
		if op.MemberKind != "" || op.MemberName != "" || op.Type != "" {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: "a then reference takes a node reference alone",
			}
		}
	} else if op.MemberKind == "" {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: "then takes a node reference or a member declaration, exactly one",
		}
	}
	if op.SequenceRef != "" {
		if _, err := checkFeatureReference(i, "sequence node", op.SequenceRef); err != nil {
			return splice{}, err
		}
		if !m.sequenceNodeVisible(ownerScope, op.SequenceRef) {
			return splice{}, &Error{
				Failure: FailureUnknownTarget, OperationIndex: i,
				Message: fmt.Sprintf("sequence node %q resolves to nothing visible from %s",
					op.SequenceRef, ownerName(op.Owner)),
			}
		}
	}
	var text string
	if op.SequenceRef != "" {
		text = op.SequenceKeyword + " " + op.SequenceRef + ";"
	} else {
		typed, ok := sequenceMemberKinds[op.MemberKind]
		if !ok {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("a then-declared member of kind %q is not admitted: it must be action, perform action, state, merge, decide, join or fork",
					op.MemberKind),
			}
		}
		if op.Type != "" && !typed {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: fmt.Sprintf("kind %q cannot carry a typing target", op.MemberKind),
			}
		}
		if op.MemberName != "" {
			if err := checkName(i, op.MemberName); err != nil {
				e := err.(*Error)
				e.Message = fmt.Sprintf("member name %q is not an identifier", op.MemberName)
				return splice{}, e
			}
			if ownerScope != nil && len(ownerScope.LookupLocalAll(symbolName(op.MemberName))) > 0 {
				return splice{}, &Error{
					Failure: FailureMemberNameTaken, OperationIndex: i,
					Message: fmt.Sprintf("%s already declares %q", ownerName(op.Owner), op.MemberName),
				}
			}
		}
		text = "then " + writeMember(op, memberKinds[op.MemberKind], "", "", "", "")
	}
	members := ast.DeclMembers(owner)
	anchor := -1
	if op.After != "" {
		for j, member := range members {
			if declaredMemberName(member) == op.After {
				anchor = j
				break
			}
		}
		if anchor < 0 {
			return splice{}, &Error{
				Failure: FailureUnknownTarget, OperationIndex: i,
				Message: fmt.Sprintf("%s declares no member named %q", ownerName(op.Owner), op.After),
			}
		}
	}
	before := members
	if anchor >= 0 {
		before = members[:anchor+1]
	}
	if op.SequenceKeyword == "then" && !sequenceSourceBefore(before) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: "`then` has no member before it to sequence from: it sequences the member after it with the nearest feature before it, and none precedes it",
		}
	}
	var ins insertion
	if anchor >= 0 {
		ins = m.memberInsertionAfter(members[anchor], text)
	} else {
		ins = m.memberInsertion(owner, text)
	}
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

// actionBodyOwner reports whether owner opens an action body, the body that
// sequences members with `first` and `then`: an action definition or usage,
// which a `perform action` usage is.
func actionBodyOwner(owner ast.Node) bool {
	switch d := owner.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefAction
	case *ast.Usage:
		return d.Kind == ast.UsageAction
	default:
		return false
	}
}

// sequenceNodeVisible reports whether ref names a node the owner's body can
// sequence to: a member it declares or inherits, a feature reachable from it,
// or one of the implicit start/done markers the grammar reserves.
func (m Model) sequenceNodeVisible(scope *symbols.Scope, ref string) bool {
	if strings.HasPrefix(ref, "$::") {
		// A `$::`-rooted name resolves the way the resolver resolves one: from
		// the document root, falling back to the global index — other
		// documents and the loaded libraries — not the edited document alone.
		segments, ok := source.QualifiedNameSegments(strings.TrimPrefix(ref, "$::"))
		if !ok || len(segments) == 0 {
			return false
		}
		qn := ast.QualifiedNameOf(segments...)
		qn.Global = true
		r, _ := m.resolver()
		_, ok = r.ResolveQualified(scope, qn)
		return ok
	}
	segments, ok := source.QualifiedNameSegments(ref)
	if !ok || len(segments) == 0 {
		return false
	}
	if len(segments) == 1 {
		if segments[0] == "start" || segments[0] == "done" {
			return true
		}
		if _, ok := resolve.ActionNodeOfBody(scope, segments[0]); ok {
			return true
		}
	}
	_, ok = resolve.FeatureSymbolInScope(scope, segments)
	return ok
}

// sequenceSourceBefore reports whether a member a `then` sequences from is
// among members, the members before it in the body.
func sequenceSourceBefore(members []ast.Node) bool {
	for _, member := range members {
		if ast.IsSuccessionSource(member) {
			return true
		}
	}
	return false
}

// declaredMemberName returns the name a member declares, the name a `then X`
// placed after it would sequence from, or "" when it declares none.
func declaredMemberName(member ast.Node) string {
	switch n := unwrapMembership(member).(type) {
	case *ast.Usage:
		if name, _ := ast.EffectiveName(n); name != "" {
			return name
		}
		return n.Ident.ShortName
	case *ast.Definition:
		return identificationName(n.Ident)
	case *ast.Package:
		return identificationName(n.Ident)
	case *ast.Namespace:
		return identificationName(n.Ident)
	case *ast.ActionExecutionNode:
		return n.Name
	case *ast.InitialNode:
		return n.Name()
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
	}
	return ""
}

// identificationName falls back to the short name, the other spelling a
// reference can use.
func identificationName(id ast.Identification) string {
	if id.Name != "" {
		return id.Name
	}
	return id.ShortName
}

// memberInsertionAfter places text, one member's notation, on the line after
// member's, the member's `;` or `}` and any comment on its line staying with it.
func (m Model) memberInsertionAfter(member ast.Node, text string) insertion {
	content := m.Source.Bytes()
	// A member's span runs on over the whitespace and comments after it, so
	// the new member follows its last code token.
	end := member.Span().End()
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF && tok.Span.Offset < member.Span().End(); tok = lx.Next() {
		if !tok.IsTrivia() && tok.Kind != lexer.RegularComment {
			end = tok.Span.End()
		}
	}
	indent := lineIndent(content, member.Span().Offset)
	lineEnd := end
	for lineEnd < len(content) && content[lineEnd] != '\n' {
		lineEnd++
	}
	// What the rest of the anchor's line holds decides the placement. An
	// empty rest, `//` notes, or block comments that close on the line with
	// nothing but whitespace and notes after them put the new member on the
	// next line at the anchor's indent; anything else — a member, the body's
	// `}`, a block comment that does not close on the line, or code after a
	// block comment — takes it inline right after the anchor's last token.
	// The new member is never written inside a comment.
	nextLine := true
	lx = lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF && tok.Span.Offset < lineEnd; tok = lx.Next() {
		if tok.Span.End() <= end || tok.Kind == lexer.Whitespace || tok.Kind == lexer.SLNote {
			continue
		}
		switch tok.Kind {
		case lexer.MLNote, lexer.RegularComment:
			nextLine = tok.Span.End() <= lineEnd
		default:
			nextLine = false
		}
		if !nextLine {
			break
		}
	}
	if !nextLine {
		next := end
		for next < len(content) && (content[next] == ' ' || content[next] == '\t') {
			next++
		}
		return insertion{
			span: source.Span{Offset: end, Len: next - end},
			text: " " + text + " ",
			at:   1,
		}
	}
	if lineEnd == len(content) {
		return insertion{
			span: source.Span{Offset: lineEnd},
			text: "\n" + indent + text + "\n",
			at:   1 + len(indent),
		}
	}
	return insertion{
		span: source.Span{Offset: lineEnd + 1},
		text: indent + text + "\n",
		at:   len(indent),
	}
}
