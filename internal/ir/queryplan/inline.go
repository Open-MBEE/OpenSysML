package queryplan

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// InlineDocument names the document and definition an inline query body is
// compiled as; no file is called this, so an origin in it is in the inline text.
const InlineDocument = "<inline query>"

const queryLibrary = "DocumentQueries"

// inlinePrefix wraps an inline body as the result expression of a query
// definition of its own, the query library imported; the body starts after it.
var inlinePrefix = "private import " + queryLibrary + "::*;\n" +
	"calc def '" + InlineDocument + "' :> " + queryBaseFQN + " {\n"

const inlineSuffix = "\n}\n"

// Inline is an inline query body, indexed as a query definition of its own
// beside the model's documents, so Compile plans it as one the model declares.
type Inline struct {
	// Index holds the model's documents (the same symbols) and the inline
	// definition; the model's own index is untouched.
	Index *symbols.Index
	// Entry is the inline definition, what Compile takes as its entry.
	Entry *symbols.Symbol
	text  string
	lines *source.LineIndex
}

// InlineError is a fault located within an inline query's text: a parse error,
// or a planning or execution error (Err) whose origin is in the text.
type InlineError struct {
	Message string
	// Pos is the 1-based line and column within the inline text.
	Pos source.Pos
	// Span is the faulty text's span, offsets within the inline text.
	Span source.Span
	// Text is the inline text, so a report can show the line under the fault.
	Text string
	// Err is the planning or execution error located, nil for a parse error.
	Err error
}

func (e *InlineError) Error() string {
	return fmt.Sprintf("inline query %d:%d: %s", e.Pos.Line, e.Pos.Col, e.Message)
}

// Unwrap exposes the located error, so its kind still decides how it is reported.
func (e *InlineError) Unwrap() error { return e.Err }

// Line is the line of the inline text the fault is on and the byte offset of
// the fault within it.
func (e *InlineError) Line() (string, int) {
	start := e.lines().OffsetAt(source.Pos{Line: e.Pos.Line, Col: 1})
	if start < 0 || start > len(e.Text) {
		return "", 0
	}
	rest := e.Text[start:]
	if end := strings.IndexByte(rest, '\n'); end >= 0 {
		rest = rest[:end]
	}
	return rest, e.Pos.Col - 1
}

func (e *InlineError) lines() *source.LineIndex {
	return source.NewLineIndex([]byte(e.Text))
}

// ParseInline parses text as the result expression of a query definition and
// indexes it beside the model's documents, leaving index as it is.
func ParseInline(index *symbols.Index, text string) (*Inline, error) {
	if index == nil {
		return nil, &Error{Kind: ErrorInvalidContext}
	}
	lines := source.NewLineIndex([]byte(text))
	if strings.TrimSpace(text) == "" {
		return nil, &InlineError{Message: "an inline query is a query expression, and none was written", Pos: source.Pos{Line: 1, Col: 1}, Text: text}
	}
	p := parser.New(source.New(InlineDocument, []byte(inlinePrefix+text+inlineSuffix)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		d := p.Diagnostics[0]
		return nil, inlineError(text, lines, d.Message, inlineSpan(text, d.Span), nil)
	}
	if !inlineShape(root) {
		return nil, inlineError(text, lines, "an inline query is one query expression", source.Span{Offset: 0, Len: len(text)}, nil)
	}
	overlay := inlineIndex(index)
	overlay.AddDocumentWithKind(InlineDocument, root, source.KindSysML)
	overlay.ExpandWildcardImports()
	entry, ok := overlay.DocumentRoot(InlineDocument).LookupLocal(InlineDocument)
	if !ok || entry == nil {
		return nil, inlineError(text, lines, "an inline query is one query expression", source.Span{Offset: 0, Len: len(text)}, nil)
	}
	return &Inline{Index: overlay, Entry: entry, text: text, lines: lines}, nil
}

// inlineShape reports whether the wrapped text parsed as the import and one
// definition, so text closing the definition early to declare more is refused.
func inlineShape(root *ast.RootNamespace) bool {
	if root == nil || len(root.Members) != 2 {
		return false
	}
	member := root.Members[1]
	if membership, ok := member.(*ast.Membership); ok {
		member = membership.Member
	}
	_, ok := member.(*ast.Definition)
	return ok
}

// inlineIndex is a writable index holding what index holds, the model's
// documents sharing index's scope trees so an element is one symbol in both.
func inlineIndex(index *symbols.Index) *symbols.Index {
	base := index.Base()
	switch {
	case base != nil:
		overlay := symbols.NewOverlay(base)
		for _, name := range base.Documents() {
			if index.DocumentRoot(name) == nil {
				overlay.RemoveDocument(name)
			}
		}
		for _, name := range index.Documents() {
			if base.DocumentRoot(name) == nil {
				shareDocument(overlay, index, name)
			}
		}
		return overlay
	case index.Frozen():
		return symbols.NewOverlay(index)
	default:
		overlay := symbols.NewIndex()
		for _, name := range index.Documents() {
			shareDocument(overlay, index, name)
		}
		return overlay
	}
}

// shareDocument adds index's named document to overlay with the scope tree
// index built for it, keeping its language and library standing.
func shareDocument(overlay, index *symbols.Index, name string) {
	overlay.AddDocumentScopeWithKind(name, nil, index.DocumentRoot(name), index.DocumentKind(name))
	if doc := index.LibraryDocumentOf(name); doc.Tier.Library() {
		overlay.MarkLibraryDocument(name, doc)
	}
}

// inlineSpan maps a span of the wrapped text onto the inline text, clamped to
// it: a fault reported at the wrapper's closing brace is at the end.
func inlineSpan(text string, span source.Span) source.Span {
	offset := min(max(span.Offset-len(inlinePrefix), 0), len(text))
	return source.Span{Offset: offset, Len: min(max(span.Len, 0), len(text)-offset)}
}

func inlineError(text string, lines *source.LineIndex, message string, span source.Span, err error) *InlineError {
	return &InlineError{Message: message, Pos: lines.PosAt(span.Offset), Span: span, Text: text, Err: err}
}

// Position locates an origin within the inline text, false for an origin elsewhere.
func (q *Inline) Position(origin symbols.Origin) (source.Pos, bool) {
	if q == nil || origin.Doc != InlineDocument {
		return source.Pos{}, false
	}
	return q.lines.PosAt(inlineSpan(q.text, origin.Span).Offset), true
}

// Locate wraps err, reported at origin, as an InlineError when the origin is in
// the inline text; any other error is returned as it is.
func (q *Inline) Locate(err error, origin symbols.Origin) error {
	if err == nil || q == nil || origin.Doc != InlineDocument {
		return err
	}
	return inlineError(q.text, q.lines, err.Error(), inlineSpan(q.text, origin.Span), err)
}

// Text is the inline text as written.
func (q *Inline) Text() string {
	if q == nil {
		return ""
	}
	return q.text
}
