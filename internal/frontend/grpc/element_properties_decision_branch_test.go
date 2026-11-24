package grpc

import (
	"slices"
	"testing"
)

// A decision's branch (`if g then b;`, `else c;`) owns a succession whose target end
// references the branch's target through a ReferenceSubsetting (SysML.xtext TargetSuccession).
func TestElementPropertiesReadADecisionBranchTargetEnd(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    action def Act {
        action a;
        action b;
        action c;
        first a then d;
        decide d;
            if true then b;
            else c;
    }
}`)
	branches := map[string]string{}
	for _, feature := range propertyElementIDs(r.property("P__Act", "ownedFeature").Values) {
		answered := r.read(feature+"_succession", "target")
		if answered.Error != "" {
			continue
		}
		if targets := propertyElementIDs(answered.Properties[0].Values); len(targets) == 1 {
			branches[targets[0]] = feature + "_succession_pend1"
		}
	}
	for _, target := range []string{"P__Act__b", "P__Act__c"} {
		end, ok := branches[target]
		if !ok {
			t.Fatalf("no branch succession targets %s (%v)", target, branches)
		}
		subsettings := propertyElementIDs(r.property(end, "ownedReferenceSubsetting").Values)
		if len(subsettings) != 1 {
			t.Fatalf("%s owns reference subsettings %v, want one", end, subsettings)
		}
		if got := propertyElementIDs(r.property(subsettings[0], "referencedFeature").Values); !slices.Equal(got, []string{target}) {
			t.Errorf("%s references %v, want %s", end, got, target)
		}
	}
}
