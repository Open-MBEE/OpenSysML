// Package modeldoc renders a workspace's document definitions, so the workspace
// model itself never links the document evaluator or the runtime it reads objects from.
package modeldoc

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/filename"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// DocumentFiles plans the file each named document is written to when the
// documents are rendered as a set in the form of extension: its stem cut to
// fit, and tagged with a hash of the whole where two would meet letter case
// aside. Every renderer links documents to one another by this plan, so a
// document rendered on its own links to the files a set writes. Two documents
// of one name cannot be told apart.
func DocumentFiles(names []string, extension string) (map[string]string, error) {
	files, err := filename.Plan(names, func(name string, tagged bool) string {
		return filename.Fit(docrender.DocumentFileStem(name), extension, '.', tagged)
	})
	var collision *filename.CollisionError
	if errors.As(err, &collision) {
		if collision.Names[0] == collision.Names[1] {
			return nil, fmt.Errorf("%s names more than one document; rename one so the name is unambiguous", source.QualifiedNameText(collision.Names[0]))
		}
		return nil, fmt.Errorf("%s and %s render to one file name %s; rename one so both files can coexist", source.QualifiedNameText(collision.Names[0]), source.QualifiedNameText(collision.Names[1]), collision.File)
	}
	return files, err
}

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
			if !docplan.IsDocumentDefinition(idx, sem, sym) {
				err = fmt.Errorf("%s is not a document: one is a part def specializing DocumentQueries::Document", fqn)
				return
			}
			var plan *docplan.Plan
			if plan, err = docplan.Compile(idx, sem, resolver, sym); err != nil {
				return
			}
			if opts.Files, err = DocumentFiles(model.DocumentNames(idx, sem), ".md"); err != nil {
				return
			}
			var document *docir.Document
			document, err = docir.EvaluateLinked(plan,
				model.SiblingDocumentPlans(idx, sem, resolver, sym),
				queryexec.Context{Index: idx, Resolver: resolver, Model: sem},
				queryexec.Options{}, r.SourceText())
			if err != nil {
				return
			}
			out, err = docrender.Markdown(document, opts)
		})
		return err
	})
	return out, err
}
