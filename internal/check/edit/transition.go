package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func (m Model) addTransitionSplice(i int, op Operation) (splice, error) {
	if m.Source.Kind() != source.KindSysML {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("transitions are not legal in %s source %q",
				m.Source.Kind(), m.Source.Name()),
		}
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if !stateBodyOwner(owner) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("transitions are only admitted in a state body, which %s does not open",
				ownerName(op.Owner)),
		}
	}
	if err := checkEnd(i, "target", op.TransitionTarget); err != nil {
		return splice{}, err
	}
	if op.Initial {
		if op.TransitionName != "" || op.TransitionSource != "" ||
			op.Trigger != "" || op.Guard != "" || op.Effect != "" {
			return splice{}, &Error{
				Failure: FailureIllegalKind, OperationIndex: i,
				Message: "an entry transition cannot have a name, source, trigger, guard or effect",
			}
		}
		for _, member := range ast.DeclMembers(owner) {
			member = unwrapMembership(member)
			if _, ok := member.(*ast.EntryMember); ok {
				return splice{}, &Error{
					Failure: FailureIllegalKind, OperationIndex: i,
					Message: fmt.Sprintf("%s already has an entry action", ownerName(op.Owner)),
				}
			}
		}
		ins := m.memberInsertion(owner, "entry; then "+op.TransitionTarget+";")
		return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
	}
	if op.TransitionName != "" {
		if err := checkName(i, op.TransitionName); err != nil {
			e := err.(*Error)
			e.Message = fmt.Sprintf("transition name %q is not an identifier", op.TransitionName)
			return splice{}, e
		}
	}
	if err := checkEnd(i, "source", op.TransitionSource); err != nil {
		return splice{}, err
	}
	if op.TransitionName != "" && ownerScope != nil &&
		len(ownerScope.LookupLocalAll(op.TransitionName)) > 0 {
		return splice{}, &Error{
			Failure: FailureMemberNameTaken, OperationIndex: i,
			Message: fmt.Sprintf("%s already declares %q", ownerName(op.Owner), op.TransitionName),
		}
	}
	text := writeTransition(op)
	if err := validateTransitionText(i, op, text); err != nil {
		return splice{}, err
	}
	ins := m.memberInsertion(owner, text)
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

func stateBodyOwner(owner ast.Node) bool {
	switch d := owner.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefState
	case *ast.Usage:
		return d.Kind == ast.UsageState
	case *ast.SubstateMember:
		return true
	default:
		return false
	}
}

func writeTransition(op Operation) string {
	var text strings.Builder
	text.WriteString("transition")
	if op.TransitionName != "" {
		text.WriteByte(' ')
		text.WriteString(op.TransitionName)
	}
	text.WriteString(" first ")
	text.WriteString(op.TransitionSource)
	if op.Trigger != "" {
		text.WriteString(" accept ")
		text.WriteString(op.Trigger)
	}
	if op.Guard != "" {
		text.WriteString(" if ")
		text.WriteString(op.Guard)
	}
	if op.Effect != "" {
		text.WriteString(" do ")
		text.WriteString(op.Effect)
	}
	text.WriteString(" then ")
	text.WriteString(op.TransitionTarget)
	text.WriteByte(';')
	return text.String()
}

