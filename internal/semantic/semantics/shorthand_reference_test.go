package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A feature stated as a name alone reference-subsets what the name resolves to (SysML.xtext OwnedReference),
// a chain as a whole (KerML 8.3.3.3.9); a declared feature and an implied multiplicity do not.
func TestShorthandReferenceIsAReferenceSubsetting(t *testing.T) {
	m, root := buildModelWithStdlibNamedKind(t, "p.sysml", source.KindSysML,
		`package P {
			requirement Load { subject v : Anything; }
			requirement h { requirement r1; }
			requirement Spec { require Load; assume h.r1; require constraint c; }
			part def V { part y; variation part x { variant y; } }
		}`)
	targets := map[string][]string{}
	for _, owner := range []string{"P::Spec", "P::V::x"} {
		sym := nestedSym(t, root, owner)
		for _, membership := range m.OwnedMemberships(sym) {
			member := membership
			if membership.Implicit != nil && membership.Implicit.Member != nil {
				member = membership.Implicit.Member
			}
			target := ""
			if relationships := m.ownedImplicitRelationships(member, referenceSubsetting); len(relationships) == 1 {
				if _, general, ok := m.implicitRelationshipEnds(relationships[0]); ok && general != nil {
					target = general.Name
					var chain []string
					for _, feature := range m.chainingFeaturesOf(general) {
						chain = append(chain, feature.Name)
					}
					if len(chain) > 0 {
						target = strings.Join(chain, ".")
					}
				}
			}
			targets[owner] = append(targets[owner], target)
		}
	}
	want := map[string][]string{"P::Spec": {"Load", "h.r1", ""}, "P::V::x": {"y", ""}}
	for owner, expected := range want {
		got := targets[owner]
		if len(got) != len(expected) {
			t.Fatalf("%s has reference subsettings %q, want %q", owner, got, expected)
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Errorf("%s member %d reference-subsets %q, want %q", owner, i, got[i], expected[i])
			}
		}
	}
}
