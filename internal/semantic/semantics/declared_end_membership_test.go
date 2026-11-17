package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A declared `end` feature is owned through a FeatureMembership whose member isEnd; only a connector's clause
// ends are EndFeatureMemberships — as the pilot exports both (SysML.xtext UsageMember, KerML.xtext ConnectorEndMember).
func TestDeclaredEndsAreFeatureMemberships(t *testing.T) {
	const sysml = `package P {
    port def OpticalPort;
    part def Observe; part def Subsystem;
    interface def OpticalInterface { end source : OpticalPort; end target : OpticalPort; }
    allocation def FunctionToPart { end function : Observe; end part : Subsystem; }
    connection def Link { end a : Observe; end b : Subsystem; }
    part def PD { end part ep : Observe; part q : Observe; }
    part p { part o : Observe; part s : Subsystem; connection l : Link { end x : Observe; } connection c connect o to s; }
}`
	const kerml = `package K {
    classifier X;
    assoc A { end feature a : X; end feature b : X; }
    classifier C { end feature e : X; feature f : X; }
    feature x : X; feature y : X;
    connector k2 { end feature ka references x; end feature kb references y; }
    struct S { feature a : X; feature b : X; connector c from a to b; succession s first a then b; }
}`
	type want struct {
		owner string
		kinds string
	}
	check := func(t *testing.T, name string, kind source.Kind, src string, wants []want, clause string) {
		for _, fixture := range reflectiveFixtures(t, name, kind, src) {
			model := fixture.model
			for _, w := range wants {
				owner := nestedSym(t, fixture.root, w.owner)
				if owner.Recorded() {
					continue
				}
				var kinds []string
				for _, membership := range model.OwnedMemberships(owner) {
					member := membership.Implicit.Member
					if member == nil || !member.IsFeature() {
						continue
					}
					isEnd, _ := model.ReflectiveFeatureValue(member, "isEnd")
					kinds = append(kinds, relMetaName(model, membership)+boolMark(isEnd.Kind == symbols.FilterValueBool && isEnd.Bool))
				}
				if got := strings.Join(kinds, " "); got != w.kinds {
					t.Errorf("%s owns %q, want %q", w.owner, got, w.kinds)
				}
			}
			connector := nestedSym(t, fixture.root, clause)
			if connector.Recorded() {
				continue
			}
			for _, membership := range model.OwnedMemberships(connector) {
				if member := membership.Implicit.Member; member != nil && member.Kind == symbols.SymbolConnectorEnd {
					if got := relMetaName(model, membership); got != "EndFeatureMembership" {
						t.Errorf("%s's clause end membership = %s, want EndFeatureMembership", clause, got)
					}
				}
			}
		}
	}
	check(t, "declared-ends.sysml", source.KindSysML, sysml, []want{
		{"P::OpticalInterface", "FeatureMembership* FeatureMembership*"},
		{"P::FunctionToPart", "FeatureMembership* FeatureMembership*"},
		{"P::Link", "FeatureMembership* FeatureMembership*"},
		{"P::PD", "FeatureMembership* FeatureMembership"},
		{"P::p::l", "FeatureMembership*"},
	}, "P::p::c")
	check(t, "declared-ends.kerml", source.KindKerML, kerml, []want{
		{"K::A", "FeatureMembership* FeatureMembership*"},
		{"K::C", "FeatureMembership* FeatureMembership"},
		{"K::k2", "FeatureMembership* FeatureMembership*"},
	}, "K::S::c")
}

func boolMark(isEnd bool) string {
	if isEnd {
		return "*"
	}
	return ""
}
