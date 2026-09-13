package identity

import (
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/core/identity/normative"
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
// UUID cannot be reversed, only looked up.
type Catalog struct {
	elements    map[string]*LibraryElement
	memberships map[string]*LibraryElement
	order       []*LibraryElement
}

// Element is the library element whose normative id is id.
func (c *Catalog) Element(id string) (*LibraryElement, bool) {
	el, ok := c.elements[id]
	return el, ok
}

// OwningMembership is the library element whose owning membership has normative id id.
func (c *Catalog) OwningMembership(id string) (*LibraryElement, bool) {
	el, ok := c.memberships[id]
	return el, ok
}

// Normative reports whether id is a library element's or owning membership's normative id.
func (c *Catalog) Normative(id string) bool {
	_, el := c.elements[id]
	_, om := c.memberships[id]
	return el || om
}

// Elements lists the catalogued elements in library walk order.
func (c *Catalog) Elements() []*LibraryElement { return c.order }

// catalogs memoizes the catalog of each frozen library index.
var catalogs sync.Map // *symbols.Index → *Catalog

// LibraryCatalog is the catalog of the library idx holds, shared by every overlay over one base.
func LibraryCatalog(idx *symbols.Index) *Catalog {
	if idx == nil {
		return &Catalog{elements: map[string]*LibraryElement{}, memberships: map[string]*LibraryElement{}}
	}
	key := idx
	if base := idx.Base(); base != nil {
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

// buildCatalog fixes the id of every element the norm names in idx's library documents.
func buildCatalog(idx *symbols.Index) *Catalog {
	c := &Catalog{elements: map[string]*LibraryElement{}, memberships: map[string]*LibraryElement{}}
	q := newQualifier(idx)
	var roots []*symbols.Scope
	for _, name := range idx.Documents() {
		if !idx.IsLibraryDocument(name) {
			continue
		}
		if root := idx.DocumentRoot(name); root != nil {
			roots = append(roots, root)
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
		c.order = append(c.order, el)
	}
	return c
}
