package repl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
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
	out, syntax, err := convert.ConvertTolerant(sessionOrigin, []byte(src), convert.FormatSysML, convert.FormatSysML)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	return append(printWarnings(syntax), notationLines(out)...), false, nil
}

// printElement prints one element and its body: the source its declaration
// spans, with the notes and comments written above it.
func (s *Session) printElement(name string) ([]string, bool, error) {
	sym, fqn, err := s.lookupSymbol(name)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	shown := notationName(fqn)
	if shown == "" {
		shown = name
	}
	var doc *model.Document
	if sym != nil && (sym.DocName == docName || sym.DocName == kermlDocName) {
		doc = s.ws.Document(sym.DocName)
	}
	if doc == nil || sym == nil || sym.Decl == nil {
		// A symbol the library index answered with is declared in a file this
		// session never read, or was restored from an index cache holding no tree.
		return []string{fmt.Sprintf("no notation to print for %s: this session declares it nowhere", shown)}, false, nil
	}
	file := source.New(doc.Name, doc.Content)
	out, syntax, err := convert.SysMLElement(file, declarationSpan(sym))
	if err != nil {
		if errors.Is(err, convert.ErrNoNotation) {
			return []string{fmt.Sprintf("no notation to print for %s: its declaration spans no source", shown)}, false, nil
		}
		return []string{errPrefix + err.Error()}, false, nil
	}
	lines := append(printWarnings(syntax), notationLines(out)...)
	if len(lines) == 0 {
		return []string{fmt.Sprintf("no notation to print for %s: its declaration spans no source", shown)}, false, nil
	}
	return lines, false, nil
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

// printWarnings reports the syntax errors of a printed buffer, in the wording a
// save reports them with: the notation is printed as typed either way.
func printWarnings(syntax *convert.SyntaxError) []string {
	if syntax == nil {
		return nil
	}
	lines := strings.Split("warning: "+syntax.Error(), "\n")
	return append(lines, "warning: the model is printed as typed; fix these and print again")
}

// notationLines splits written notation into prompt lines, dropping the trailing
// blank line the writer ends a document with.
func notationLines(out []byte) []string {
	text := strings.TrimRight(string(out), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
