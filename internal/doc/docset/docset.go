// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

// Package docset renders one document of a model's set end to end — plan, siblings,
// the files a set writes, a backend's text — for the service, engine and workspace alike.
package docset

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/filename"
)

// Definitions lists the document definitions declared in scope, outermost first.
func Definitions(index *symbols.Index, sem *semantics.Model, scope *symbols.Scope) []*symbols.Symbol {
	var out []*symbols.Symbol
	scope.WalkDeclarations(func(sym *symbols.Symbol) {
		if docplan.IsDocumentDefinition(index, sem, sym) {
			out = append(out, sym)
		}
	})
	return out
}

// Names lists the qualified names of every document definition the model's
// workspace documents declare, sorted.
func Names(index *symbols.Index, sem *semantics.Model) []string {
	var names []string
	for _, doc := range index.WorkspaceDocuments() {
		for _, sym := range Definitions(index, sem, index.DocumentRoot(doc)) {
			names = append(names, symbols.FQNOf(sym))
		}
	}
	sort.Strings(names)
	return names
}

// SiblingPlans compiles every other document definition the workspace documents declare,
// skipping any that does not compile, so a single render emits the anchors siblings link to.
func SiblingPlans(index *symbols.Index, sem *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) []*docplan.Plan {
	var plans []*docplan.Plan
	for _, doc := range index.WorkspaceDocuments() {
		for _, sym := range Definitions(index, sem, index.DocumentRoot(doc)) {
			if sym == entry {
				continue
			}
			plan, err := docplan.Compile(index, sem, resolver, sym)
			if err != nil {
				continue
			}
			plans = append(plans, plan)
		}
	}
	return plans
}

