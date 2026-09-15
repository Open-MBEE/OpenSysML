package identity

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity/normative"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// LibraryLanguage is the half of the standard library a bundled tier belongs to:
// Kernel is KerML, Systems and Domain are SysML; extensions and workspace are neither.
func LibraryLanguage(tier symbols.LibraryTier) (normative.Language, bool) {
	switch tier {
	case symbols.TierKernelSemantic, symbols.TierKernelDataType, symbols.TierKernelFunction:
		return normative.KerML, true
	case symbols.TierSystems, symbols.TierDomain:
		return normative.SysML, true
	}
	return 0, false
}

// qualifier decides which library symbols the norm gives a qualified name, and so an id.
type qualifier struct {
	idx  *symbols.Index
	memo map[*symbols.Symbol]bool
}

func newQualifier(idx *symbols.Index) *qualifier {
	return &qualifier{idx: idx, memo: make(map[*symbols.Symbol]bool)}
}

// language is the language whose norm fixes sym's id, if any.
func (q *qualifier) language(sym *symbols.Symbol) (normative.Language, bool) {
	lang, ok := LibraryLanguage(q.idx.LibraryTier(sym))
	if !ok || !q.qualified(sym) {
		return 0, false
	}
	return lang, true
}

// qualified reports whether sym and every owner is named and first so named in
// its namespace; an unnamed or shadowed element, and anything below one, has no id.
func (q *qualifier) qualified(sym *symbols.Symbol) bool {
	if known, seen := q.memo[sym]; seen {
		return known
	}
	ok := sym.Name != "" && sym.Kind != symbols.SymbolAlias && firstSoNamed(sym)
	if ok && sym.OwnerScope != nil {
		if owner := sym.OwnerScope.Owner(); owner != nil {
			ok = q.qualified(owner)
		}
	}
	q.memo[sym] = ok
	return ok
}

// firstSoNamed reports whether sym is the first owned (non-alias) member of its
// namespace to carry its name, which is the one the name qualifies.
func firstSoNamed(sym *symbols.Symbol) bool {
	if sym.OwnerScope == nil {
		return true
	}
	for _, member := range sym.OwnerScope.LookupLocalAll(sym.Name) {
		if member.Kind != symbols.SymbolAlias {
			return member == sym
		}
	}
	return false
}

// LibraryElement is one named element of the standard library and the ids the
// norm fixes for it and for the membership that owns it.
type LibraryElement struct {
	Symbol             *symbols.Symbol
	FQN                string
	Language           normative.Language
	ID                 string
	OwningMembershipID string
}

// Catalog indexes the normative ids of the bundled library by id: a version-5
// UUID cannot be reversed, only looked up. Elements are indexed by name too;
// roots holds every library document's top-level packages, id or none, and
// kinds each document's language.
type Catalog struct {
	elements    map[string]*LibraryElement
	memberships map[string]*LibraryElement
	names       map[string]*LibraryElement
	roots       map[string]*LibraryElement
	kinds       map[string]source.Kind
	order       []*LibraryElement
}

// Element is the library element whose normative id is id.
func (c *Catalog) Element(id string) (*LibraryElement, bool) {
	el, ok := c.elements[id]
	return el, ok
}

// ElementNamed is the library element whose qualified name is fqn.
func (c *Catalog) ElementNamed(fqn string) (*LibraryElement, bool) {
	el, ok := c.names[fqn]
	return el, ok
}

// RootNamed is the top-level package of a library document named fqn; its ID is
// "" when the document's tier has no normative language. A name more than one
// library document declares at its top names no single document, so it is not found.
func (c *Catalog) RootNamed(fqn string) (*LibraryElement, bool) {
	el, ok := c.roots[fqn]
	return el, ok && el != nil
}

// DocumentKind is the language the named library document is written in.
func (c *Catalog) DocumentKind(doc string) source.Kind { return c.kinds[doc] }

