package repl

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// renderDocumentUsage is what %render-document accepts: a document's name,
// then optionally the form its graph-shaped diagrams are written in.
const renderDocumentUsage = "usage: %render-document <name> [mermaid|dot]"

// RenderDocumentMarkdown compiles the named document definition, evaluates its
// queries against the session's model, and renders the result as Markdown. A
// document binds its queries' parameters in the model, so the invocation is
// the document's name alone.
func (s *Session) RenderDocumentMarkdown(invocation string, opts docrender.MarkdownOptions) (string, error) {
	defer s.enter()()
	return s.renderDocumentMarkdown(invocation, opts)
}

// RenderDocumentHTML compiles the named document definition, evaluates its
// queries against the session's model, and renders the result as HTML.
func (s *Session) RenderDocumentHTML(invocation string, opts docrender.HTMLOptions) (string, error) {
	defer s.enter()()
	document, err := s.evaluateDocument(invocation)
	if err != nil {
		return "", err
	}
	return docrender.HTML(document, opts)
}

func (s *Session) renderDocumentMarkdown(invocation string, opts docrender.MarkdownOptions) (string, error) {
	document, err := s.evaluateDocument(invocation)
	if err != nil {
		return "", err
	}
	return docrender.Markdown(document, opts)
}

// evaluateDocument compiles the named document and evaluates it against the
// session's model, linked with its siblings so cross-document references
// resolve.
func (s *Session) evaluateDocument(invocation string) (*docir.Document, error) {
	fields := splitQueryArgs(strings.TrimSpace(invocation))
	if len(fields) == 0 {
		return nil, fmt.Errorf("a document to render must be named")
	}
	if len(fields) > 1 {
		return nil, fmt.Errorf("a document binds its queries' parameters in the model; unexpected argument %q", fields[1])
	}
	sym, fqn, err := s.lookupSymbol(fields[0])
	if err != nil {
		return nil, err
	}
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("runtime init: %w", err)
	}
	idx := s.browseIndex()
	sem, resolver := ctx.Semantics(), ctx.Resolver()
	if !docplan.IsDocumentDefinition(idx, sem, sym) {
		return nil, fmt.Errorf("%s is not a document: one is a part def specializing DocumentQueries::Document", notationName(fqn))
	}
	plan, err := docplan.Compile(idx, sem, resolver, sym)
	if err != nil {
		return nil, err
	}
	return docir.EvaluateLinked(plan,
		model.SiblingDocumentPlans(idx, sem, resolver, sym),
		queryexec.Context{Index: idx, Resolver: resolver, Model: sem},
		queryexec.Options{},
		s.sessionSourceText())
}

// RenderedDocument is one document of a rendered multi-document set, rendered
// in one backend's form.
type RenderedDocument struct {
	Name     string
	FileName string
	Content  string
}

// RenderDocumentSetMarkdown compiles every document definition the session's
// model declares, evaluates them together, and renders each as Markdown with
// its deterministic file name, so cross-document references link on disk.
func (s *Session) RenderDocumentSetMarkdown(opts docrender.MarkdownOptions) ([]RenderedDocument, error) {
	return s.renderDocumentSet(docrender.DocumentFileName,
		func(document *docir.Document) (string, error) { return docrender.Markdown(document, opts) })
}

// RenderDocumentSetHTML renders the same set as linked HTML files, each
// referring to the others by their .html file names.
func (s *Session) RenderDocumentSetHTML(opts docrender.HTMLOptions) ([]RenderedDocument, error) {
	return s.renderDocumentSet(docrender.DocumentHTMLFileName,
		func(document *docir.Document) (string, error) { return docrender.HTML(document, opts) })
}

// renderDocumentSet evaluates every declared document together and renders
// each with one backend, naming its file as that backend does.
func (s *Session) renderDocumentSet(
	fileName func(string) string,
	render func(*docir.Document) (string, error),
) ([]RenderedDocument, error) {
	defer s.enter()()
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, fmt.Errorf("runtime init: %w", err)
	}
	idx := s.browseIndex()
	sem, resolver := ctx.Semantics(), ctx.Resolver()
	syms := s.symbolsInLoadOrder(func(scope *symbols.Scope) []*symbols.Symbol {
		return model.DeclaredDocumentDefinitions(idx, sem, scope)
	})
	sort.SliceStable(syms, func(i, j int) bool {
		return symbols.FQNOf(syms[i]) < symbols.FQNOf(syms[j])
	})
	plans := make([]*docplan.Plan, 0, len(syms))
	names := map[string]bool{}
	// Filenames are compared case-folded so the set stays writable on
	// case-insensitive filesystems.
	files := map[string]string{}
	for _, sym := range syms {
		plan, err := docplan.Compile(idx, sem, resolver, sym)
		if err != nil {
			return nil, err
		}
		if names[plan.Name()] {
			return nil, fmt.Errorf("%s names more than one document; rename one so the name is unambiguous", notationName(plan.Name()))
		}
		names[plan.Name()] = true
		file := fileName(plan.Name())
		if other, ok := files[strings.ToLower(file)]; ok {
			return nil, fmt.Errorf("%s and %s render to file names that differ only by letter case; rename one so both files can coexist on a case-insensitive filesystem", notationName(other), notationName(plan.Name()))
		}
		files[strings.ToLower(file)] = plan.Name()
		plans = append(plans, plan)
	}
	documents, err := docir.EvaluateSet(plans,
		queryexec.Context{Index: idx, Resolver: resolver, Model: sem},
		queryexec.Options{},
		s.sessionSourceText())
	if err != nil {
		return nil, err
	}
	out := make([]RenderedDocument, 0, len(documents))
	for _, document := range documents {
		rendered, err := render(document)
		if err != nil {
			return nil, err
		}
		out = append(out, RenderedDocument{
			Name:     document.Name(),
			FileName: fileName(document.Name()),
			Content:  rendered,
		})
	}
	return out, nil
}

// doRenderDocument carries out %render-document, printing the rendered
// Markdown or reporting a document that could not be rendered. A second
// word names the form its graph-shaped diagrams are written in.
func (s *Session) doRenderDocument(invocation string) ([]string, bool, error) {
	fields := splitQueryArgs(strings.TrimSpace(invocation))
	if len(fields) == 0 || len(fields) > 2 {
		return []string{renderDocumentUsage}, false, nil
	}
	var opts docrender.MarkdownOptions
	if len(fields) == 2 {
		opts.DiagramForm = view.Form(fields[1])
		if !slices.Contains(view.DiagramForms(), opts.DiagramForm) {
			return []string{errPrefix + fmt.Sprintf("%q is not a diagram form (%s); a document binds its queries' parameters in the model",
				fields[1], view.FormNames(view.DiagramForms()))}, false, nil
		}
	}
	markdown, err := s.renderDocumentMarkdown(fields[0], opts)
	if err != nil {
		return []string{errPrefix + err.Error()}, false, nil
	}
	return strings.Split(strings.TrimRight(markdown, "\n"), "\n"), false, nil
}
