package symbolfacts

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// Member is one member of a symbol, own or inherited, as the service reports it.
type Member struct {
	// Id is what a symbol lookup takes: the member's qualified name, or the
	// positional id (`Owner::@3`) of an unnamed one.
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
// directly still reports what it declares. A member no id reaches is left out.
func (sc *Context) membersOf(sym *symbols.Symbol, includeLibrary bool) ([]*Member, int32) {
	out := []*Member{}
	var withheld int32
	own := make(map[*symbols.Symbol]bool)
	// The own members are the scope's, exactly as child_ids lists them: a `#M`
	// prefix is owned through its Annotation yet navigable like any member.
	if sym.Scope != nil {
		for _, member := range sym.Scope.AllMembers() {
			id := sc.Identity(member)
			if id == "" {
				continue
			}
			own[member] = true
			out = append(out, &Member{Id: id, Name: sc.Semantics.EffectiveNameOf(member), Kind: member.Kind.String(), Library: sc.libraryDeclared(sym)})
		}
	}
	for _, member := range sc.Semantics.Members(sym) {
		if !member.Inherited || own[member.Symbol] {
			continue
		}
		id := sc.Identity(member.Symbol)
		if id == "" {
			continue
		}
		// Provenance is the declaring type's: an unnamed member has no
		// qualified name of its own to place it by.
		source := member.Source
		if source == nil {
			source = sym
		}
		// A parameter the library gives a behavior (`result`, `verdict`) is part
		// of its signature, so it is listed as a declared `return r` is.
		if sc.libraryDeclared(source) && !includeLibrary && !parameterMemberships[sc.Semantics.MembershipMetaclassOf(sym, member.Symbol)] {
			withheld++
			continue
		}
		info := &Member{
			Id:            id,
			Name:          sc.Semantics.EffectiveNameOf(member.Symbol),
			Kind:          member.Symbol.Kind.String(),
			Inherited:     true,
			InheritedFrom: sc.Index.GetFQN(source),
			Library:       sc.libraryDeclared(source),
		}
		out = append(out, info)
	}
	return out, withheld
}

// parameterMemberships is ParameterMembership and its specializations (SysML v2
// §8.3.21.11, §8.3.21.1, §8.3.21.12; KerML §8.3.4.6 ReturnParameterMembership).
var parameterMemberships = map[string]bool{
	"ParameterMembership":       true,
	"ReturnParameterMembership": true,
	"SubjectMembership":         true,
	"ActorMembership":           true,
	"StakeholderMembership":     true,
}
