package sysmlv1

import "testing"

// Eclipse UML2 serializes a package of the SysML profile without a URI of its own
// under the hostless nsURI http:///SysML/<path>.ecore; Cameo's "Eclipse UML2 (v5.x)
// XMI" export declares the standard profile's packages this way.
func TestUML2HostlessSysMLNamespaces(t *testing.T) {
	for _, ns := range []string{
		"http:///SysML/Activities.ecore",
		"http:///SysML/Allocations.ecore",
		"http:///SysML/Blocks.ecore",
		"http:///SysML/BuiltinOperations.ecore",
		"http:///SysML/ConstraintBlocks.ecore",
		"http:///SysML/DeprecatedElements.ecore",
		"http:///SysML/Libraries.ecore",
		"http:///SysML/Libraries/ControlValues.ecore",
		"http:///SysML/Libraries/PrimitiveValueTypes.ecore",
		"http:///SysML/Libraries/PrimitiveValueTypes/Deprecated.ecore",
		"http:///SysML/Libraries/UnitAndQuantityKind.ecore",
		"http:///SysML/Libraries/UnitAndQuantityKind/Deprecated.ecore",
		"http:///SysML/ModelElements.ecore",
		"http:///SysML/ModelElements/Deprecated.ecore",
		"http:///SysML/NonNormativeExtensions.ecore",
		"http:///SysML/NonNormativeExtensions/Activity.ecore",
		"http:///SysML/NonNormativeExtensions/Actors.ecore",
		"http:///SysML/NonNormativeExtensions/Block.ecore",
		"http:///SysML/NonNormativeExtensions/Diagrams.ecore",
		"http:///SysML/NonNormativeExtensions/Distributions.ecore",
		"http:///SysML/NonNormativeExtensions/Parametric.ecore",
		"http:///SysML/NonNormativeExtensions/Requirement.ecore",
		"http:///SysML/PortsFlows.ecore",
		"http:///SysML/Requirements.ecore",
		// Any package serialized under the SysML root is the profile's own.
		"http:///SysML/UserProfile.ecore",
	} {
		if !IsSysMLNamespace(ns) {
			t.Errorf("Eclipse UML2 SysML namespace not recognized: %s", ns)
		}
	}
	for _, ns := range []string{
		"https://evil.example/SysML/Blocks.ecore",
		"http://localhost/SysML/Blocks.ecore",
		"http://user@/SysML/Blocks.ecore",
		"https:///SysML/Blocks.ecore",
		"http:///Other/Blocks.ecore",
		"http:///SysMLx/Blocks.ecore",
		"http:///SysML.ecore",
		"http:///SysML/.ecore",
		"http:///SysML/Blocks",
		"http:///SysML/Blocks.ecore#fake",
		"http:///SysML/Blocks.ecore?fake",
		"http:///SysML/Blocks.ecore?",
		"http:///schemas/SysML/_id/0",
		"http:///Vendor/SysML/Blocks.ecore",
	} {
		if IsSysMLNamespace(ns) {
			t.Errorf("non-SysML namespace accepted as standard SysML: %s", ns)
		}
	}
}
