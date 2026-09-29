package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func (m Model) addVerifySplice(i int, op Operation) (splice, error) {
	if m.Source.Kind() != source.KindSysML {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: fmt.Sprintf("verify is not legal in %s source %q", m.Source.Kind(), m.Source.Name()),
		}
	}
	if err := checkEnd(i, "requirement", op.Requirement); err != nil {
		return splice{}, err
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if ownerSym := ownerScope.Owner(); ownerSym != nil && passes.IsObjectiveUsage(ownerSym.Decl) {
		if ownerSym.OwnerScope == nil || !passes.IsVerificationCase(ownerSym.OwnerScope.Owner()) {
			return splice{}, verifyOwnerRefusal(i)
		}
		ins := m.memberInsertion(owner, "verify "+op.Requirement+";")
		return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
	}
	caseSym := ownerScope.Owner()
	if !passes.IsVerificationCase(caseSym) {
		return splice{}, verifyOwnerRefusal(i)
	}
	var objectives []ast.Node
	for _, member := range ast.DeclMembers(owner) {
		if passes.IsObjectiveUsage(unwrapMembership(member)) {
			objectives = append(objectives, member)
		}
	}
	switch len(objectives) {
	case 1:
		ins := m.memberInsertion(unwrapMembership(objectives[0]), "verify "+op.Requirement+";")
		return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
	case 0:
		_, sem := m.resolver()
		_, inherited := sem.ObjectivesOf(caseSym)
		for _, objective := range inherited {
			if objective != nil && !m.Index.IsLibraryDocument(objective.DocName) {
				return splice{}, &Error{
					Failure: FailureIllegalKind, OperationIndex: i,
					Message: "the verification case inherits an objective; add that objective explicitly before adding a verification",
				}
			}
		}
	}
	if len(objectives) > 1 {
		return splice{}, &Error{
			Failure: FailureIllegalKind, OperationIndex: i,
			Message: "the verification case has multiple objectives; name an objective explicitly",
		}
	}
	memberIndent := m.ownerMemberIndent(owner)
	ownerIndent := lineIndent(m.Source.Bytes(), owner.Span().Offset)
	indentUnit := strings.TrimPrefix(memberIndent, ownerIndent)
	if indentUnit == "" {
		indentUnit = "\t"
		if !strings.Contains(string(m.Source.Bytes()), "\t") {
			indentUnit = "    "
		}
	}
	text := "objective {\n" + memberIndent + indentUnit + "verify " + op.Requirement +
		";\n" + memberIndent + "}"
	ins := m.memberInsertion(owner, text)
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

func verifyOwnerRefusal(i int) error {
	return &Error{
		Failure: FailureIllegalKind, OperationIndex: i,
		Message: "a verify is only admitted in the objective of a verification case",
	}
}
