package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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

// setAnnotationValueSplice rewrites the value bound by an annotation usage to
// one of its metadata features. It targets the binding in the usage's body,
// never the metadata definition's own default for that feature.
func (m Model) setAnnotationValueSplice(i int, op Operation) (splice, error) {
	fail := func(reason string) (splice, error) {
		return splice{}, &Error{Failure: FailureUnknownTarget, OperationIndex: i, Message: reason}
	}
	if len(op.About) != 0 {
		for _, about := range op.About {
			if err := checkEnd(i, "annotation about reference", about); err != nil {
				return splice{}, err
			}
		}
	}
	if len(op.About) != 1 {
		return fail("a set-annotation-value operation names exactly one annotated element")
	}
	if err := checkQualifiedReference(i, "metadata type", op.MetadataType); err != nil {
		return splice{}, err
	}
	if err := checkEnd(i, "annotation feature", op.AnnotationFeature); err != nil {
		return splice{}, err
	}
	if err := m.checkExpression(i, "annotation value", op.AnnotationFeature, op.Value); err != nil {
		return splice{}, err
	}
	annotated, err := m.declaredOnceIn(i, op.About[0], "")
	if err != nil {
		return splice{}, err
	}
	_, model := m.resolver()
	named := m.declared(op.MetadataType)
	var found []splice
	for _, metadata := range model.ElementMetadataOf(annotated) {
		if metadata.Doc != m.Source.Name() || !sameMetadataType(metadata.Type, op.MetadataType, named) {
			continue
		}
		if op.MetadataName != "" {
			root := m.Index.DocumentRoot(metadata.Doc)
			var sym *symbols.Symbol
			if root != nil {
				sym = root.DeclaredAt(metadata.Node.Span())
			}
			if sym == nil || sym.Name != op.MetadataName {
				continue
			}
		}
		for _, binding := range metadata.Bindings {
			if binding.Feature != op.AnnotationFeature {
				continue
			}
			sp, err := m.bindingValueSplice(i, op, binding)
			if err != nil {
				return splice{}, err
			}
			found = append(found, sp)
		}
	}
	switch len(found) {
	case 0:
		return fail(fmt.Sprintf("%s carries no %s annotation binding %q in this document",
			op.About[0], op.MetadataType, op.AnnotationFeature))
	case 1:
		return found[0], nil
	default:
		return fail(fmt.Sprintf("%s carries %d %s annotations binding %q; name the annotation usage",
			op.About[0], len(found), op.MetadataType, op.AnnotationFeature))
	}
}

// sameMetadataType reports whether metadata is the type named in the operation,
// by qualified name or by a name unique over the model.
func sameMetadataType(metadata *symbols.Symbol, name string, named []*symbols.Symbol) bool {
	if metadata == nil {
		return false
	}
	if symbols.FQNOf(metadata) == name {
		return true
	}
	for _, sym := range named {
		if symbols.SameElement(sym, metadata) {
			return true
		}
	}
	return false
}

// bindingValueSplice is the byte range a metadata binding's value occupies, or
// the insertion point before its terminator when it binds no value yet.
func (m Model) bindingValueSplice(i int, op Operation, binding semantics.MetadataBinding) (splice, error) {
	target := op.MetadataType + "::" + binding.Feature
	if binding.Value != nil {
		return splice{
			span: m.tokenSpan(binding.Value.Span()), text: op.Value,
			opIndex: i, target: target,
		}, nil
	}
	semi, ok := m.terminator(binding.Node)
	if !ok {
		return splice{}, &Error{
			Failure: FailureNotValued, OperationIndex: i,
			Message: fmt.Sprintf("%s has no value and no terminating ';' to add one before", target),
		}
	}
	text := "= " + op.Value
	if semi.Offset > 0 && !isSpace(m.Source.Bytes()[semi.Offset-1]) {
		text = " " + text
	}
	return splice{
		span: source.Span{Offset: semi.Offset, Len: 0}, text: text,
		opIndex: i, target: target,
	}, nil
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
