package edit

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// addDocumentationSplice writes a `doc` as the first body member of Target,
// opening a body for a declaration ended by `;`, or rewrites the one
// documentation Target owns when ReplaceDoc is set.
func (m Model) addDocumentationSplice(i int, op Operation) (splice, error) {
	sym, err := m.target(i, op)
	if err != nil {
		return splice{}, err
	}
	owner := sym.Decl
	if sym.Scope == nil || !admitsBody(owner) {
		return splice{}, &Error{
			Failure: FailureOwnerNotNamespace, OperationIndex: i,
			Message: fmt.Sprintf("%q has no body to own documentation", op.Target),
		}
	}
	docs := ownedDocumentation(sym)
	if len(docs) > 0 && !op.ReplaceDoc {
		return splice{}, &Error{
			Failure: FailureMemberNameTaken, OperationIndex: i,
			Message: fmt.Sprintf("%q already has documentation; set replace to rewrite it", op.Target),
		}
	}
	if len(docs) > 1 {
		return splice{}, &Error{
			Failure: FailureAmbiguousTarget, OperationIndex: i,
			Message: fmt.Sprintf("%q has %d documentation elements, so which one to replace is ambiguous",
				op.Target, len(docs)),
		}
	}
	if op.DocName != "" {
		if err := checkName(i, op.DocName); err != nil {
			return splice{}, err
		}
		for _, taken := range sym.Scope.LookupLocalAll(symbolName(op.DocName)) {
			if len(docs) == 0 || taken != docs[0] {
				return splice{}, &Error{
					Failure: FailureMemberNameTaken, OperationIndex: i,
					Message: fmt.Sprintf("%s already declares %q", op.Target, op.DocName),
				}
			}
		}
	}
	if len(docs) == 1 {
		doc := docs[0].Decl.(*ast.Documentation)
		span := source.Span{Offset: doc.Span().Offset, Len: doc.BodySpan.End() - doc.Span().Offset}
		text, err := documentationText(i, op.DocName, op.DocLocale, op.Doc, lineIndent(m.Source.Bytes(), span.Offset))
		if err != nil {
			return splice{}, err
		}
		return splice{span: span, text: text, opIndex: i, target: op.Target}, nil
	}
	indent := m.ownerMemberIndent(owner)
	text, err := documentationText(i, op.DocName, op.DocLocale, op.Doc, indent)
	if err != nil {
		return splice{}, err
	}
	ins := m.firstMemberInsertion(owner, text)
	return splice{span: ins.span, text: ins.text, opIndex: i, target: op.Target}, nil
}

// admitsBody reports whether a declaration is written with a body, or with a
// `;` a body can stand in for: the declarations bodyInfo reads.
func admitsBody(node ast.Node) bool {
	switch node.(type) {
	case *ast.Package, *ast.Namespace, *ast.Definition, *ast.Usage,
		*ast.SubstateMember, *ast.TransitionMember, *ast.SuccessionEdge,
		*ast.Dependency, *ast.MultiplicityDecl, *ast.RelationshipMember:
		return true
	default:
		return false
	}
}

// ownedDocumentation lists the `doc` members sym declares, in order.
func ownedDocumentation(sym *symbols.Symbol) []*symbols.Symbol {
	var docs []*symbols.Symbol
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if _, ok := member.Decl.(*ast.Documentation); ok {
			docs = append(docs, member)
		}
		return true
	})
	return docs
}

// documentationText writes `doc [name] [locale "…"] /* body */`, the comment
// reading back as exactly body (source.CommentText) under indent, the
// indentation of the line the `doc` starts.
func documentationText(i int, name, locale, body, indent string) (string, error) {
	comment, err := commentBodyText(i, "documentation", body, indent)
	if err != nil {
		return "", err
	}
	header := "doc"
	if name != "" {
		header += " " + name
	}
	if locale != "" {
		header += " locale " + source.StringText(locale)
	}
	return header + " " + comment, nil
}

// commentBodyText is the REGULAR_COMMENT whose body is exactly body. `*/` ends
// the token (KerML 1.1 §8.2.2.2 COMMENT_LINE_TEXT excludes it), and a `\r` is
// not a line terminator the body reads back, so neither can be written.
func commentBodyText(i int, what, body, indent string) (string, error) {
	comment, ok := source.CommentText(body, indent)
	if ok {
		return comment, nil
	}
	reason := "contains a carriage return; write line breaks as \"\\n\""
	if strings.Contains(body, "*/") {
		reason = "contains \"*/\", which would close its comment"
	}
	return "", &Error{Failure: FailureInvalidValue, OperationIndex: i, Message: what + " body " + reason}
}

// firstMemberInsertion places text ahead of owner's first body member, or where
// memberInsertion puts it when the body is empty or not yet opened.
func (m Model) firstMemberInsertion(owner ast.Node, text string) insertion {
	if body, hasBody := bodyInfo(owner); hasBody {
		if first, ok := m.firstBodyToken(body); ok {
			return m.memberInsertionBefore(first, text)
		}
	}
	return m.memberInsertion(owner, text)
}

// firstBodyToken is the offset of the first token inside the braces closing
// span, or false for an empty body.
func (m Model) firstBodyToken(span source.Span) (int, bool) {
	var opens []int
	var tokens []lexer.Token
	open := -1
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF && tok.Span.Offset < span.End(); tok = lx.Next() {
		if tok.Span.Offset < span.Offset || tok.IsTrivia() {
			continue
		}
		tokens = append(tokens, tok)
		switch tok.Kind {
		case lexer.LBrace:
			opens = append(opens, len(tokens)-1)
		case lexer.RBrace:
			if len(opens) > 0 {
				open = opens[len(opens)-1]
				opens = opens[:len(opens)-1]
			}
		}
	}
	if open < 0 || open+1 >= len(tokens) || tokens[open+1].Kind == lexer.RBrace {
		return 0, false
	}
	return tokens[open+1].Span.Offset, true
}
