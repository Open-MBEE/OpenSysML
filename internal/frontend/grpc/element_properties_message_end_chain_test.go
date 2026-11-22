package grpc

import "testing"

// A message end's chain resolves where the message is written: a sibling named as a feature
// the message inherits (Actions::Action::loops) is the sibling (SysML.xtext MessageEventMember).
func TestElementPropertiesChainAMessageEndThroughAShadowingSibling(t *testing.T) {
	r := parseForElementProperties(t, `package P {
	item def M;
	part def L;
	part def A { event occurrence sent; }
	action def V {
		part a : A;
		part loops : L { event occurrence held; }
		message m of M from a.sent to loops.held;
	}
}`)
	for chaining, want := range map[string]string{
		"P__V__m_pend1_pchain_pfc0": "P::V::loops",
		"P__V__m_pend1_pchain_pfc1": "P::V::loops::held",
	} {
		property := r.property(chaining, "chainingFeature")
		if len(property.Values) != 1 || property.Values[0].GetElement().GetQualifiedName() != want {
			t.Errorf("%s.chainingFeature = %+v, want %s", chaining, property, want)
		}
	}
	if inherited := r.property("P__V__m_pend1", "inheritedFeature"); inherited.Unsupported != "" || len(inherited.Values) == 0 {
		t.Errorf("the end's inheritedFeature = %+v, want its inherited features", inherited)
	}
}