// Files plans the file a set rendered in the form of extension writes each named document to:
// its stem cut to fit, tagged where two would meet letter case aside; two of one name cannot be told apart.
func Files(names []string, extension string) (map[string]string, error) {
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

// Form is the text a document definition is rendered as: what a RenderDocument
// request names in form, on every transport that serves one.
type Form string

const (
	FormMarkdown Form = "markdown"
	FormHTML     Form = "html"
	FormAsciiDoc Form = "asciidoc"
)

// Forms lists every form Render renders, in the order a refusal names them.
// PDF is not one: it drives converters only the sysml command runs.
func Forms() []Form { return []Form{FormMarkdown, FormHTML, FormAsciiDoc} }

// extension is the file extension a set rendered in the form writes.
func (f Form) extension() string {
	switch f {
	case FormHTML:
		return ".html"
	case FormAsciiDoc:
		return ".adoc"
	default:
		return ".md"
	}
}

// UnknownFormError refuses a form Render does not render, naming the ones it does.
type UnknownFormError struct{ Form string }

func (e *UnknownFormError) Error() string {
	forms := Forms()
	names := make([]string, len(forms))
	for i, form := range forms {
		names[i] = string(form)
	}
	return fmt.Sprintf("form %q is not one a document is rendered in: %s or %s",
		e.Form, strings.Join(names[:len(names)-1], ", "), names[len(names)-1])
}

// NotDocumentError refuses an element that is not a document definition.
type NotDocumentError struct{ Name string }

func (e *NotDocumentError) Error() string {
	return fmt.Sprintf("%s is not a document: one is a part def specializing DocumentQueries::Document", e.Name)
}

// Options is how Render writes a document.
type Options struct {
	// Form is the form the text is written in; empty renders Markdown.
	Form Form
	// WithoutGraphviz records that no Graphviz draws DOT for this render, so a
	// positioned diagram is written as Mermaid, the output stating so.
	WithoutGraphviz bool
}

// Rendered is one document's text in one form and the file a set rendered in
// that form writes it to.
type Rendered struct {
	Text     string
	FileName string
}

// Evaluate compiles sym's document definition and evaluates its queries in
// qctx, over the objects qctx holds: the document tree every form is written from.
func Evaluate(qctx queryexec.Context, sym *symbols.Symbol, text view.SourceText) (*docir.Document, error) {
	if !docplan.IsDocumentDefinition(qctx.Index, qctx.Model, sym) {
		return nil, &NotDocumentError{Name: symbols.FQNOf(sym)}
	}
	plan, err := docplan.Compile(qctx.Index, qctx.Model, qctx.Resolver, sym)
	if err != nil {
		return nil, err
	}
	return docir.EvaluateLinked(plan,
		SiblingPlans(qctx.Index, qctx.Model, qctx.Resolver, sym),
		qctx, queryexec.Options{}, text)
}

// Render evaluates sym's document and writes it in opts.Form, linking the
// model's other documents by the files a set in that form writes them to.
func Render(qctx queryexec.Context, sym *symbols.Symbol, text view.SourceText, opts Options) (Rendered, error) {
	form := opts.Form
	if form == "" {
		form = FormMarkdown
	}
	if !slices.Contains(Forms(), form) {
		return Rendered{}, &UnknownFormError{Form: string(opts.Form)}
	}
	document, err := Evaluate(qctx, sym, text)
	if err != nil {
		return Rendered{}, err
	}
	files, err := Files(Names(qctx.Index, qctx.Model), form.extension())
	if err != nil {
		return Rendered{}, err
	}
	rendered := Rendered{FileName: files[symbols.FQNOf(sym)]}
	markdown := docrender.MarkdownOptions{Files: files, WithoutGraphviz: opts.WithoutGraphviz}
	switch form {
	case FormHTML:
		rendered.Text, err = docrender.HTML(document, docrender.HTMLOptions{Files: files, WithoutGraphviz: opts.WithoutGraphviz})
	case FormAsciiDoc:
		rendered.Text, err = docrender.AsciiDoc(document, docrender.AsciiDocOptions(markdown))
	default:
		rendered.Text, err = docrender.Markdown(document, markdown)
	}
	if err != nil {
		return Rendered{}, err
	}
	return rendered, nil
}

// Fault is who a document failure is charged to: the status a transport
// reports the failure under.
type Fault int

const (
	// FaultInternal is the pipeline's own failure, or one it does not classify.
	FaultInternal Fault = iota
	// FaultArgument is the caller's: a form, name or binding the request got wrong.
	FaultArgument
	// FaultModel is a fault in the model's own document and query definitions.
	FaultModel
	// FaultBudget is a budget or the held-objects bound the evaluation exhausted.
	FaultBudget
)

// FaultOf classifies err: a wrong binding, form or non-document is the caller's, an exhausted
// budget a resource limit, a missing library or context the implementation's, the rest the model's.
func FaultOf(err error) Fault {
	var notDocument *NotDocumentError
	var unknownForm *UnknownFormError
	var planErr *queryplan.Error
	var execErr *queryexec.Error
	var docPlanErr *docplan.Error
	switch {
	case errors.As(err, &notDocument), errors.As(err, &unknownForm):
		return FaultArgument
	case errors.Is(err, runtime.ErrInstanceLimitExceeded):
		return FaultBudget
	case errors.As(err, &planErr):
		switch planErr.Kind {
		case queryplan.ErrorNotQueryDefinition:
			return FaultArgument
		case queryplan.ErrorLibraryUnavailable, queryplan.ErrorInvalidContext:
			return FaultInternal
		default:
			return FaultModel
		}
	case errors.As(err, &execErr):
		switch execErr.Kind {
		case queryexec.ErrorUnknownBinding, queryexec.ErrorMissingBinding,
			queryexec.ErrorBindingType, queryexec.ErrorBindingMultiplicity:
			return FaultArgument
		case queryexec.ErrorVisitBudget, queryexec.ErrorInvocationBudget,
			queryexec.ErrorInvocationDepth:
			return FaultBudget
		case queryexec.ErrorInvalidContext:
			return FaultInternal
		default:
			return FaultModel
		}
	case errors.As(err, &docPlanErr):
		switch docPlanErr.Kind {
		case docplan.ErrorNotDocumentDefinition:
			return FaultArgument
		case docplan.ErrorLibraryUnavailable, docplan.ErrorInvalidContext:
			return FaultInternal
		default:
			return FaultModel
		}
	default:
		// A docir.Error not wrapping a query's failure is the evaluator's own.
		return FaultInternal
	}
}

// OriginOf is the source declaration the pipeline names behind err, unlocated
// when it names none.
func OriginOf(err error) symbols.Origin {
	var planErr *queryplan.Error
	var execErr *queryexec.Error
	var docPlanErr *docplan.Error
	var docIRErr *docir.Error
	switch {
	case errors.As(err, &planErr):
		return planErr.Origin
	case errors.As(err, &execErr):
		return execErr.Origin
	case errors.As(err, &docPlanErr):
		return docPlanErr.Origin
	case errors.As(err, &docIRErr):
		return docIRErr.Origin
	default:
		return symbols.Origin{}
	}
}

// QueryOutcomeOf is the outcome record of the query failure behind err, nil
// when no query failed.
func QueryOutcomeOf(err error) *queryexec.Outcome {
	var execErr *queryexec.Error
	if errors.As(err, &execErr) {
		return execErr.Outcome
	}
	return nil
}
