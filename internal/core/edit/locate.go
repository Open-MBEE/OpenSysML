package edit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// declared returns the declarations a qualified name, spelled as the notation
// does, names: `'x::y'` is one name and `x::y` two, so each finds its own.
func (m Model) declared(name string) []*symbols.Symbol {
	names, ok := source.QualifiedNameSegments(name)
	if !ok {
		return nil
	}
	joined := strings.Join(names, "::")
	var declaring []*symbols.Symbol
	for _, sym := range m.Index.LookupQualifiedFrom(joined, joined) {
		if sym != nil && slices.Equal(symbols.NameChain(sym), names) {
			declaring = append(declaring, sym)
		}
	}
	return declaring
}

// notationName spells a symbol's qualified name as declared reads it back.
func notationName(sym *symbols.Symbol) string {
	return source.QualifiedNameOf(symbols.NameChain(sym))
}

// target returns the declaration an operation names. Only a declaration of this
// model's own document can be edited: its source is the only one being rewritten.
func (m Model) target(i int, op Operation) (*symbols.Symbol, error) {
	sym, err := m.element(i, op)
	if err != nil {
		return nil, err
	}
	if doc := sym.DocName; doc != m.Source.Name() {
		return nil, &Error{
			Failure:        FailureUnknownTarget,
			OperationIndex: i,
			Message: fmt.Sprintf("%q is declared in %s, not in this model's source",
				op.Target, docLabel(doc)),
		}
	}
	return sym, nil
}

// element is the declaration an operation edits, in whichever document: the one
// Target names, or the one at Declaration in its document when Target is empty.
// A Target stated DeclaredIn a document is the one declared there, so that a
// namesake another document declares does not stand in for it nor make it
// ambiguous.
func (m Model) element(i int, op Operation) (*symbols.Symbol, error) {
	if op.Target != "" || op.Declaration.Len == 0 {
		return m.declaredOnceIn(i, op.Target, op.DeclarationDoc)
	}
	root, err := m.declarationRoot(i, op)
	if err != nil {
		return nil, err
	}
	if sym := root.DeclaredAt(op.Declaration); sym != nil {
		return sym, nil
	}
	return nil, &Error{
		Failure:        FailureUnknownTarget,
		OperationIndex: i,
		Message:        fmt.Sprintf("nothing is declared at %s of this model", m.at(op)),
	}
}

// declarationRoot is the root scope of the document an operation's Declaration
// is a span of; a document the index does not hold is refused.
func (m Model) declarationRoot(i int, op Operation) (*symbols.Scope, error) {
	root := m.Index.DocumentRoot(m.declarationDoc(op))
	if root == nil {
		return nil, &Error{Failure: FailureUnknownTarget, OperationIndex: i,
			Message: fmt.Sprintf("%s is no document of this workspace", op.DeclarationDoc)}
	}
	return root, nil
}

// declarationDoc names the document an operation's Declaration is a span of.
func (m Model) declarationDoc(op Operation) string {
	if op.DeclarationDoc == "" {
		return m.Source.Name()
	}
	return op.DeclarationDoc
}

// label names the element an operation edits for a message and a splice: its
// qualified name, or where it is declared when it is reached by no name.
func (m Model) label(op Operation) string {
	if op.Target != "" || op.Declaration.Len == 0 {
		return op.Target
	}
	return "the element declared at " + m.at(op)
}

// at spells where an operation's Declaration starts, as line:column, qualified
// by its document when that is another than the edited one.
func (m Model) at(op Operation) string {
	doc := m.declarationDoc(op)
	in, ok := m.inDocument(doc)
	if !ok {
		return fmt.Sprintf("byte %d of %s", op.Declaration.Offset, doc)
	}
	pos := in.Source.Lines().PosAt(op.Declaration.Offset)
	if doc != m.Source.Name() {
		return fmt.Sprintf("%d:%d of %s", pos.Line, pos.Col, doc)
	}
	return fmt.Sprintf("%d:%d", pos.Line, pos.Col)
}

// qualified reports whether a qualified name reaches sym: it and every namespace
// declaring it are named, so an `about` can refer to it.
func qualified(sym *symbols.Symbol) bool {
	if sym.Name == "" {
		return false
	}
	for scope := sym.OwnerScope; scope != nil && scope.Owner() != nil; scope = scope.Owner().OwnerScope {
		if scope.Owner().Name == "" {
			return false
		}
	}
	return true
}

// declaredOnceIn is the one declaration name names in doc, or in whichever
// document when doc is empty. A name declared only elsewhere is reported with
// the documents declaring it.
func (m Model) declaredOnceIn(i int, name, doc string) (*symbols.Symbol, error) {
	return oneOf(i, name, doc, m.declared(name))
}

