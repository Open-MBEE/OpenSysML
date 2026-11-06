package identity_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// The positional paths are the keys the pilot's exporter gives the same unnamed
// members of the standard library (its key table for a training example).
func TestAnonymousMemberPathsMatchThePilotsPositionalKeys(t *testing.T) {
	base := libs.SharedBase()
	resolver := resolve.New(base)
	model := semantics.NewModel(resolver)
	catalog := identity.LibraryCatalogWithOpenSysML(base)
	cases := []struct {
		owner, metaclass string
		ordinal          int
		path, membership string
	}{
		{"Parts::Part", "Documentation", 0, "Parts::Part/Documentation[0]", "Parts::Part/OwningMembership[0]"},
		{"Occurrences::Occurrence", "Documentation", 0, "Occurrences::Occurrence/Documentation[0]", "Occurrences::Occurrence/OwningMembership[0]"},
		{"Occurrences::Occurrence", "Connector", 0, "Occurrences::Occurrence/Connector[0]", "Occurrences::Occurrence/FeatureMembership[4]"},
		{"Occurrences::Occurrence", "Invariant", 0, "Occurrences::Occurrence/Invariant[0]", "Occurrences::Occurrence/FeatureMembership[23]"},
	}
	for _, c := range cases {
		owner := resolver.Index().LookupQualified(c.owner)[0]
		var found *identity.AnonymousMember
		seen := -1
		owner.Scope.ForEachAnonymousMember(func(sym *symbols.Symbol) bool {
			member, ok := identity.AnonymousMemberOf(model, catalog, sym)
			if ok && member.Metaclass == c.metaclass {
				seen++
				if seen == c.ordinal {
					found = &member
					return false
				}
			}
			return true
		})
		if found == nil {
			t.Errorf("%s has no anonymous %s #%d", c.owner, c.metaclass, c.ordinal)
			continue
		}
		if found.Path != c.path || found.MembershipPath != c.membership {
			t.Errorf("%s %s #%d: paths %s / %s, want %s / %s", c.owner, c.metaclass, c.ordinal, found.Path, found.MembershipPath, c.path, c.membership)
		}
		ownerElement, _ := catalog.ElementNamed(c.owner)
		if found.ID != identity.AnonymousMemberID(ownerElement.ID, c.metaclass, c.ordinal) || found.OwningMembershipID != identity.AnonymousMemberOwningMembershipID(ownerElement.ID, c.metaclass, c.ordinal) {
			t.Errorf("%s %s #%d: ids differ from the minting functions", c.owner, c.metaclass, c.ordinal)
		}
	}
}
