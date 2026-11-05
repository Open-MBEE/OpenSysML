package symbols

import "github.com/Open-MBEE/OpenSysML/internal/syntax/ast"

// MetadataBodyResolver is the inheritance-aware lookup needed by metadata bodies.
type MetadataBodyResolver interface {
	LookupMember(*Symbol, string) (*Symbol, bool)
}

// MetadataBodyTarget returns the feature a metadata body declaration implicitly
// redefines, accepting either its primary or short name.
func MetadataBodyTarget(lookup MetadataBodyResolver, owner *Symbol,
	ident ast.Identification) *Symbol {
	if lookup == nil || owner == nil {
		return nil
	}
	for _, name := range []string{ident.Name, ident.ShortName} {
		if name == "" {
			continue
		}
		if member, ok := lookup.LookupMember(owner, name); ok {
			return member
		}
	}
	return nil
}

// IsPrefixMetadata reports whether s is the metadata usage a `#M` prefix
// declares on the element owning it (SysML v2 PrefixMetadataMember), rather
// than one written as a member of a body: the builder registers it in the
// annotated element's scope with a body scope that names the declaration it is
// written on, and a record keeps the fact as ModPrefixMetadata.
func (s *Symbol) IsPrefixMetadata() bool {
	if s == nil || s.Kind != SymbolMetadataUsage {
		return false
	}
	if s.Recorded() {
		return s.Facts.Modifiers&ModPrefixMetadata != 0
	}
	return s.Scope != nil && s.Scope.Annotated() != nil
}
