package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// addCommentSplice writes `comment [name] [about …] [locale "…"] /* body */`
// (KerML 1.1 §8.2.3.3.2) where a new member of Owner goes; "" is the root.
func (m Model) addCommentSplice(i int, op Operation) (splice, error) {
	owner, scope, err := m.addOwner(op.Owner)
	if err != nil {
		e := err.(*Error)
		e.OperationIndex = i
		return splice{}, e
	}
	if op.DocName != "" {
		if err := checkName(i, op.DocName); err != nil {
			return splice{}, err
		}
		if taken := scope.LookupLocalAll(symbolName(op.DocName)); len(taken) > 0 {
			return splice{}, &Error{
				Failure: FailureMemberNameTaken, OperationIndex: i,
				Message: fmt.Sprintf("%s already declares %q", ownerName(op.Owner), op.DocName),
			}
		}
	}
	header := "comment"
	if op.DocName != "" {
		header += " " + op.DocName
	}
	for j, about := range op.About {
		if _, err := checkFeatureReference(i, "annotated element", about); err != nil {
			return splice{}, err
		}
		if isFeatureChain(about) {
			return splice{}, &Error{
				Failure: FailureInvalidName, OperationIndex: i,
				Message: fmt.Sprintf("annotated element %q is a feature chain, not a qualified name", about),
			}
		}
		if j == 0 {
			header += " about " + about
		} else {
			header += ", " + about
		}
	}
	if op.DocLocale != "" {
		header += " locale " + source.StringText(op.DocLocale)
	}
	comment, err := commentBodyText(i, "comment", op.Doc, m.ownerMemberIndent(owner))
	if err != nil {
		return splice{}, err
	}
	ins := m.memberInsertion(owner, header+" "+comment)
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Owner}, nil
}

// addNoteSplice writes the line note `// text` on its own line directly above
// Target's declaration. A note is lexical trivia (KerML 1.1 §8.2.2.2), no
// element, and delete and move carry it as one of Target's leading comments.
func (m Model) addNoteSplice(i int, op Operation) (splice, error) {
	if strings.ContainsAny(op.Note, "\n\r") {
		return splice{}, &Error{
			Failure: FailureInvalidValue, OperationIndex: i,
			Message: "a note is one line: its text may not contain a line break",
		}
	}
	sym, err := m.target(i, op)
	if err != nil {
		return splice{}, err
	}
	if sym.Decl == nil {
		return splice{}, &Error{
			Failure: FailureUnknownTarget, OperationIndex: i,
			Message: fmt.Sprintf("%q has no declaration to write a note before", op.Target),
		}
	}
	content := m.Source.Bytes()
	start := symbolSpan(sym).Offset
	note := "//"
	if op.Note != "" {
		note += " " + op.Note
	}
	lineStart := start
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}
	if onlyWhitespace(content[lineStart:start]) {
		text := string(content[lineStart:start]) + note + "\n"
		return splice{span: source.Span{Offset: lineStart}, text: text, opIndex: i, target: op.Target}, nil
	}
	indent := ""
	if owner := sym.OwnerScope.Node(); owner != nil && owner != m.Root {
		indent = m.ownerMemberIndent(owner)
	}
	gap := start
	for gap > lineStart && (content[gap-1] == ' ' || content[gap-1] == '\t') {
		gap--
	}
	return splice{
		span:    source.Span{Offset: gap, Len: start - gap},
		text:    "\n" + indent + note + "\n" + indent,
		opIndex: i, target: op.Target,
	}, nil
}

// isFeatureChain reports whether a feature reference joins names with `.`.
func isFeatureChain(ref string) bool {
	lx := lexer.New(source.New("<ref>", []byte(ref)))
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Kind == lexer.Dot {
			return true
		}
	}
	return false
}
