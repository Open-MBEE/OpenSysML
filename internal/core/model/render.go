package model

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ErrNoView is a document that declares no view, which is rendered through a
// pseudo-view instead.
var ErrNoView = errors.New("declares no view")

// ViewInfo is one view a document declares: its qualified name, the rendering
// kind it states, whether this implementation produces that kind, and why not
// when it does not. Origin is where the view is declared, so a client can tell
// which view a cursor is in; the zero Origin for a view without a declaration.
type ViewInfo struct {
	Name      string
	Kind      view.Kind
	Supported bool
	Reason    string
	Origin    view.Origin
}

// Views lists the views a document declares, in qualified-name order, each with
// the rendering kind it states. A recognized kind this build does not produce
// is listed as unsupported with the reason. The document returned is the
// snapshot the listing was read from, so the origins are located in its text;
// nil when the workspace does not hold the document.
func (w *Workspace) Views(doc string) ([]ViewInfo, *Document) {
	w.mu.Lock()
	defer w.mu.Unlock()
	d := w.docs[doc]
	if d == nil {
		return nil, nil
	}
	out := []ViewInfo{}
	w.queryLocked(doc, func(*resolve.Resolver, *semantics.Model) {
		renderer := w.rendererLocked(doc)
		for _, sym := range w.documentViewsLocked(doc) {
			info := ViewInfo{Name: notationFQN(w.index, sym), Supported: true, Origin: declarationOrigin(d, sym)}
			kind, _, err := renderer.KindOf(sym)
			switch {
			case err == nil:
				info.Kind = kind
			default:
				info.Supported = false
				info.Reason = err.Error()
				var unsupported *view.UnsupportedKindError
				if errors.As(err, &unsupported) {
					info.Kind = unsupported.Kind
				}
			}
			out = append(out, info)
		}
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, d
}

// declarationOrigin locates a symbol's declaration, trimmed of the trailing
// whitespace the parser reads into a declaration's span, so that a cursor
// between two declarations is in neither.
func declarationOrigin(doc *Document, sym *symbols.Symbol) view.Origin {
	origin := sym.Origin()
	if !origin.Located() || origin.Doc != doc.Name || origin.Span.End() > len(doc.Content) {
		return origin
	}
	trimmed := strings.TrimRightFunc(string(doc.Content[origin.Span.Offset:origin.Span.End()]), unicode.IsSpace)
	origin.Span.Len = len(trimmed)
	return origin
}

// Snapshot is the workspace's documents as one read of them, the read a
// rendering was made under, so that a span the rendering locates in any of
// them is a span of the text held here, whatever the workspace holds since.
type Snapshot struct {
	// Rendered is the document the rendering was asked of.
	Rendered *Document
	docs     map[string]*Document
}

// Document is the named document as the snapshot holds it; nil for a name the
// workspace held no document of, a bundled library file's included.
func (s *Snapshot) Document(name string) *Document {
	return s.docs[name]
}

// RenderView renders a view of a document. fqn names a declared view or a
// pseudo-view (`#<kind>[:<fqn>]`); "" renders the document's own view. The
// snapshot returned holds the documents the rendering was made from, read under
// the same lock, so their versions, content and scopes are the rendering's.
func (w *Workspace) RenderView(doc, fqn string) (*view.Rendering, *Snapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.renderViewLocked(doc, fqn)
}

// renderViewLocked is RenderView under the lock.
func (w *Workspace) renderViewLocked(doc, fqn string) (*view.Rendering, *Snapshot, error) {
	d := w.docs[doc]
	if d == nil {
		return nil, nil, fmt.Errorf("%s: no such document", doc)
	}
	var rendering *view.Rendering
	var err error
	w.queryLocked(doc, func(*resolve.Resolver, *semantics.Model) {
		rendering, err = w.renderDocumentViewLocked(d, fqn)
	})
	if err != nil {
		return nil, nil, err
	}
	return rendering, &Snapshot{Rendered: d, docs: maps.Clone(w.docs)}, nil
}

// renderDocumentViewLocked renders fqn of the held document d, as a query owned by d.
func (w *Workspace) renderDocumentViewLocked(d *Document, fqn string) (*view.Rendering, error) {
	doc := d.Name
	renderer := w.rendererLocked(doc)
	if strings.HasPrefix(fqn, view.PseudoViewPrefix) {
		return w.renderPseudoLocked(doc, fqn, renderer)
	}
	if fqn == "" {
		views := w.documentViewsLocked(doc)
		switch len(views) {
		case 0:
			return nil, fmt.Errorf("%s: %w; render %s instead", doc, ErrNoView, strings.Join(view.PseudoViewSpecs(), ", "))
		case 1:
			return renderer.Render(views[0])
		default:
			return nil, fmt.Errorf("%s: declares %d views (%s); name the one to render",
				doc, len(views), strings.Join(viewNames(w.index, views), ", "))
		}
	}
	sym := w.viewNamedLocked(doc, fqn)
	if sym == nil {
		return nil, fmt.Errorf("%s: no view named %s", doc, fqn)
	}
	rendering, err := renderer.Render(sym)
	if err != nil {
		if errors.Is(err, semantics.ErrNotAView) {
			return nil, fmt.Errorf("%s: %w", fqn, err)
		}
		return nil, err
	}
	return rendering, nil
}

// renderPseudoLocked renders a pseudo-view: `#<kind>` renders the document's own
// top-level elements, `#<kind>:<fqn>` the element named.
func (w *Workspace) renderPseudoLocked(doc, spec string, renderer *view.Renderer) (*view.Rendering, error) {
	kind, target, ok := view.ParsePseudoView(spec)
	if !ok {
		return nil, fmt.Errorf("%s is no pseudo-view: write %s", spec, strings.Join(view.PseudoViewSpecs(), ", "))
	}

	exposed := []*symbols.Symbol{}
	stated := fmt.Sprintf("no view declared; rendering %s directly", doc)
	if target != "" {
		sym := w.declaredInLocked(doc, target)
		if sym == nil {
			return nil, fmt.Errorf("%s: %s names nothing in this document", spec, target)
		}
		exposed = append(exposed, sym)
		stated = fmt.Sprintf("no view declared; rendering %s directly", notationFQN(w.index, sym))
	} else {
		exposed = append(exposed, TopLevelDeclarations(w.index.DocumentRoot(doc))...)
	}
	return renderer.RenderExposed(exposed, kind, stated)
}

// rendererLocked builds a renderer over the workspace's resolver and model,
// reading any of its documents for the labels a rendering takes verbatim, since
// what a behavior inherits is written elsewhere. It is the same construction
// Session.viewRenderer makes in the REPL.
func (w *Workspace) rendererLocked(doc string) *view.Renderer {
	if w.docs[doc] == nil {
		return nil
	}
	resolver, sem := w.semanticsLocked()
	return view.NewRenderer(sem, resolver, w.sourceText())
}

// sourceText reads notation from whichever document the workspace currently
// holds under a name, and behind them from the library files its index holds,
// for the labels a rendering takes verbatim across document boundaries. It is
// read under the lock, so it always sees the document the index was built from.
func (w *Workspace) sourceText() view.SourceText {
	lib := libs.Text(w.libSource)
	return func(doc string, span source.Span) string {
		if d := w.docs[doc]; d != nil {
			return d.sf.Text(span)
		}
		if lib == nil {
			return ""
		}
		return lib(doc, span)
	}
}

// documentViewsLocked are the views the document declares, outermost first, in
// declaration order.
func (w *Workspace) documentViewsLocked(doc string) []*symbols.Symbol {
	return DeclaredViews(w.index.DocumentRoot(doc))
}

// viewNamedLocked is the view fqn names, resolved as the index spells it and as
// the document itself spells it, so a client may name a view either way.
func (w *Workspace) viewNamedLocked(doc, fqn string) *symbols.Symbol {
	if sym := w.declaredInLocked(doc, fqn); sym != nil && semantics.IsView(sym) {
		return sym
	}
	return nil
}

// declaredInLocked is the element fqn names in the document: by qualified name in
// the index, else by qualified or simple name among the document's own
// declarations.
func (w *Workspace) declaredInLocked(doc, fqn string) *symbols.Symbol {
	return declaredIn(w.index, doc, fqn)
}

// declaredIn is declaredInLocked over any index.
func declaredIn(idx *symbols.Index, doc, fqn string) *symbols.Symbol {
	for _, sym := range idx.LookupQualified(fqn) {
		if sym.DocName == doc {
			return sym
		}
	}
	var found *symbols.Symbol
	walkScope(idx.DocumentRoot(doc), func(sym *symbols.Symbol) {
		if found != nil {
			return
		}
		if idx.GetFQN(sym) == fqn || sym.Name == fqn {
			found = sym
		}
	})
	return found
}

// namedFrom is the element fqn names from doc: declaredIn, else the one held
// document declaring it by qualified name; an error lists several such documents.
func namedFrom(idx *symbols.Index, held func(doc string) bool, doc, fqn string) (*symbols.Symbol, error) {
	if sym := declaredIn(idx, doc, fqn); sym != nil {
		return sym, nil
	}
	var found []*symbols.Symbol
	for _, sym := range idx.LookupQualified(fqn) {
		if held(sym.DocName) {
			found = append(found, sym)
		}
	}
	switch len(found) {
	case 0:
		return nil, nil
	case 1:
		return found[0], nil
	}
	docs := make([]string, 0, len(found))
	for _, sym := range found {
		if !slices.Contains(docs, sym.DocName) {
			docs = append(docs, sym.DocName)
		}
	}
	sort.Strings(docs)
	return nil, fmt.Errorf("%s is declared in %s", fqn, strings.Join(docs, ", "))
}

// viewNames names views for an ambiguity message.
func viewNames(idx *symbols.Index, views []*symbols.Symbol) []string {
	out := make([]string, 0, len(views))
	for _, sym := range views {
		out = append(out, notationFQN(idx, sym))
	}
	return out
}

// notationFQN names a symbol by qualified name as the index spells it, falling
// back to its own name.
func notationFQN(idx *symbols.Index, sym *symbols.Symbol) string {
	if idx != nil {
		if fqn := idx.GetFQN(sym); fqn != "" {
			return fqn
		}
	}
	return sym.Name
}
