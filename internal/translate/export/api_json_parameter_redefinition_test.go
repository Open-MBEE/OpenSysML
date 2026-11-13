package export

import (
	"slices"
	"testing"
)

const parameterRedefinitionModel = `package Probe {
	action def TakePicture { in scene; out picture; }
	action takePicture : TakePicture { in s; out p; }
	action mismatched : TakePicture { out q; }
}`

// A step's parameter redefines the general behavior's parameter at its position (SysML
// 8.3.6.4, KerML 8.4.4.6), so the redefined one leaves the step's member lists as it does
// in the pilot — whatever the directions: a disagreeing pair is a violation, not inheritance.
func TestAPIJSONPositionalParameterRedefinitionLeavesTheGeneralParameterOut(t *testing.T) {
	graph := duplicateGraph(t, parameterRedefinitionModel, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	step := elements["Probe__takePicture"]
	for _, property := range []string{"feature", "usage", "directedUsage", "input", "output", "member", "membership", "inheritedMembership", "inheritedFeature", "directedFeature"} {
		ids := referenceIDs(t, step, property)
		for _, redefined := range []string{"Probe__TakePicture__scene", "Probe__TakePicture__picture", "Probe__TakePicture__scene_om", "Probe__TakePicture__picture_om"} {
			if slices.Contains(ids, redefined) {
				t.Errorf("takePicture.%s = %v lists %s, which its own parameter redefines by position", property, ids, redefined)
			}
		}
	}
	if ids := referenceIDs(t, step, "input"); !slices.Equal(ids, []string{"Probe__takePicture__s"}) {
		t.Errorf("takePicture.input = %v, want its own s only", ids)
	}
	if ids := referenceIDs(t, step, "output"); !slices.Equal(ids, []string{"Probe__takePicture__p"}) {
		t.Errorf("takePicture.output = %v, want its own p only", ids)
	}
	mismatched := elements["Probe__mismatched"]
	if value, present := mismatched["input"]; present {
		t.Errorf("mismatched.input = %v, want none: its out at scene's position redefines scene", value)
	}
	if ids := referenceIDs(t, mismatched, "output"); !slices.Equal(ids, []string{"Probe__mismatched__q", "Probe__TakePicture__picture"}) {
		t.Errorf("mismatched.output = %v, want q and the inherited picture, which no owned parameter's position claims", ids)
	}
}
