package edit

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func (m Model) addSatisfySplice(i int, op Operation) (splice, error) {
	if m.Source.Kind() != source.KindSysML {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("satisfy is not legal in %s source %q", m.Source.Kind(), m.Source.Name()),
		}
	}
	if err := checkEnd(i, "requirement", op.Requirement); err != nil {
		return splice{}, err
	}
	if op.SatisfyingFeature != "" {
		if err := checkEnd(i, "satisfying feature", op.SatisfyingFeature); err != nil {
			return splice{}, err
		}
	}
	owner, _, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if !parser.BodyAdmitsBehaviorUsage(owner) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("satisfy is not admitted in the body of %s", ownerName(op.Owner)),
		}
	}
	text := ""
	if op.Asserted {
		text += "assert "
	}
	if op.Negated {
		text += "not "
	}
	text += "satisfy " + op.Requirement
	if op.SatisfyingFeature != "" {
		text += " by " + op.SatisfyingFeature
	}
	ins := m.memberInsertion(owner, text+";")
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

func (m Model) addRequirementConstraintSplice(i int, op Operation) (splice, error) {
	if m.Source.Kind() != source.KindSysML {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("requirement constraints are not legal in %s source %q",
				m.Source.Kind(), m.Source.Name()),
		}
	}
	if op.ConstraintKind != "require" && op.ConstraintKind != "assume" {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("constraint kind %q is not require or assume", op.ConstraintKind),
		}
	}
	if op.Expression == "" {
		return splice{}, &Error{
			Failure: FailureInvalidValue, OperationIndex: i,
			Message: "requirement constraint expression is empty",
		}
	}
	if err := m.checkExpression(i, "expression", op.Owner, op.Expression); err != nil {
		return splice{}, err
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if !parser.BodyIsRequirement(owner) {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("requirement constraints are only admitted in a requirement body, which %s does not open",
				ownerName(op.Owner)),
		}
	}
	if op.ConstraintName != "" {
		if err := checkName(i, op.ConstraintName); err != nil {
			e := err.(*Error)
			e.Message = fmt.Sprintf("constraint name %q is not an identifier", op.ConstraintName)
			return splice{}, e
		}
		if ownerScope != nil && len(ownerScope.LookupLocalAll(op.ConstraintName)) > 0 {
			return splice{}, &Error{
				Failure: FailureMemberNameTaken, OperationIndex: i,
				Message: fmt.Sprintf("%s already declares %q", op.Owner, op.ConstraintName),
			}
		}
	}
	text := op.ConstraintKind + " constraint"
	if op.ConstraintName != "" {
		text += " " + op.ConstraintName
	}
	text += " { " + op.Expression + " }"
	ins := m.memberInsertion(owner, text)
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}
