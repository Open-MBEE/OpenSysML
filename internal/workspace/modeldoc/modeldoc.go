// Package modeldoc renders a workspace's document definitions, so the workspace
// model itself never links the document evaluator or the runtime it reads objects from.
package modeldoc

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docset"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// RenderDocumentMarkdown compiles the named document definition, evaluates its
// queries against the workspace model, and renders the result as Markdown,
// linking the other documents by the files a Markdown set writes them to.
func RenderDocumentMarkdown(ws *model.Workspace, fqn string, opts docrender.MarkdownOptions) (string, error) {
	var out string
	err := ws.Read(func(r *model.Reading) error {
		idx := r.Index()
		matches := symbols.PreferDeclared(idx.LookupQualified(fqn))
		if len(matches) == 0 {
			return fmt.Errorf("no element named %s", fqn)
		}
		if len(matches) > 1 {
			return fmt.Errorf("%s names %d elements; rename one so the name is unambiguous", fqn, len(matches))
		}
		sym := matches[0]
		var err error
		r.Query(sym.DocName, func(resolver *resolve.Resolver, sem *semantics.Model) {
			lineIndexes := r.LineIndexes()
			var document *docir.Document
			document, err = docset.Evaluate(queryexec.Context{Index: idx, Resolver: resolver, Model: sem, LineIndex: func(doc string) *source.LineIndex {
				return lineIndexes[doc]
			}, ElementID: elementIDs(idx, r)}, sym, r.SourceText())
			if err != nil {
				return
			}
			if opts.Files, err = docset.Files(docset.Names(idx, sem), ".md"); err != nil {
				return
			}
			out, err = docrender.Markdown(document, opts)
		})
		return err
	})
	return out, err
}

// elementIDs reads the elementId each element of the workspace is written
// with, through the registered converter; nil when the binary links none.
func elementIDs(idx *symbols.Index, r *model.Reading) func(*symbols.Symbol) (string, error) {
	docs := r.Documents()
	documents := make([]queryexec.ElementIDDocument, 0, len(docs))
	for _, doc := range docs {
		documents = append(documents, queryexec.ElementIDDocument{File: doc.SourceFile(), Root: doc.AST, ParseDiags: doc.ParseDiagnostics})
	}
	read := queryexec.OpenElementIDs(documents)
	if read == nil {
		return nil
	}
	return func(sym *symbols.Symbol) (string, error) {
		return queryexec.ElementIDOf(read, idx, sym)
	}
}
