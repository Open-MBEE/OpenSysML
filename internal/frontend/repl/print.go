package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// errPrefix opens the line a command writes when it fails.
const errPrefix = "error: "

// doPrint prints the session's model as SysML notation, whole when name is
// empty and one element of it otherwise. It is a read of the session: nothing is
// materialized, no debugging session is disturbed, and the buffer is unchanged,
// so what is printed can be typed back in.
func (s *Session) doPrint(name string) ([]string, bool, error) {
	if strings.TrimSpace(name) == "" {
		return s.printSession()
	}
	return s.printElement(name)
}

// printSession prints the whole buffer through the writer a `.sysml` save
// writes with, so the prompt shows what a save would hold. RDF is another
// format, and none of it is reported here.
func (s *Session) printSession() ([]string, bool, error) {
	// The text as typed, not the analyzed buffer, for the reason `%save` uses it:
	// work the parser could not read is masked out of that buffer.
	src := s.text()
	if strings.TrimSpace(src) == "" {
		return []string{"nothing to print: the session is empty"}, false, nil
	}
	return replext.Notation().Print(sessionOrigin, []byte(src)), false, nil
}

// printElement prints one element and its body: the source its declaration
// spans, with the notes and comments written above it.
func (s *Session) printElement(name string) ([]string, bool, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	if sym != nil && sym.Recorded() {
		// The body is printed from the tree: a document held as its record is
		// hydrated, and the name looked up again among the tree-backed symbols.
		if err := s.ws.Hydrate(sym.DocName); err != nil {
			return []string{errPrefix + err.Error()}, false, nil
		}
		if sym, fqn, err = s.lookupSymbol(name); err != nil {
			return []string{errPrefix + err.Error()}, false, nil
		}
	}
	shown := notationName(fqn)
	if shown == "" {
		shown = name
	}
	var doc *model.Document
	if sym != nil {
		doc = s.ws.Document(sym.DocName)
	}
	if doc == nil || sym == nil || sym.Decl == nil {
		// A symbol the library index answered with is declared in a file this
		// session never read, or was restored from an index cache holding no tree.
		return []string{fmt.Sprintf("no notation to print for %s: this session declares it nowhere", shown)}, false, nil
	}
	file := sourceForKind(doc.Name, doc.Content, doc.Kind())
	return replext.Notation().PrintElement(file, declarationSpan(sym), shown), false, nil
}

// declarationSpan is the source one element occupies: its declaration together
// with the notes and comments written above it, which belong to what is printed.
func declarationSpan(sym *symbols.Symbol) source.Span {
	span := sym.DeclSpan
	if sym.Decl != nil {
		span = sym.Decl.Span()
	}
	start := span.Offset
	for _, tr := range sym.LeadingTrivia {
		if tr.Span.Offset >= span.Offset {
			continue
		}
		if tr.Span.Offset < start {
			start = tr.Span.Offset
		}
	}
	return source.Span{Offset: start, Len: span.End() - start}
}
