package semantics

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// uniqueByLibraryNote names the Collections features that redefine a nonunique
// `elements` and are unique by the library's own note (docs/project/omg-issues.md).
var uniqueByLibraryNote = map[string]bool{
	"Collections::UniqueCollection::elements": true,
	"Collections::Map::elements":              true,
	"Collections::OrderedSet::elements":       true,
	"Collections::OrderedMap::elements":       true,
}

// IsUnique reports whether a feature holds no two equal values (KerML 7.3.4.4): not
// if declared nonunique, else when every feature it redefines is (7.3.4.5), else yes.
func (m *Model) IsUnique(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return true
	}
	if cached, ok := m.unique[sym]; ok {
		return cached
	}
	unique := m.isUnique(sym, map[*symbols.Symbol]bool{sym: true})
	if m.computingRedefinedFeatures == 0 {
		m.unique[sym] = unique
	}
	return unique
}

// isUnique is IsUnique along the redefinition chain; path holds the features
// being computed, so a cyclic chain contributes nothing.
func (m *Model) isUnique(sym *symbols.Symbol, path map[*symbols.Symbol]bool) bool {
	if DeclaredNonunique(sym) {
		return false
	}
	if m.uniqueByLibraryNote(sym) {
		return true
	}
	for _, general := range m.directRedefinedFeatures(sym) {
		if general == nil || path[general] {
			continue
		}
		path[general] = true
		unique := m.isUnique(general, path)
		delete(path, general)
		if !unique {
			return false
		}
	}
	return true
}

// UniquenessViolation words a value written twice to a unique feature, for the
// static and runtime checks alike; first and second are 1-based positions.
func UniquenessViolation(text string, first, second int) string {
	return fmt.Sprintf("%s is written at positions %d and %d of a unique feature", text, first, second)
}

// The library Collections declare `elements` once as nonunique, at the root,
// and redefine it unique (UniqueCollection, Map) or ordered (OrderedCollection).
const (
	collectionElementsFQN = "Collections::Collection::elements"
	orderedCollectionFQN  = "Collections::OrderedCollection"
)

// HoldsSet reports whether a multi-valued feature of typeSym holds a set rather
// than a sequence: `elements` of a library Collection whose own redefinition of
// it is unique and not ordered — Set's and Map's, not Bag's, which inherits the
// nonunique root, and not OrderedSet's or OrderedMap's — unless the feature
// itself is declared ordered or nonunique.
func (m *Model) HoldsSet(feat, typeSym *symbols.Symbol) bool {
	if m == nil || feat == nil || DeclaredOrdered(feat) || DeclaredNonunique(feat) {
		return false
	}
	root := m.librarySymbol(collectionElementsFQN)
	if root == nil || !m.specializes(feat, root) {
		return false
	}
	if m.specializes(typeSym, m.librarySymbol(orderedCollectionFQN)) {
		return false
	}
	redefined := m.libraryDeclared(feat) && feat != root
	for _, sup := range m.AllSupertypes(feat) {
		if sup == root || !m.libraryDeclared(sup) || !m.specializes(sup, root) {
			continue
		}
		if DeclaredOrdered(sup) || DeclaredNonunique(sup) {
			return false
		}
		redefined = true
	}
	return redefined
}

// specializes reports whether sym is general, or has it among its supertypes.
func (m *Model) specializes(sym, general *symbols.Symbol) bool {
	if sym == nil || general == nil {
		return false
	}
	if sym == general {
		return true
	}
	for _, sup := range m.AllSupertypes(sym) {
		if sup == general {
			return true
		}
	}
	return false
}

// libraryDeclared reports whether sym was declared by a bundled library document.
func (m *Model) libraryDeclared(sym *symbols.Symbol) bool {
	if m.resolver == nil || m.resolver.Index() == nil {
		return false
	}
	return m.resolver.Index().Library(sym)
}

// librarySymbol is the declaration the bundled library makes under fqn, nil
// where it is not loaded.
func (m *Model) librarySymbol(fqn string) *symbols.Symbol {
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	for _, sym := range m.resolver.Index().LookupQualified(fqn) {
		if m.libraryDeclared(sym) {
			return sym
		}
	}
	return nil
}

// uniqueByLibraryNote reports a bundled library declaration read as unique.
func (m *Model) uniqueByLibraryNote(sym *symbols.Symbol) bool {
	if m.resolver == nil || m.resolver.Index() == nil {
		return false
	}
	idx := m.resolver.Index()
	return idx.Library(sym) && uniqueByLibraryNote[idx.GetFQN(sym)]
}

// DeclaredNonunique reports whether a feature's own declaration says nonunique.
func DeclaredNonunique(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.IsNonunique
	case *ast.CrossFeatureMember:
		return d.IsNonunique
	}
	return false
}

// DeclaredOrdered reports whether a feature's own declaration says ordered.
func DeclaredOrdered(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		return d.IsOrdered
	case *ast.CrossFeatureMember:
		return d.IsOrdered
	}
	return false
}
