package sysmlv1

import "testing"

func TestCameoV5SysMLNamespaces(t *testing.T) {
	for _, ns := range []string{"http:///SysML/Activities.ecore", "http:///SysML/Allocations.ecore", "http:///SysML/Blocks.ecore", "http:///SysML/BuiltinOperations.ecore", "http:///SysML/ConstraintBlocks.ecore", "http:///SysML/DeprecatedElements.ecore", "http:///SysML/Libraries.ecore", "http:///SysML/Libraries/ControlValues.ecore", "http:///SysML/Libraries/PrimitiveValueTypes.ecore", "http:///SysML/Libraries/PrimitiveValueTypes/Deprecated.ecore", "http:///SysML/Libraries/UnitAndQuantityKind.ecore", "http:///SysML/Libraries/UnitAndQuantityKind/Deprecated.ecore", "http:///SysML/ModelElements.ecore", "http:///SysML/ModelElements/Deprecated.ecore", "http:///SysML/NonNormativeExtensions.ecore", "http:///SysML/NonNormativeExtensions/Activity.ecore", "http:///SysML/NonNormativeExtensions/Actors.ecore", "http:///SysML/NonNormativeExtensions/Block.ecore", "http:///SysML/NonNormativeExtensions/Diagrams.ecore", "http:///SysML/NonNormativeExtensions/Distributions.ecore", "http:///SysML/NonNormativeExtensions/Parametric.ecore", "http:///SysML/NonNormativeExtensions/Requirement.ecore", "http:///SysML/PortsFlows.ecore", "http:///SysML/Requirements.ecore"} {
		if !IsSysMLNamespace(ns) {
			t.Errorf("standard Cameo SysML namespace not recognized: %s", ns)
		}
	}
	for _, ns := range []string{"http:///SysML/UserProfile.ecore", "https://evil.example/SysML/Blocks.ecore", "http:///Other/Blocks.ecore", "http:///SysML/Blocks.ecore#fake", "http:///SysML/Blocks.ecore?fake"} {
		if IsSysMLNamespace(ns) {
			t.Errorf("unrecognized profile accepted as standard SysML: %s", ns)
		}
	}
}
