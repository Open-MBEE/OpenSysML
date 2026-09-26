package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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
	if !ok || transition.Name != op.TransitionName ||
		qualifiedNameText(transition.Source) != op.TransitionSource ||
		qualifiedNameText(transition.Target) != op.TransitionTarget ||
		(transition.Trigger != nil) != (op.Trigger != "") ||
		(transition.Guard != nil) != (op.Guard != "") ||
		transition.HasEffect != (op.Effect != "") {
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

func qualifiedNameText(name *ast.QualifiedName) string {
	if name == nil {
		return ""
	}
	var text strings.Builder
	if name.Global {
		text.WriteString("$::")
	}
	for i, part := range name.Parts {
		if i > 0 {
			if part.Chained {
				text.WriteByte('.')
			} else {
				text.WriteString("::")
			}
		}
		text.WriteString(part.Text)
	}
	return text.String()
}

func unwrapMembership(member ast.Node) ast.Node {
	if membership, ok := member.(*ast.Membership); ok && membership != nil {
		return membership.Member
	}
	return member
}
