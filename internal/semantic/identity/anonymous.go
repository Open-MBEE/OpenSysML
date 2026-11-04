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
	Owner               *LibraryElement
	Metaclass           string
	MembershipMetaclass string
	ID                  string
	OwningMembershipID  string
	Path                string
	MembershipPath      string
}

// AnonymousMemberOf describes sym, an unnamed member declared directly in a
// named library element; ok is false for a named symbol or an anonymous owner.
func AnonymousMemberOf(model *semantics.Model, catalog *Catalog, sym *symbols.Symbol) (AnonymousMember, bool) {
	if model == nil || catalog == nil || sym == nil || sym.Name != "" {
		return AnonymousMember{}, false
	}
	owner := sym.Owner()
	if owner == nil || owner.Scope == nil {
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
	ordinal := -1
	owner.Scope.ForEachAnonymousMember(func(member *symbols.Symbol) bool {
		if metaclassName(model, member) == metaclass {
			ordinal++
		}
		return member != sym
	})
	out := AnonymousMember{
		Owner:              ownerElement,
		Metaclass:          metaclass,
		ID:                 AnonymousMemberID(ownerElement.ID, metaclass, ordinal),
		OwningMembershipID: AnonymousMemberOwningMembershipID(ownerElement.ID, metaclass, ordinal),
	}
	if out.ID == "" {
		return AnonymousMember{}, false
	}
	members, ok := model.ReflectiveElements(owner, "ownedMember")
	if !ok {
		return AnonymousMember{}, false
	}
	index := 0
	for _, member := range members {
		if member == sym {
			out.Path = ownerElement.FQN + "/" + metaclass + "[" + strconv.Itoa(index) + "]"
			break
		}
		if metaclassName(model, member) == metaclass {
			index++
		}
	}
	if out.Path == "" {
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
}

// AnonymousMemberIndex indexes every unnamed member declared directly in a
// named element of catalog, by element id and by owning-membership id.
func AnonymousMemberIndex(model *semantics.Model, catalog *Catalog) *AnonymousIndex {
	index := &AnonymousIndex{byID: map[string]AnonymousMember{}, byMembershipID: map[string]AnonymousMember{}}
	if model == nil || catalog == nil {
		return index
	}
	for _, owner := range catalog.Elements() {
		if owner.Symbol == nil || owner.Symbol.Scope == nil {
			continue
		}
		owner.Symbol.Scope.ForEachAnonymousMember(func(sym *symbols.Symbol) bool {
			if member, ok := AnonymousMemberOf(model, catalog, sym); ok {
				index.byID[member.ID] = member
				index.byMembershipID[member.OwningMembershipID] = member
			}
			return true
		})
	}
	return index
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
