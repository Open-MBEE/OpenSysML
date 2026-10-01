package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// MetadataValue is one feature binding written in a metadata body.
type MetadataValue struct {
	Feature string
	Value   string
}

func (m Model) addMetadataSplice(i int, op Operation) (splice, error) {
	if err := checkQualifiedReference(i, "metadata type", op.MetadataType); err != nil {
		return splice{}, err
	}
	if op.MetadataName != "" {
		if err := checkName(i, op.MetadataName); err != nil {
			return splice{}, err
		}
	}
	for _, about := range op.About {
		if err := checkEnd(i, "metadata about reference", about); err != nil {
			return splice{}, err
		}
	}
	seen := make(map[string]bool, len(op.MetadataValues))
	for _, binding := range op.MetadataValues {
		if err := checkEnd(i, "metadata feature", binding.Feature); err != nil {
			return splice{}, err
		}
		if seen[binding.Feature] {
			return splice{}, &Error{
				Failure: FailureInvalidValue, OperationIndex: i,
				Message: fmt.Sprintf("metadata feature %q is bound more than once", binding.Feature),
			}
		}
		seen[binding.Feature] = true
		if err := m.checkExpression(i, "metadata value", op.Owner, binding.Value); err != nil {
			return splice{}, err
		}
	}
	owner, ownerScope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if op.MetadataName != "" && nameTaken(ownerScope, op.MetadataName) {
		return splice{}, &Error{
			Failure: FailureMemberNameTaken, OperationIndex: i,
			Message: fmt.Sprintf("%s already declares %q", op.Owner, op.MetadataName),
		}
	}
	text := "metadata "
	if op.Shorthand {
		text = "@"
	}
	if op.MetadataName != "" {
		text += op.MetadataName + " : "
	}
	text += op.MetadataType
	if len(op.About) > 0 {
		text += " about " + strings.Join(op.About, ", ")
	}
	if len(op.MetadataValues) == 0 {
		text += ";"
	} else {
		text += " " + writeBindings(op.MetadataValues)
	}
	ins := m.memberInsertion(owner, text)
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

func checkQualifiedReference(i int, role, ref string) error {
	if err := checkEnd(i, role, ref); err != nil {
		return err
	}
	lx := lexer.New(source.New("<ref>", []byte(ref)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind == lexer.Dot {
			return &Error{
				Failure: FailureInvalidName, OperationIndex: i,
				Message: fmt.Sprintf("%s %q is not a qualified name", role, ref),
			}
		}
	}
	return nil
}
