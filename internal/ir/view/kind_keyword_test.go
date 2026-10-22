package view

import "testing"

// A kind-keyed decision reads the keyword behind a written `abstract` or
// `variation`: the Cameo frame type, the palette family and the case and
// requirement node families are those of the plain kind.
func TestKindDecisionsReadTheKeywordBehindTheModifier(t *testing.T) {
	for kind, want := range map[string]string{
		"part def": "Block", "abstract part def": "Block", "variation part def": "Block",
		"state def": "State Machine", "abstract state def": "State Machine",
		"action def": "Activity", "abstract action def": "Activity",
		"abstract datatype": "Datatype", "abstract port def": "Port",
	} {
		if got := cameoFrameType(kind); got != want {
			t.Errorf("cameoFrameType(%q) = %q, want %q", kind, got, want)
		}
	}
	for kind, plain := range map[string]string{
		"abstract part def": "part def", "abstract state": "state", "variation action def": "action def", "abstract datatype": "datatype",
	} {
		if got, want := paletteFamily(kind), paletteFamily(plain); got != want {
			t.Errorf("paletteFamily(%q) = %q, want %q as for %q", kind, got, want, plain)
		}
	}
	for _, kind := range []string{"abstract use case def", "abstract analysis def", "variation verification def", "abstract requirement def", "abstract concern"} {
		if !caseNodeKind(kind) && !requirementNodeKind(kind) {
			t.Errorf("%q is neither a case nor a requirement node kind", kind)
		}
	}
	if !isDefinitionKind("abstract datatype") || isDefinitionKind("abstract part") {
		t.Error("isDefinitionKind reads the kind keyword behind abstract")
	}
}
