package symbolfacts

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Member is one member of a symbol, own or inherited, as the service reports it.
type Member struct {
	// Id is the member's FQN, what a symbol lookup takes; an unnamed member has
	// one that names its owner only.
	Id   string
	Name string
	Kind string
	// Inherited is true when the symbol inherits the member rather than
	// declaring it.
	Inherited bool
	// InheritedFrom is the FQN of the type declaring an inherited member; empty
	// for an own member.
	InheritedFrom string
	// Library is true when the member is declared in standard-library content.
	Library bool
}

// membersOf reports the members of sym (KerML Type::member): its own members
// in declaration order, then the ones it inherits in inheritance order, a
// redefinition masking what it redefines and a private member of a supertype
// not inherited (semantics.Model.Members). A member inherited from
// standard-library content — the dozens every part has through Parts::Part and
// Base::Anything — is withheld and counted unless includeLibrary, as
// attributesOf withholds library attributes; a library element asked about
// directly still reports what it declares.
func (sc *Context) membersOf(sym *symbols.Symbol, includeLibrary bool) ([]*Member, int32) {
	out := []*Member{}
	var withheld int32
	for _, member := range sc.Semantics.Members(sym) {
		// Provenance is the declaring type's: an unnamed member has no
		// qualified name of its own to place it by.
		source := member.Source
		if source == nil {
			source = sym
		}
		library := sc.libraryDeclared(source)
		if library && member.Inherited && !includeLibrary {
			withheld++
			continue
		}
		info := &Member{
			Id:        sc.Index.GetFQN(member.Symbol),
			Name:      member.Symbol.Name,
			Kind:      member.Symbol.Kind.String(),
			Inherited: member.Inherited,
			Library:   library,
		}
		if member.Inherited && member.Source != nil {
			info.InheritedFrom = sc.Index.GetFQN(member.Source)
		}
		out = append(out, info)
	}
	return out, withheld
}