// OwningMembership is the library element whose owning membership has normative id id.
func (c *Catalog) OwningMembership(id string) (*LibraryElement, bool) {
	el, ok := c.memberships[id]
	return el, ok
}

// Elements lists the catalogued elements in library walk order.
func (c *Catalog) Elements() []*LibraryElement { return c.order }

// catalogs memoizes the catalog of each frozen library index.
var catalogs sync.Map // *symbols.Index → *Catalog

// LibraryCatalog is the catalog of the library idx holds, shared by every
// overlay that shows its base's library documents as the base does.
func LibraryCatalog(idx *symbols.Index) *Catalog {
	if idx == nil {
		return newCatalog()
	}
	key := idx
	if base := idx.Base(); base != nil && showsLibraryOf(idx, base) {
		key = base
	}
	if !key.Frozen() {
		return buildCatalog(key)
	}
	if c, ok := catalogs.Load(key); ok {
		return c.(*Catalog)
	}
	c, _ := catalogs.LoadOrStore(key, buildCatalog(key))
	return c.(*Catalog)
}

// showsLibraryOf reports whether idx holds exactly base's library documents, each
// as base does: an overlay may shadow one under its name, remove it, or add its own.
func showsLibraryOf(idx, base *symbols.Index) bool {
	held := 0
	for _, name := range idx.Documents() {
		if !idx.IsLibraryDocument(name) {
			continue
		}
		if !base.IsLibraryDocument(name) || idx.DocumentRoot(name) != base.DocumentRoot(name) ||
			idx.LibraryDocumentOf(name) != base.LibraryDocumentOf(name) ||
			idx.DocumentKind(name) != base.DocumentKind(name) {
			return false
		}
		held++
	}
	for _, name := range base.Documents() {
		if base.IsLibraryDocument(name) {
			held--
		}
	}
	return held == 0
}

func newCatalog() *Catalog {
	return &Catalog{
		elements:    map[string]*LibraryElement{},
		memberships: map[string]*LibraryElement{},
		names:       map[string]*LibraryElement{},
		roots:       map[string]*LibraryElement{},
		kinds:       map[string]source.Kind{},
	}
}

// buildCatalog fixes the id of every element the norm names in idx's library documents.
func buildCatalog(idx *symbols.Index) *Catalog {
	c := newCatalog()
	q := newQualifier(idx)
	var roots []*symbols.Scope
	for _, name := range idx.Documents() {
		if !idx.IsLibraryDocument(name) {
			continue
		}
		if root := idx.DocumentRoot(name); root != nil {
			roots = append(roots, root)
			c.kinds[name] = idx.DocumentKind(name)
		}
	}
	for _, sym := range collectSymbols(roots) {
		lang, ok := q.language(sym)
		if !ok {
			continue
		}
		fqn := idx.GetFQN(sym)
		if fqn == "" {
			continue
		}
		el := &LibraryElement{
			Symbol:             sym,
			FQN:                fqn,
			Language:           lang,
			ID:                 normative.ElementID(lang, fqn),
			OwningMembershipID: normative.OwningMembershipID(lang, fqn),
		}
		c.elements[el.ID] = el
		c.memberships[el.OwningMembershipID] = el
		c.names[el.FQN] = el
		c.order = append(c.order, el)
	}
	for _, root := range roots {
		for _, sym := range root.Members() {
			if sym.Kind != symbols.SymbolPackage || !firstSoNamed(sym) {
				continue
			}
			fqn := idx.GetFQN(sym)
			if fqn == "" {
				continue
			}
			if _, seen := c.roots[fqn]; seen {
				c.roots[fqn] = nil // declared at the top of two documents: neither's
			} else if el, ok := c.names[fqn]; ok && el.Symbol == sym {
				c.roots[fqn] = el
			} else {
				c.roots[fqn] = &LibraryElement{Symbol: sym, FQN: fqn}
			}
		}
	}
	return c
}