// oneOf is the one of the declarations of name in doc, or in whichever document
// when doc is empty.
func oneOf(i int, name, doc string, declaring []*symbols.Symbol) (*symbols.Symbol, error) {
	if doc != "" {
		var elsewhere []string
		declaring = slices.DeleteFunc(declaring, func(sym *symbols.Symbol) bool {
			if sym.DocName == doc {
				return false
			}
			if label := docLabel(sym.DocName); !slices.Contains(elsewhere, label) {
				elsewhere = append(elsewhere, label)
			}
			return true
		})
		if len(declaring) == 0 && len(elsewhere) > 0 {
			slices.Sort(elsewhere)
			return nil, &Error{Failure: FailureUnknownTarget, OperationIndex: i,
				Message: fmt.Sprintf("%q is declared in %s, not in %s as stated",
					name, strings.Join(elsewhere, " and "), docLabel(doc))}
		}
	}
	switch len(declaring) {
	case 0:
		return nil, &Error{
			Failure:        FailureUnknownTarget,
			OperationIndex: i,
			Message:        fmt.Sprintf("no element named %q in this model", name),
		}
	case 1:
		return declaring[0], nil
	default:
		return nil, &Error{
			Failure:        FailureAmbiguousTarget,
			OperationIndex: i,
			Message: fmt.Sprintf("%q names %d declarations; it does not say which to edit",
				name, len(declaring)),
		}
	}
}

// docLabel names a document for a message, for the library declarations that
// carry no document name of their own.
func docLabel(doc string) string {
	if doc == "" {
		return "the standard library"
	}
	return doc
}

// valueSplice is the byte range a set-value operation rewrites: the expression
// of an existing `= <expr>`, or an insertion before the declaration's `;`.
func (m Model) valueSplice(i int, op Operation, sym *symbols.Symbol) (splice, error) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return splice{}, &Error{
			Failure:        FailureNotValued,
			OperationIndex: i,
			Message: fmt.Sprintf("%s declares no feature that can carry a value (%s)",
				op.Target, sym.Kind),
		}
	}
	if err := m.checkValue(i, op); err != nil {
		return splice{}, err
	}
	if usage.Value != nil {
		return splice{
			span:    m.tokenSpan(usage.Value.Span()),
			text:    op.Value,
			opIndex: i,
			target:  op.Target,
		}, nil
	}
	if usage.ConnectorEnds != nil || usage.FlowEnds != nil {
		return splice{}, &Error{
			Failure:        FailureNotValued,
			OperationIndex: i,
			Message:        fmt.Sprintf("%s connects features rather than carrying a value", op.Target),
		}
	}
	semi, ok := m.terminator(usage)
	if !ok {
		return splice{}, &Error{
			Failure:        FailureNotValued,
			OperationIndex: i,
			Message: fmt.Sprintf("%s has no value and no terminating ';' to add one before"+
				" (a value cannot be added to a declaration with a body)", op.Target),
		}
	}
	text := "= " + op.Value
	if semi.Offset > 0 && !isSpace(m.Source.Bytes()[semi.Offset-1]) {
		text = " " + text
	}
	return splice{
		span:    source.Span{Offset: semi.Offset, Len: 0},
		text:    text,
		opIndex: i,
		target:  op.Target,
	}, nil
}

// tokenSpan narrows a node's span to the bytes its own tokens cover. A span ends
// at the next token's start, so whitespace and comments written after the node
// fall inside it and would be spliced away with it.
func (m Model) tokenSpan(span source.Span) source.Span {
	end := span.Offset
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset >= span.End() {
			break
		}
		if tok.Span.Offset < span.Offset || tok.IsTrivia() || tok.Kind == lexer.RegularComment {
			continue
		}
		if e := tok.Span.End(); e > end && e <= span.End() {
			end = e
		}
	}
	if end <= span.Offset {
		return span
	}
	return source.Span{Offset: span.Offset, Len: end - span.Offset}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// terminator returns the span of the `;` ending a declaration that has no body.
// Only such a declaration can take a value appended to it.
func (m Model) terminator(usage *ast.Usage) (source.Span, bool) {
	if usage.HasBody {
		return source.Span{}, false
	}
	span := usage.Span()
	var last source.Span
	found := false
	lx := lexer.New(m.Source)
	for tok := lx.Next(); tok.Kind != lexer.EOF; tok = lx.Next() {
		if tok.Span.Offset >= span.End() {
			break
		}
		if tok.Span.Offset >= span.Offset && tok.Kind == lexer.Semicolon {
			last, found = tok.Span, true
		}
	}
	return last, found
}

// resolver is a resolver and semantic model set up as the checker's: inherited and
// subsetted members are visible, and a call's overloads are told apart by argument type.
func (m Model) resolver() (*resolve.Resolver, *semantics.Model) {
	r := resolve.New(m.Index)
	sem := semantics.NewModel(r)
	r.SetModel(sem)
	sem.SetArgumentTyper(passes.NewArgumentTyper(r, sem))
	return r, sem
}
