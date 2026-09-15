package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// ErrNoView is a document that declares no view, which is rendered through a
// pseudo-view instead.
var ErrNoView = errors.New("declares no view")

// ViewInfo is one view a document declares: its qualified name, the rendering
// kind it states, whether this implementation produces that kind, and why not
// when it does not.
type ViewInfo struct {
	Name      string
	Kind      view.Kind
	Supported bool
	Reason    string
}

// Views lists the views a document declares, in qualified-name order, each with
// the rendering kind it states. A recognized kind this build does not produce
// is listed as unsupported with the reason.
func (w *Workspace) Views(doc string) []ViewInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	renderer := w.rendererLocked(doc)
	if renderer == nil {
		return nil
	}
	out := []ViewInfo{}
	for _, sym := range w.documentViewsLocked(doc) {
		info := ViewInfo{Name: notationFQN(w.index, sym), Supported: true}
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
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RenderView renders a view of a document. fqn names a declared view or a
// pseudo-view (`#<kind>[:<fqn>]`); "" renders the document's own view. The
// document returned is the one the rendering was made from, read under the same
// lock, so its version, content and scope are the rendering's.
func (w *Workspace) RenderView(doc, fqn string) (*view.Rendering, *Document, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	d := w.docs[doc]
	if d == nil {
		return nil, nil, fmt.Errorf("%s: no such document", doc)
	}
	rendering, err := w.renderViewLocked(d, fqn)
	if err != nil {
		return nil, nil, err
	}
	return rendering, d, nil
}

// renderViewLocked renders fqn of the held document d under the read lock.
func (w *Workspace) renderViewLocked(d *Document, fqn string) (*view.Rendering, error) {
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

// rendererLocked builds a renderer over the workspace index, reading the
// document's own content for the labels a rendering takes verbatim. It is the
// same construction Session.viewRenderer makes in the REPL.
func (w *Workspace) rendererLocked(doc string) *view.Renderer {
	d := w.docs[doc]
	if d == nil {
		return nil
	}
	resolver, sem := w.newResolver()
	sf := source.New(doc, d.Content)
	text := func(name string, span source.Span) string {
		if name != doc {
			return ""
		}
		return sf.Text(span)
	}
	return view.NewRenderer(sem, resolver, text)
}

// sourceTextLocked reads notation from any of the workspace's documents, and
// behind them from the library files its index holds, for the labels a
// rendering takes verbatim across document boundaries.
func (w *Workspace) sourceTextLocked() view.SourceText {
	files := make(map[string]*source.SourceFile, len(w.docs))
	for name, d := range w.docs {
		files[name] = source.New(name, d.Content)
	}
	return source.TextOf(files, libs.Text(w.libSource))
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

// Declared is declaredInLocked for a caller outside the lock.
func (w *Workspace) Declared(doc, fqn string) *symbols.Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return declaredIn(w.index, doc, fqn)
}

// DeclarationText is the source text of sym's declaration as the workspace
// holds it now, "" when its document is gone.
func (w *Workspace) DeclarationText(sym *symbols.Symbol) string {
	if sym == nil || sym.Decl == nil {
		return ""
	}
	return w.TextAt(sym.DocName, sym.Decl.Span())
}

// TextAt is the source text at span in doc as the workspace holds it now, from
// the document or the library file behind the documents; "" when neither holds doc.
func (w *Workspace) TextAt(doc string, span source.Span) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if d := w.docs[doc]; d != nil {
		return source.New(doc, d.Content).Text(span)
	}
	if text := libs.Text(w.libSource); text != nil {
		return text(doc, span)
	}
	return ""
}

// DeclaredView is the view RenderView renders for fqn in doc: the view named, the
// document's one view for "", nil for a pseudo-view or a name declaring none.
func (w *Workspace) DeclaredView(doc, fqn string) *symbols.Symbol {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if strings.HasPrefix(fqn, view.PseudoViewPrefix) {
		return nil
	}
	if fqn == "" {
		if views := w.documentViewsLocked(doc); len(views) == 1 {
			return views[0]
		}
		return nil
	}
	return w.viewNamedLocked(doc, fqn)
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
