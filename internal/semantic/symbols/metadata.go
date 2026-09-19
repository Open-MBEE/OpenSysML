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