func validateTransitionText(i int, op Operation, text string) error {
	wrapped := "state def __Transition {\n" + text + "\n}"
	p := parser.New(source.New("<transition>", []byte(wrapped)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		return invalidTransitionText(i)
	}
	if len(root.Members) != 1 {
		return invalidTransitionText(i)
	}
	owner := unwrapMembership(root.Members[0])
	if _, ok := owner.(*ast.Definition); !ok {
		return invalidTransitionText(i)
	}
	members := ast.DeclMembers(owner)
	if len(members) != 1 {
		return invalidTransitionText(i)
	}
	transition, ok := unwrapMembership(members[0]).(*ast.TransitionMember)
	if !ok || transition.Source == nil || transition.Target == nil {
		return invalidTransitionText(i)
	}
	nameText, nameOK := transitionSpanText(wrapped, transition.NameSpan)
	sourceText, sourceOK := transitionQualifiedNameText(wrapped, transition.Source)
	targetText, targetOK := transitionQualifiedNameText(wrapped, transition.Target)
	triggerText, triggerOK := "", op.Trigger == ""
	if transition.Trigger != nil {
		triggerText, triggerOK = transitionTriggerText(wrapped, transition)
	}
	guardText, guardOK := "", op.Guard == ""
	if transition.Guard != nil {
		guardText, guardOK = transitionNodeText(wrapped, transition.Guard)
	}
	effectText, effectOK := "", op.Effect == ""
	if transition.HasEffect {
		effectText, effectOK = transitionEffectText(wrapped, transition.Effect)
	}
	if !nameOK || !sourceOK || !targetOK || !triggerOK || !guardOK || !effectOK ||
		nameText != op.TransitionName ||
		sourceText != op.TransitionSource ||
		targetText != op.TransitionTarget ||
		triggerText != op.Trigger ||
		guardText != op.Guard ||
		effectText != op.Effect {
		return invalidTransitionText(i)
	}
	return nil
}

func invalidTransitionText(i int) error {
	return &Error{
		Failure: FailureInvalidValue, OperationIndex: i,
		Message: "transition trigger, guard or effect does not form one grammar-admissible transition",
	}
}

func transitionSpanText(text string, span source.Span) (string, bool) {
	end := span.Offset + span.Len
	if span.Offset < 0 || span.Len < 0 || end < span.Offset || end > len(text) {
		return "", false
	}
	return text[span.Offset:end], true
}

func transitionNodeText(text string, node ast.Node) (string, bool) {
	if node == nil {
		return "", false
	}
	return transitionTrimmedSpanText(text, node.Span())
}

func transitionTriggerText(text string, transition *ast.TransitionMember) (string, bool) {
	if transition.Trigger == nil {
		return "", false
	}
	span := transition.Trigger.Span()
	if transition.Via != nil {
		viaText, ok := transitionQualifiedNameText(text, transition.Via)
		if !ok {
			return "", false
		}
		end := transition.Via.NodeSpan.Offset + len(viaText)
		if end < span.Offset {
			return "", false
		}
		span.Len = end - span.Offset
	}
	return transitionTrimmedSpanText(text, span)
}

func transitionEffectText(text string, effects []ast.Node) (string, bool) {
	if len(effects) == 0 {
		return "", false
	}
	start := effects[0].Span().Offset
	end := effects[0].Span().End()
	for _, effect := range effects[1:] {
		span := effect.Span()
		if span.Offset < start || span.End() < end {
			return "", false
		}
		end = span.End()
	}
	if end < start {
		return "", false
	}
	return transitionTrimmedSpanText(text, source.Span{Offset: start, Len: end - start})
}

func transitionTrimmedSpanText(text string, span source.Span) (string, bool) {
	raw, ok := transitionSpanText(text, span)
	if !ok {
		return "", false
	}
	lx := lexer.New(source.New("<transition>", []byte(raw)))
	end := 0
	for {
		token := lx.Next()
		if token.Kind == lexer.EOF {
			break
		}
		if token.IsTrivia() || token.Kind == lexer.RegularComment {
			continue
		}
		end = token.Span.End()
	}
	if end == 0 {
		return "", false
	}
	return raw[:end], true
}

func transitionQualifiedNameText(text string, name *ast.QualifiedName) (string, bool) {
	if name == nil || len(name.Parts) == 0 {
		return "", false
	}
	last := name.Parts[len(name.Parts)-1].Span
	end := last.Offset + last.Len
	if end < last.Offset || end < name.NodeSpan.Offset {
		return "", false
	}
	return transitionSpanText(text, source.Span{
		Offset: name.NodeSpan.Offset,
		Len:    end - name.NodeSpan.Offset,
	})
}

func unwrapMembership(member ast.Node) ast.Node {
	if membership, ok := member.(*ast.Membership); ok && membership != nil {
		return membership.Member
	}
	return member
}
