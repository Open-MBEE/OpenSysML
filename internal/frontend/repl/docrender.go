package repl

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// renderDocumentUsage is what %render-document accepts: a document's name,
// then optionally the form its graph-shaped diagrams are written in and the
// drawing style the dot form draws them in.
const renderDocumentUsage = "usage: %render-document <name> [mermaid|dot|plantuml [pilot|cameo]]"

// RenderDocumentMarkdown compiles the named document definition, evaluates its
// queries against the session's model, and renders the result as Markdown. A
// document binds its queries' parameters in the model, so the invocation is
// the document's name alone. Its cross-document links point at the files a
// set of the model's documents writes.
func (s *Session) RenderDocumentMarkdown(invocation string, opts docrender.MarkdownOptions) (string, error) {
	defer s.enter()()
	return s.renderDocumentMarkdown(invocation, opts)
}

// RenderDocumentHTML compiles the named document definition, evaluates its
// queries against the session's model, and renders the result as HTML, its
// cross-document links pointing at the files a set of the model's documents writes.
func (s *Session) RenderDocumentHTML(invocation string, opts docrender.HTMLOptions) (string, error) {
	defer s.enter()()
	document, files, err := s.evaluateDocument(invocation, ".html")
	if err != nil {
		return "", err
	}
	opts.Files = files
	return docrender.HTML(document, opts)
}

// EvaluateDocument compiles the named document definition and evaluates its
// queries against the session's model, for a backend rendering the result.
func (s *Session) EvaluateDocument(invocation string) (*docir.Document, error) {
	defer s.enter()()
	document, _, err := s.evaluateDocument(invocation, "")
	return document, err
}

func (s *Session) renderDocumentMarkdown(invocation string, opts docrender.MarkdownOptions) (string, error) {
	document, files, err := s.evaluateDocument(invocation, ".md")
	if err != nil {
		return "", err
	}
	opts.Files = files
	return docrender.Markdown(document, opts)
}

// evaluateDocument compiles the named document and evaluates it against the
// session's model, linked with its siblings so cross-document references
// resolve, and plans the files a set of the model's documents in the form of
// extension writes, none when extension is empty.
func (s *Session) evaluateDocument(invocation, extension string) (*docir.Document, map[string]string, error) {
	fields := splitQueryArgs(strings.TrimSpace(invocation))
	if len(fields) == 0 {
		return nil, nil, fmt.Errorf("a document to render must be named")
	}
	if len(fields) > 1 {
		return nil, nil, fmt.Errorf("a document binds its queries' parameters in the model; unexpected argument %q", fields[1])
	}
	sym, fqn, err := s.lookupSymbol(fields[0])
	if err != nil {
		return nil, nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, nil, fmt.Errorf("runtime init: %w", err)
	}
	idx := s.browseIndex()
	sem, resolver := ctx.Semantics(), ctx.Resolver()
	if !docplan.IsDocumentDefinition(idx, sem, sym) {
		return nil, nil, fmt.Errorf("%s is not a document: one is a part def specializing DocumentQueries::Document", notationName(fqn))
	}
	plan, err := docplan.Compile(idx, sem, resolver, sym)
	if err != nil {
		return nil, nil, err
	}
	var files map[string]string
	if extension != "" {
		var names []string
		for _, sibling := range s.documentSymbols(idx, sem) {
			names = append(names, symbols.FQNOf(sibling))
		}
		if files, err = model.DocumentFiles(names, extension); err != nil {
			return nil, nil, err
		}
	}
	document, err := docir.EvaluateLinked(plan,
		model.SiblingDocumentPlans(idx, sem, resolver, sym),
		s.queryContext(ctx),
		queryexec.Options{},
		s.sessionSourceText())
	return document, files, err
}

// RenderedDocument is one document of a rendered multi-document set, rendered
// in one backend's form. Err is the error that kept the document from
// rendering, whose Content then is a page stating it; nil for a rendered document.
type RenderedDocument struct {
	Name     string
	FileName string
	Content  string
	Err      error
}

// RenderDocumentSetMarkdown compiles every document definition the session's
// model declares, evaluates them together, and renders each as Markdown with
// its deterministic file name, so cross-document references link on disk. A
// document that cannot be rendered is a page stating why, under Err, so the
// links into it land on the reason; the others are rendered whole.
func (s *Session) RenderDocumentSetMarkdown(opts docrender.MarkdownOptions) ([]RenderedDocument, error) {
	return s.renderDocumentSet(".md", func(document *docir.Document, files map[string]string) (string, error) {
		opts.Files = files
		return docrender.Markdown(document, opts)
	})
}

