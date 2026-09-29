package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// sequenceMemberKinds are the usage kinds a `then` declares in an action body
// (SysML.xtext:1368 ActionNodeMember, plus a target usage member): the named usages
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
			Message: fmt.Sprintf("action-body items are only admitted in an action body, which %s does not open",
				ownerName(op.Owner)),
		}
	}
	if op.SequenceKeyword != "" && op.SequenceKeyword != "first" &&
		op.SequenceKeyword != "then" && op.SequenceKeyword != "if" &&
		op.SequenceKeyword != "else" {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("action-body keyword %q is not first, then, if, else or empty", op.SequenceKeyword),
		}
	}
	memberIndent := m.ownerMemberIndent(owner)
	base := lineIndent(m.Source.Bytes(), owner.Span().Offset)
	unit := strings.TrimPrefix(memberIndent, base)
	text, err := m.formatSequenceItem(i, op.Owner, ownerScope, op, 0, memberIndent, unit)
	if err != nil {
		return splice{}, err
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
	if requiresSequenceSource(op.SequenceKeyword) && !sequenceSourceBefore(before) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("%q has no source member before it to sequence from", op.SequenceKeyword),
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

func (m Model) formatSequenceItem(i int, owner string, scope *symbols.Scope, op Operation, depth int, memberIndent, unit string) (string, error) {
	if op.Kind != OpAddSequence {
		return "", sequenceError(i, "body items must be add-sequence operations")
	}
	if depth > 128 {
		return "", sequenceError(i, "nested action-body items exceed the maximum depth")
	}
	if depth > 0 && (op.Owner != "" || op.After != "") {
		return "", sequenceError(i, "nested body items cannot specify owner or after")
	}
	if op.SequenceKeyword != "" && op.SequenceKeyword != "first" &&
		op.SequenceKeyword != "then" && op.SequenceKeyword != "if" &&
		op.SequenceKeyword != "else" {
		return "", sequenceError(i, fmt.Sprintf("action-body keyword %q is not first, then, if, else or empty", op.SequenceKeyword))
	}
	if err := validateSourceMultiplicity(i, op.SequenceKeyword, op.Multiplicity); err != nil {
		return "", err
	}

	if op.SequenceKeyword == "first" || op.SequenceKeyword == "then" ||
		op.SequenceKeyword == "if" || op.SequenceKeyword == "else" {
		if op.SequenceRef != "" {
			return m.formatSequenceReference(i, owner, scope, op)
		}
	}
	if op.SequenceRef != "" {
		return "", sequenceError(i, "a plain action-body member cannot name a sequence reference")
	}
	if op.SequenceKeyword == "if" || op.SequenceKeyword == "else" {
		return "", sequenceError(i, fmt.Sprintf("%s succession requires a sequence reference", op.SequenceKeyword))
	}
	if op.SequenceKeyword == "first" {
		return "", sequenceError(i, "first takes a sequence reference alone")
	}
	if op.SequenceKeyword == "then" && op.MemberKind == "" {
		return "", sequenceError(i, "then takes a sequence reference or a member declaration")
	}
	if op.MemberKind == "" {
		return "", sequenceError(i, "a plain action-body item requires a member kind")
	}
	if _, ok := sequenceMemberKinds[op.MemberKind]; ok {
		return m.formatSequenceDeclaration(i, owner, scope, op)
	}
	return m.formatActionStatement(i, owner, scope, op, depth, memberIndent, unit)
}

func (m Model) formatSequenceReference(i int, owner string, scope *symbols.Scope, op Operation) (string, error) {
	if op.SequenceRef == "" {
		return "", sequenceError(i, "sequence reference is empty")
	}
	if op.MemberKind != "" || op.MemberName != "" || op.Type != "" ||
		op.SequenceValue != "" || op.SequenceTarget != "" || op.SequenceVia != "" ||
		op.SequenceUntil != "" || op.SequenceParameter != "" ||
		len(op.SequenceBody) > 0 || len(op.SequenceElse) > 0 {
		return "", sequenceError(i, "a sequence reference cannot carry member or statement fields")
	}
	switch op.SequenceKeyword {
	case "first", "then", "else":
		if op.SequenceCondition != "" {
			return "", sequenceError(i, fmt.Sprintf("%s succession does not take a condition", op.SequenceKeyword))
		}
	case "if":
		if op.SequenceCondition == "" {
			return "", sequenceError(i, "guarded then requires a guard")
		}
		if err := m.checkExpression(i, "guard", owner, op.SequenceCondition); err != nil {
			return "", err
		}
	default:
		return "", sequenceError(i, "a sequence reference requires first, then, if or else")
	}
	if _, err := checkFeatureReference(i, "sequence node", op.SequenceRef); err != nil {
		return "", err
	}
	if !m.sequenceNodeVisible(scope, op.SequenceRef) {
		return "", &Error{
			Failure: FailureUnknownTarget, OperationIndex: i,
			Message: fmt.Sprintf("sequence node %q resolves to nothing visible from %s",
				op.SequenceRef, ownerName(owner)),
		}
	}
	switch op.SequenceKeyword {
	case "first":
		return "first " + op.SequenceRef + ";", nil
	case "then":
		if op.Multiplicity != "" {
			return op.Multiplicity + " then " + op.SequenceRef + ";", nil
		}
		return "then " + op.SequenceRef + ";", nil
	case "if":
		return "if " + op.SequenceCondition + " then " + op.SequenceRef + ";", nil
	case "else":
		if op.Multiplicity != "" || op.SequenceCondition != "" {
			return "", sequenceError(i, "else succession takes a reference alone")
		}
		return "else " + op.SequenceRef + ";", nil
	default:
		return "", sequenceError(i, "a sequence reference requires first, then, if or else")
	}
}

func (m Model) formatSequenceDeclaration(i int, owner string, scope *symbols.Scope, op Operation) (string, error) {
	typed, admitted := sequenceMemberKinds[op.MemberKind]
	if !admitted {
		return "", sequenceError(i, fmt.Sprintf("member kind %q is not admitted in action sequencing", op.MemberKind))
	}
	if op.SequenceCondition != "" || op.SequenceValue != "" || op.SequenceTarget != "" ||
		op.SequenceVia != "" || op.SequenceUntil != "" || op.SequenceParameter != "" ||
		len(op.SequenceBody) > 0 || len(op.SequenceElse) > 0 {
		return "", sequenceError(i, "a sequence declaration cannot carry statement fields")
	}
	if op.Type != "" && !typed {
		return "", sequenceError(i, fmt.Sprintf("kind %q cannot carry a typing target", op.MemberKind))
	}
	if op.Type != "" {
		if _, err := checkFeatureReference(i, "typing target", op.Type); err != nil {
			return "", err
		}
	}
	if op.MemberName != "" {
		if err := checkName(i, op.MemberName); err != nil {
			e := err.(*Error)
			e.Message = fmt.Sprintf("member name %q is not an identifier", op.MemberName)
			return "", e
		}
		if scope != nil && len(scope.LookupLocalAll(symbolName(op.MemberName))) > 0 {
			return "", &Error{
				Failure: FailureMemberNameTaken, OperationIndex: i,
				Message: fmt.Sprintf("%s already declares %q", ownerName(owner), op.MemberName),
			}
		}
	}
	prefix := ""
	if op.SequenceKeyword == "then" {
		prefix = "then "
		if op.Multiplicity != "" {
			prefix += op.Multiplicity + " "
		}
	} else if op.SequenceKeyword != "" {
		return "", sequenceError(i, fmt.Sprintf("member declaration cannot use keyword %q", op.SequenceKeyword))
	}
	member := op
	member.Multiplicity = ""
	return prefix + writeMember(member, memberKinds[op.MemberKind]), nil
}

func (m Model) formatActionStatement(i int, owner string, scope *symbols.Scope, op Operation, depth int, memberIndent, unit string) (string, error) {
	if op.MemberName != "" {
		return "", sequenceError(i, "action-body statement kinds do not admit member_name")
	}
	if op.SequenceKeyword != "" && op.SequenceKeyword != "then" {
		return "", sequenceError(i, fmt.Sprintf("statement kind %q only admits keyword then or empty", op.MemberKind))
	}
	prefix := ""
	if op.SequenceKeyword == "then" {
		prefix = "then "
		if op.Multiplicity != "" {
			prefix += op.Multiplicity + " "
		}
	}
	bodyText, err := m.formatNestedBody(i, owner, scope, op.SequenceBody, depth, memberIndent, unit)
	if err != nil {
		return "", err
	}
	var text string
	switch op.MemberKind {
	case "accept":
		if op.SequenceParameter == "" {
			return "", sequenceError(i, "accept requires a payload parameter")
		}
		if err := checkName(i, op.SequenceParameter); err != nil {
			return "", err
		}
		if op.Type != "" {
			if _, err := checkFeatureReference(i, "accept type", op.Type); err != nil {
				return "", err
			}
		}
		if op.SequenceVia != "" {
			if err := m.checkExpression(i, "via port", owner, op.SequenceVia); err != nil {
				return "", err
			}
		}
		if err := noStatementFields(i, op, "accept", "type", "via", "parameter", "multiplicity"); err != nil {
			return "", err
		}
		text = "accept " + op.SequenceParameter
		if op.Type != "" {
			text += " : " + op.Type
		}
		if op.SequenceVia != "" {
			text += " via " + op.SequenceVia
		}
		text += ";"
	case "send":
		if op.SequenceValue == "" {
			return "", sequenceError(i, "send requires a payload value")
		}
		if err := m.checkExpression(i, "send payload", owner, op.SequenceValue); err != nil {
			return "", err
		}
		if op.SequenceVia != "" {
			if err := m.checkExpression(i, "via port", owner, op.SequenceVia); err != nil {
				return "", err
			}
		}
		if op.SequenceTarget != "" {
			if err := m.checkExpression(i, "send receiver", owner, op.SequenceTarget); err != nil {
				return "", err
			}
		}
		if err := noStatementFields(i, op, "send", "value", "target", "via", "multiplicity"); err != nil {
			return "", err
		}
		text = "send " + op.SequenceValue
		if op.SequenceVia != "" {
			text += " via " + op.SequenceVia
		}
		if op.SequenceTarget != "" {
			text += " to " + op.SequenceTarget
		}
		text += ";"
	case "assign":
		if op.SequenceTarget == "" || op.SequenceValue == "" {
			return "", sequenceError(i, "assign requires both target and value")
		}
		if _, err := checkFeatureReference(i, "assignment target", op.SequenceTarget); err != nil {
			return "", err
		}
		if err := m.checkExpression(i, "assigned value", owner, op.SequenceValue); err != nil {
			return "", err
		}
		if err := noStatementFields(i, op, "assign", "target", "value", "multiplicity"); err != nil {
			return "", err
		}
		text = "assign " + op.SequenceTarget + " := " + op.SequenceValue + ";"
	case "if":
		if op.SequenceCondition == "" {
			return "", sequenceError(i, "if requires a condition")
		}
		if err := m.checkExpression(i, "condition", owner, op.SequenceCondition); err != nil {
			return "", err
		}
		text = "if " + op.SequenceCondition + bodyText
		if len(op.SequenceElse) > 0 {
			elseText, err := m.formatNestedBody(i, owner, scope, op.SequenceElse, depth, memberIndent, unit)
			if err != nil {
				return "", err
			}
			text += " else" + elseText
		}
		if err := noStatementFields(i, op, "if", "condition", "body", "else_body", "multiplicity"); err != nil {
			return "", err
		}
	case "while":
		if op.SequenceCondition == "" {
			return "", sequenceError(i, "while requires a condition")
		}
		if err := m.checkExpression(i, "loop condition", owner, op.SequenceCondition); err != nil {
			return "", err
		}
		if op.SequenceUntil != "" {
			if err := m.checkExpression(i, "until condition", owner, op.SequenceUntil); err != nil {
				return "", err
			}
		}
		text = "while " + op.SequenceCondition + bodyText
		if op.SequenceUntil != "" {
			text += " until " + op.SequenceUntil + ";"
		}
		if err := noStatementFields(i, op, "while", "condition", "body", "until", "multiplicity"); err != nil {
			return "", err
		}
	case "loop":
		if op.SequenceCondition != "" {
			return "", sequenceError(i, "loop does not take a condition")
		}
		if op.SequenceUntil != "" {
			if err := m.checkExpression(i, "until condition", owner, op.SequenceUntil); err != nil {
				return "", err
			}
		}
		text = "loop" + bodyText
		if op.SequenceUntil != "" {
			text += " until " + op.SequenceUntil + ";"
		}
		if err := noStatementFields(i, op, "loop", "body", "until", "multiplicity"); err != nil {
			return "", err
		}
	case "for":
		if op.SequenceParameter == "" || op.SequenceValue == "" {
			return "", sequenceError(i, "for requires a variable and collection")
		}
		if err := checkName(i, op.SequenceParameter); err != nil {
			return "", err
		}
		if op.Type != "" {
			if _, err := checkFeatureReference(i, "for variable type", op.Type); err != nil {
				return "", err
			}
		}
		if err := m.checkExpression(i, "for collection", owner, op.SequenceValue); err != nil {
			return "", err
		}
		if err := noStatementFields(i, op, "for", "parameter", "type", "value", "body", "multiplicity"); err != nil {
			return "", err
		}
		text = "for " + op.SequenceParameter
		if op.Type != "" {
			text += " : " + op.Type
		}
		text += " in " + op.SequenceValue + bodyText
	case "terminate":
		if op.SequenceValue != "" {
			if err := m.checkExpression(i, "terminate occurrence", owner, op.SequenceValue); err != nil {
				return "", err
			}
		}
		if err := noStatementFields(i, op, "terminate", "value", "multiplicity"); err != nil {
			return "", err
		}
		text = "terminate"
		if op.SequenceValue != "" {
			text += " " + op.SequenceValue
		}
		text += ";"
	default:
		return "", sequenceError(i, fmt.Sprintf("member kind %q is not an action-body statement", op.MemberKind))
	}
	return prefix + text, nil
}

func (m Model) formatNestedBody(i int, owner string, scope *symbols.Scope, body []Operation, depth int, memberIndent, unit string) (string, error) {
	if len(body) == 0 {
		return " { }", nil
	}
	var lines []string
	hasSource := false
	for _, item := range body {
		if item.SequenceKeyword == "then" && !hasSource {
			return "", sequenceError(i, "`then` in a nested body has no earlier source item")
		}
		if requiresSequenceSource(item.SequenceKeyword) && !hasSource {
			return "", sequenceError(i, fmt.Sprintf("%q in a nested body has no earlier source item", item.SequenceKeyword))
		}
		formatted, err := m.formatSequenceItem(i, owner, scope, item, depth+1, memberIndent, unit)
		if err != nil {
			return "", err
		}
		lines = append(lines, memberIndent+strings.Repeat(unit, depth+1)+formatted)
		if sequenceItemIsSource(item) {
			hasSource = true
		}
	}
	return " {\n" + strings.Join(lines, "\n") + "\n" + memberIndent + strings.Repeat(unit, depth) + "}", nil
}

func sequenceItemIsSource(op Operation) bool {
	if op.SequenceKeyword == "first" {
		return true
	}
	if op.SequenceRef != "" {
		return false
	}
	return op.MemberKind != ""
}

func requiresSequenceSource(keyword string) bool {
	return keyword == "then" || keyword == "if" || keyword == "else"
}

func noStatementFields(i int, op Operation, kind string, allowed ...string) error {
	allow := map[string]bool{}
	for _, field := range allowed {
		allow[field] = true
	}
	fields := []struct {
		name  string
		value bool
	}{
		{"condition", op.SequenceCondition != ""},
		{"value", op.SequenceValue != ""},
		{"target", op.SequenceTarget != ""},
		{"via", op.SequenceVia != ""},
		{"until", op.SequenceUntil != ""},
		{"body", len(op.SequenceBody) > 0},
		{"else_body", len(op.SequenceElse) > 0},
		{"parameter", op.SequenceParameter != ""},
		{"type", op.Type != ""},
		{"multiplicity", op.Multiplicity != ""},
	}
	for _, field := range fields {
		if field.value && !allow[field.name] {
			return sequenceError(i, fmt.Sprintf("member kind %q does not admit field %s", kind, field.name))
		}
	}
	return nil
}

func sequenceError(i int, message string) error {
	return &Error{Failure: FailureIllegalKind, OperationIndex: i, Message: message}
}

func validateSourceMultiplicity(i int, keyword, multiplicity string) error {
	if multiplicity == "" {
		return nil
	}
	if keyword != "then" {
		return sequenceError(i, "source multiplicity is only legal with keyword then")
	}
	text := strings.TrimSpace(multiplicity)
	sf := source.New("<multiplicity>", []byte(text))
	refuse := func(reason string, diags []diag.Diagnostic) error {
		return &Error{
			Failure:        FailureInvalidValue,
			OperationIndex: i,
			Diagnostics:    diags,
			Diagnosed:      sf,
			Message:        fmt.Sprintf("source multiplicity %q %s", multiplicity, reason),
		}
	}
	if text == "" {
		return refuse("is empty", []diag.Diagnostic{{
			Severity: diag.SeverityError,
			Span:     source.Span{},
			Message:  "expected a multiplicity beginning with '['",
			Code:     "syntax",
			Source:   "syntax",
		}})
	}
	p := parser.New(sf)
	mult := p.ParseMultiplicity()
	if len(p.Diagnostics) > 0 {
		return refuse("does not parse as a multiplicity", parseDiagnostics(p.Diagnostics))
	}
	if mult == nil {
		return refuse("does not parse as a multiplicity", []diag.Diagnostic{{
			Severity: diag.SeverityError,
			Span:     source.Span{Offset: 0, Len: len(text)},
			Message:  "expected a multiplicity beginning with '['",
			Code:     "syntax",
			Source:   "syntax",
		}})
	}
	if end := mult.Span().End(); end != len(text) {
		return refuse(fmt.Sprintf("is not one multiplicity: %q is left over", text[end:]),
			[]diag.Diagnostic{{
				Severity: diag.SeverityError,
				Span:     source.Span{Offset: end, Len: len(text) - end},
				Message:  "unexpected text after multiplicity",
				Code:     "syntax",
				Source:   "syntax",
			}})
	}
	return nil
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
