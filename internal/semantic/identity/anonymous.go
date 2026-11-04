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
