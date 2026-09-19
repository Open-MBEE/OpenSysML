package semantics

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The names of the shared frames the workspace-wide indexes are built in (see
// Model.shared). A frame is named so Regather can drop it by name when a gather
// it was built from changed; the names cannot be a document's.
const (
	sharedAbout     = "\x00about"
	sharedUnits     = "\x00units"
	sharedDocs      = "\x00docs"
	sharedScalars   = "\x00scalars"
	sharedBaseUnits = "\x00baseUnits"
)

// docGather is what a workspace-wide index is built from per writable document,
// collected syntactically so it is cheap to keep current: the `about` metadata
// usages it declares and the attribute usages its packages declare, the
// candidates for measurement units.
type docGather struct {
	about   []*symbols.Symbol
	units   []*symbols.Symbol
	library bool
}

// gatherOf collects doc's gather from the tree the index holds for it; a
// document the index does not hold gathers nothing.
func (m *Model) gatherOf(doc string) *docGather {
	root := m.resolver.Index().DocumentRoot(doc)
	if root == nil {
		return nil
	}
	g := &docGather{library: m.resolver.Index().IsLibraryDocument(doc)}
	m.collectAbout(root, g, make(map[*symbols.Symbol]bool))
	collectUnitCandidates(root, g)
	return g
}

// collectAbout walks a scope tree — anonymous members included, so `metadata :
// T about x;` counts like a named usage — for its `about` metadata usages.
func (m *Model) collectAbout(scope *symbols.Scope, g *docGather, seen map[*symbols.Symbol]bool) {
	if scope == nil {
		return
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym == nil || seen[sym] {
			return true
		}
		seen[sym] = true
		if usage, ok := sym.Decl.(*ast.Usage); ok && sym.Kind == symbols.SymbolMetadataUsage && annotatesOthers(usage) {
			g.about = append(g.about, sym)
		}
		m.collectAbout(sym.Scope, g, seen)
		return true
	})
}

// collectUnitCandidates walks a scope tree's packages for the attribute usages
// they declare, which indexCoherentUnits judges as units.
func collectUnitCandidates(scope *symbols.Scope, g *docGather) {
	for _, sym := range scope.Members() {
		switch sym.Kind {
		case symbols.SymbolPackage, symbols.SymbolNamespace:
			if sym.Scope != nil {
				collectUnitCandidates(sym.Scope, g)
			}
		case symbols.SymbolAttributeUsage:
			g.units = append(g.units, sym)
		}
	}
}

// gathers returns the per-document gathers, collecting them from every document
// the index holds on first use; Regather keeps them current from then on. The
// frozen documents' `about` usages come from the cache built when they froze.
func (m *Model) gathers() map[string]*docGather {
	if m.docGathers != nil {
		return m.docGathers
	}
	m.docGathers = map[string]*docGather{}
	m.resolver.Untracked(func() {
		idx := m.resolver.Index()
		for _, doc := range idx.Documents() {
			if usages, cached := idx.FrozenAboutUsages(doc); cached {
				m.docGathers[doc] = &docGather{about: usages, library: true}
				continue
			}
			if g := m.gatherOf(doc); g != nil {
				m.docGathers[doc] = g
			}
		}
	})
	return m.docGathers
}

// gatheredDocs lists the gathered documents in index order.
func (m *Model) gatheredDocs() []string {
	gathers := m.gathers()
	out := make([]string, 0, len(gathers))
	for doc := range gathers {
		out = append(out, doc)
	}
	sort.Strings(out)
	return out
}

// Regather implements resolve.Regatherer: it collects the gathers of the
// documents that changed anew and names the shared frames whose indexes they
// were built into, for the resolver to drop with their dependents.
func (m *Model) Regather(docs map[string]bool) []string {
	if m.docGathers == nil {
		return nil
	}
	changed := map[string]bool{}
	for doc := range docs {
		old, had := m.docGathers[doc]
		g := m.gatherOf(doc)
		if had != (g != nil) {
			changed[sharedDocs] = true
		}
		if len(old.aboutOf()) > 0 || len(g.aboutOf()) > 0 {
			changed[sharedAbout] = true
		}
		if len(old.unitsOf()) > 0 || len(g.unitsOf()) > 0 {
			changed[sharedUnits] = true
		}
		if g == nil {
			delete(m.docGathers, doc)
		} else {
			m.docGathers[doc] = g
		}
	}
	out := make([]string, 0, len(changed))
	for name := range changed {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (g *docGather) aboutOf() []*symbols.Symbol {
	if g == nil {
		return nil
	}
	return g.about
}

func (g *docGather) unitsOf() []*symbols.Symbol {
	if g == nil {
		return nil
	}
	return g.units
}