// RenderDocumentSetHTML renders the same set as linked HTML files, each
// referring to the others by their .html file names.
func (s *Session) RenderDocumentSetHTML(opts docrender.HTMLOptions) ([]RenderedDocument, error) {
	return s.renderDocumentSet(".html", func(document *docir.Document, files map[string]string) (string, error) {
		opts.Files = files
		return docrender.HTML(document, opts)
	})
}

// renderDocumentSet evaluates every declared document together and renders
// each with one backend into the file planned for it under extension. A
// document that fails to compile, evaluate or render is rendered as the page
// stating its error, which is the document's Err.
func (s *Session) renderDocumentSet(
	extension string,
	render func(*docir.Document, map[string]string) (string, error),
) ([]RenderedDocument, error) {
	defer s.enter()()
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("runtime init: %w", err)
	}
	idx := s.browseIndex()
	sem, resolver := ctx.Semantics(), ctx.Resolver()
	syms := s.documentSymbols(idx, sem)
	names := make([]string, 0, len(syms))
	for _, sym := range syms {
		names = append(names, symbols.FQNOf(sym))
	}
	files, err := model.DocumentFiles(names, extension)
	if err != nil {
		return nil, err
	}
	failed := map[string]error{}
	titles := map[string]string{}
	plans := make([]*docplan.Plan, 0, len(syms))
	for _, sym := range syms {
		plan, err := docplan.Compile(idx, sem, resolver, sym)
		if err != nil {
			failed[symbols.FQNOf(sym)] = err
			continue
		}
		titles[plan.Name()] = plan.Title()
		plans = append(plans, plan)
	}
	outcomes, err := docir.EvaluateSet(plans,
		s.queryContext(ctx),
		queryexec.Options{},
		s.sessionSourceText())
	if err != nil {
		return nil, err
	}
	documents := map[string]*docir.Document{}
	for _, outcome := range outcomes {
		if outcome.Err != nil {
			failed[outcome.Name] = outcome.Err
			continue
		}
		documents[outcome.Name] = outcome.Document
	}
	out := make([]RenderedDocument, 0, len(names))
	for _, name := range names {
		rendered := RenderedDocument{Name: name, FileName: files[name]}
		if document, ok := documents[name]; ok {
			rendered.Content, rendered.Err = render(document, files)
		} else {
			rendered.Err = failed[name]
		}
		if rendered.Err != nil {
			rendered.Content, err = render(docir.Unrendered(name, titles[name], rendered.Err, plans), files)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, rendered)
	}
	return out, nil
}

// documentSymbols is every document definition the session's model declares,
// in fully-qualified-name order.
func (s *Session) documentSymbols(idx *symbols.Index, sem *semantics.Model) []*symbols.Symbol {
	syms := s.symbolsInLoadOrder(func(scope *symbols.Scope) []*symbols.Symbol {
		return model.DeclaredDocumentDefinitions(idx, sem, scope)
	})
	sort.SliceStable(syms, func(i, j int) bool {
		return symbols.FQNOf(syms[i]) < symbols.FQNOf(syms[j])
	})
	return syms
}

// doRenderDocument carries out %render-document, printing the rendered
// Markdown or reporting a document that could not be rendered. A second
// word names the form its graph-shaped diagrams are written in, a third the
// drawing style.
func (s *Session) doRenderDocument(invocation string) ([]string, bool, error) {
	fields := splitQueryArgs(strings.TrimSpace(invocation))
	if len(fields) == 0 || len(fields) > 3 {
		return []string{renderDocumentUsage}, false, nil
	}
	var opts docrender.MarkdownOptions
	if len(fields) >= 2 {
		opts.DiagramForm = view.Form(fields[1])
		if !slices.Contains(view.DiagramForms(), opts.DiagramForm) {
			return []string{errPrefix + fmt.Sprintf("%q is not a diagram form (%s); a document binds its queries' parameters in the model",
				fields[1], view.FormNames(view.DiagramForms()))}, false, nil
		}
	}
	if len(fields) == 3 {
		style, ok := view.ParseDrawingStyle(fields[2])
		if !ok {
			return []string{errPrefix + (&view.UnknownDrawingStyleError{Name: fields[2]}).Error() + "; " + renderDocumentUsage}, false, nil
		}
		opts.Style = style
	}
	markdown, err := s.renderDocumentMarkdown(fields[0], opts)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	return strings.Split(strings.TrimRight(markdown, "\n"), "\n"), false, nil
}
