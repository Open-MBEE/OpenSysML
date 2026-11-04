package apijson

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// anonymousLibraryKey matches the bridge's key for an unnamed element owned
// directly by a named library element: owner, metaclass and sibling index.
var anonymousLibraryKey = regexp.MustCompile(`^([^/\[\]]+)/([A-Za-z]+)\[(\d+)\]$`)

// anonymousLibraryKeys keys this repository's ids for the unnamed members of
// library elements the bridge keyed positionally. The bridge indexes an unnamed
// element among its owner's owned elements of the same metaclass (named ones
// counted); our id orders it among the owner's anonymous members of that
// metaclass, so the two are joined through the shared declaration order.
func anonymousLibraryKeys(pilotKeys idKeys) idKeys {
	owners := make(map[string]bool)
	for _, key := range pilotKeys {
		if m := anonymousLibraryKey.FindStringSubmatch(key); m != nil {
			owners[m[1]] = true
		}
	}
	if len(owners) == 0 {
		return nil
	}
	names := make([]string, 0, len(owners))
	for owner := range owners {
		names = append(names, owner)
	}
	sort.Strings(names)
	base := libs.SharedBase()
	resolver := resolve.New(base)
	model := semantics.NewModel(resolver)
	catalog := identity.LibraryCatalogWithOpenSysML(base)
	keys := make(idKeys)
	for _, owner := range names {
		keyAnonymousMembers(owner, resolver, model, catalog, keys)
	}
	return keys
}

// keyAnonymousMembers keys the anonymous members of one named library owner
// and their owning memberships.
func keyAnonymousMembers(owner string, resolver *resolve.Resolver, model *semantics.Model, catalog *identity.Catalog, keys idKeys) {
	symbolsFound := resolver.Index().LookupQualified(owner)
	if len(symbolsFound) != 1 || symbolsFound[0].Scope == nil {
		return
	}
	sym := symbolsFound[0]
	element, ok := catalog.ElementForSymbol(sym)
	if !ok || element == nil || element.ID == "" {
		return
	}
	members, ok := model.ReflectiveElements(sym, "ownedMember")
	if !ok {
		return
	}
	metaclassOf := func(s *symbols.Symbol) string {
		if metaclass := model.MetaclassOf(s); metaclass != nil {
			return metaclass.Name
		}
		return ""
	}
	oursOrdinal := make(map[*symbols.Symbol]int)
	anonymousSeen := make(map[string]int)
	sym.Scope.ForEachAnonymousMember(func(member *symbols.Symbol) bool {
		metaclass := metaclassOf(member)
		oursOrdinal[member] = anonymousSeen[metaclass]
		anonymousSeen[metaclass]++
		return true
	})
	elementSeen := make(map[string]int)
	membershipSeen := make(map[string]int)
	for _, member := range members {
		metaclass := metaclassOf(member)
		membershipKind := metamodel.ElementOf(member).Membership.Metaclass()
		elementIndex, membershipIndex := elementSeen[metaclass], membershipSeen[membershipKind]
		elementSeen[metaclass]++
		membershipSeen[membershipKind]++
		ordinal, anonymous := oursOrdinal[member]
		if !anonymous || metaclass == "" {
			continue
		}
		if id := identity.AnonymousMemberID(element.ID, metaclass, ordinal); id != "" {
			keys[id] = owner + "/" + metaclass + "[" + strconv.Itoa(elementIndex) + "]"
		}
		if membershipKind == "" {
			continue
		}
		if id := identity.AnonymousMemberOwningMembershipID(element.ID, metaclass, ordinal); id != "" {
			keys[id] = owner + "/" + membershipKind + "[" + strconv.Itoa(membershipIndex) + "]"
		}
	}
}
