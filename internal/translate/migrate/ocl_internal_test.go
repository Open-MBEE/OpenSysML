package migrate

import (
	"strings"
	"testing"
)

// The OCL subset of a «TableExpressionColumn» lowers to a cell expression
// over the row that reads the v2 features its v1 metaproperties migrated to.
func TestLowerOCL(t *testing.T) {
	m := &migration{}
	cases := []struct {
		name, src, rowKind, want, rowType string
	}{
		{"typed-by count", "_typedElementOfType->size()", "Class",
			`RelatedElements(source = row, relationshipKind = "typing", direction = "incoming", maxDepth = 1)->SequenceFunctions::size()`,
			"KerML::Core::Type"},
		{"bare name", "name", "", "row.name", "KerML::Root::Element"},
		{"self name", "self.name", "", "row.name", "KerML::Root::Element"},
		{"end types", "end.role.type->asSet()", "Connector",
			"row.connectorEnd->ControlFunctions::collect {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::last()}.type->Distinct()",
			"KerML::Kernel::Connector"},
		{"end types unknown rows", "end.role.type->asSet()", "",
			"row.connectorEnd->ControlFunctions::collect {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::last()}.type->Distinct()",
			"KerML::Kernel::Connector"},
		{"path through a part",
			"end.appliedStereotypeInstance.slot->select(x|x.definingFeature.name = 'propertyPath' and x.value.oclAsType(ElementValue).element->exists(y|y.oclAsType(Property).type.name = 'Bench')).value.oclAsType(ElementValue).element->select(z|z.oclAsType(Property).type.oclAsType(NamedElement).name <> 'Bench').oclAsType(NamedElement).name",
			"Connector",
			`row.connectorEnd->ControlFunctions::select {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::excluding(e.chainingFeature->SequenceFunctions::last())->ControlFunctions::exists {in y : KerML::Core::Feature; y.type.name == "Bench"}}->ControlFunctions::collect {in e1 : KerML::Core::Feature; e1.chainingFeature->SequenceFunctions::excluding(e1.chainingFeature->SequenceFunctions::last())}->ControlFunctions::select {in z : KerML::Core::Feature; z.type.name != "Bench"}.name`,
			"KerML::Kernel::Connector"},
		{"flow direction",
			"end.role.type.oclAsType(Class).member->select(y|y.appliedStereotypeInstance.classifier->any(x|x.name = 'FlowProperty')->size() <> 0).appliedStereotypeInstance.slot->select(x|x.oclAsType(Slot).definingFeature.name = 'direction').oclAsType(Slot).value.oclAsType(InstanceValue).instance.name->asSet()",
			"Connector",
			"row.connectorEnd->ControlFunctions::collect {in e : KerML::Core::Feature; e.chainingFeature->SequenceFunctions::last()}.type.member->ControlFunctions::select {in y : KerML::Core::Feature; y.direction->SequenceFunctions::notEmpty()}.direction->Distinct()",
			"KerML::Kernel::Connector"},
		{"block test", "ownedElement->select(p | p.appliedStereotypeInstance.classifier->exists(s | s.name = 'Block'))->size()", "Package",
			`row.ownedElement->ControlFunctions::select {in p : KerML::Root::Element; WhereType(source = p, type = ("PartDefinition"))->SequenceFunctions::notEmpty()}->SequenceFunctions::size()`,
			"KerML::Root::Element"},
		{"booleans", "not (name = 'a' or name <> 'b') and ownedElement->isEmpty()", "",
			`not (row.name == "a" or row.name != "b") and row.ownedElement->SequenceFunctions::isEmpty()`, "KerML::Root::Element"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, rowType, _, err := m.lowerOCL(tc.src, "", oclValue{text: "row", kind: tc.rowKind, single: true, variable: "row"})
			if err != nil {
				t.Fatalf("lowerOCL(%q): %v", tc.src, err)
			}
			if got != tc.want {
				t.Errorf("lowerOCL(%q)\n got %s\nwant %s", tc.src, got, tc.want)
			}
			if rowType != tc.rowType {
				t.Errorf("row type = %s, want %s", rowType, tc.rowType)
			}
		})
	}
}

// What the subset does not cover is refused with the construct quoted.
func TestLowerOCLRefuses(t *testing.T) {
	m := &migration{}
	cases := []struct{ src, rowKind, want string }{
		{"end.role", "Class", `"end" reads "end" of a Class, which no v2 feature stands for`},
		{"name->foo()", "", `"name->foo()" applies "foo", which is not a collection operation the lowering knows`},
		{"name.oclIsKindOf(String)", "", `"name.oclIsKindOf(String)" tests a metaclass`},
		{"owner->iterate(x; acc : Integer = 0 | acc)", "", `the character ";"`},
		{"name = ", "", "the expression ends where a value was expected"},
		{"end.appliedStereotypeInstance.slot->select(x | x.value->notEmpty()).value.oclAsType(ElementValue).element", "Connector",
			"selects slots without testing definingFeature.name against a string"},
		{"appliedStereotypeInstance.slot", "", "reads stereotype applications, which the v2 model carries as features"},
		{"name and true", "", `"name" is not a Boolean`},
	}
	for _, tc := range cases {
		_, _, _, err := m.lowerOCL(tc.src, "", oclValue{text: "row", kind: tc.rowKind, single: true, variable: "row"})
		if err == nil {
			t.Errorf("lowerOCL(%q) lowered; want a refusal mentioning %q", tc.src, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("lowerOCL(%q) = %q, want it to mention %q", tc.src, err.Error(), tc.want)
		}
	}
}
