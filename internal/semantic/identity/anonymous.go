package identity

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// AnonymousMember is an unnamed member of a named library element: the ids this
// implementation mints for it and the positional paths that name it, in the form
// the pilot's exporter keys unnamed elements by (owner/Metaclass[i], i counting the
// owner's preceding owned elements, or relationships, of the same metaclass).
type AnonymousMember struct {
	Symbol              *symbols.Symbol
	Owner               *LibraryElement
	Metaclass           string
	MembershipMetaclass string
	ID                  string
	OwningMembershipID  string
	Path                string
	MembershipPath      string
}

// AnonymousMemberOf describes sym, an unnamed member (declared or implied) a named library element
// owns, placed by the owner's ownedMember order; ok is false for a named symbol — a parameter
// named by the one its position redefines included — or an anonymous owner.
func AnonymousMemberOf(model *semantics.Model, catalog *Catalog, sym *symbols.Symbol) (AnonymousMember, bool) {
	if model == nil || catalog == nil || sym == nil || model.QualifiedNameOf(sym) != "" {
		return AnonymousMember{}, false
	}
	owner := sym.Owner()
	if owner == nil {
		return AnonymousMember{}, false
	}
	ownerElement, ok := catalog.ElementForSymbol(owner)
	if !ok || ownerElement == nil || ownerElement.ID == "" {
		return AnonymousMember{}, false
	}
	metaclass := metaclassName(model, sym)
	if metaclass == "" {
		return AnonymousMember{}, false
	}
	members, ok := model.ReflectiveElements(owner, "ownedMember")
	if !ok {
		return AnonymousMember{}, false
	}
	ordinal, index, found := 0, 0, false
	for _, member := range members {
		if member == sym {
			found = true
			break
		}
		if metaclassName(model, member) == metaclass {
			index++
			if model.QualifiedNameOf(member) == "" {
				ordinal++
			}
		}
	}
	if !found {
		return AnonymousMember{}, false
	}
	out := AnonymousMember{
		Symbol:             sym,
		Owner:              ownerElement,
		Metaclass:          metaclass,
		ID:                 AnonymousMemberID(ownerElement.ID, metaclass, ordinal),
		OwningMembershipID: AnonymousMemberOwningMembershipID(ownerElement.ID, metaclass, ordinal),
		Path:               ownerElement.FQN + "/" + metaclass + "[" + strconv.Itoa(index) + "]",
	}
	if out.ID == "" {
		return AnonymousMember{}, false
	}
	memberships := model.OwnedMemberships(owner)
	for _, membership := range memberships {
		if membership.Implicit != nil && membership.Implicit.Member == sym {
			out.MembershipMetaclass = metaclassName(model, membership)
			break
		}
	}
	if out.MembershipMetaclass == "" {
		return AnonymousMember{}, false
	}
	index = 0
	for _, membership := range memberships {
		if membership.Implicit != nil && membership.Implicit.Member == sym {
			out.MembershipPath = ownerElement.FQN + "/" + out.MembershipMetaclass + "[" + strconv.Itoa(index) + "]"
			break
		}
		if metaclassName(model, membership) == out.MembershipMetaclass {
			index++
		}
	}
	return out, true
}

func metaclassName(model *semantics.Model, sym *symbols.Symbol) string {
	if metaclass := model.MetaclassOf(sym); metaclass != nil {
		return metaclass.Name
	}
	return ""
}

// AnonymousIndex finds the unnamed members of the bundled library by the ids this
// implementation mints for them, which a version-5 UUID cannot be reversed to.
type AnonymousIndex struct {
	byID, byMembershipID map[string]AnonymousMember
	bySymbol             map[*symbols.Symbol]AnonymousMember
}

// AnonymousMemberIndex indexes every unnamed member a named element of catalog
// owns, declared or implied, by element id and by owning-membership id.
func AnonymousMemberIndex(model *semantics.Model, catalog *Catalog) *AnonymousIndex {
	index := &AnonymousIndex{
		byID: map[string]AnonymousMember{}, byMembershipID: map[string]AnonymousMember{},
		bySymbol: map[*symbols.Symbol]AnonymousMember{},
	}
	if model == nil || catalog == nil {
		return index
	}
	for _, owner := range catalog.Elements() {
		if owner.Symbol == nil {
			continue
		}
		members, ok := model.ReflectiveElements(owner.Symbol, "ownedMember")
		if !ok {
			continue
		}
		for _, sym := range members {
			if sym == nil || sym.Name != "" {
				continue
			}
			if member, ok := AnonymousMemberOf(model, catalog, sym); ok {
				index.byID[member.ID] = member
				index.byMembershipID[member.OwningMembershipID] = member
				index.bySymbol[sym] = member
			}
		}
	}
	return index
}

// Member is the indexed unnamed member declared by sym, as AnonymousMemberOf derives it.
func (i *AnonymousIndex) Member(sym *symbols.Symbol) (AnonymousMember, bool) {
	if i == nil {
		return AnonymousMember{}, false
	}
	member, ok := i.bySymbol[sym]
	return member, ok
}

// Element is the unnamed member with the given element id.
func (i *AnonymousIndex) Element(id string) (AnonymousMember, bool) {
	if i == nil {
		return AnonymousMember{}, false
	}
	member, ok := i.byID[id]
	return member, ok
}

// OwningMembership is the unnamed member whose owning membership has the given id.
func (i *AnonymousIndex) OwningMembership(id string) (AnonymousMember, bool) {
	if i == nil {
		return AnonymousMember{}, false
	}
	member, ok := i.byMembershipID[id]
	return member, ok
}

// OwningMembershipMetaclass is the metaclass of the membership through which a
// library element's owner owns it (FeatureMembership, OwningMembership, …), or "".
func OwningMembershipMetaclass(model *semantics.Model, sym *symbols.Symbol) string {
	if model == nil || sym == nil {
		return ""
	}
	owner := sym.Owner()
	if owner == nil {
		return ""
	}
	for _, membership := range model.OwnedMemberships(owner) {
		if membership.Implicit != nil && membership.Implicit.Member == sym {
			return metaclassName(model, membership)
		}
	}
	return ""
}
