package queryexec

import (
	"testing"
)

const invalidBareActivityOperandFixture = `
calc def InvalidActivityOperand :> Query {
	in root : Element;
	WhereFeature(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "StateUsage", exact = true),
		'feature' = "isActive", operator = "=", value = "ready"
	)
}
`

// A native filter over state rows reads the Boolean isActive built-in alone,
// so a text operand is invalid: only a shadowing feature can defer its kind.
func TestQueryRejectsInvalidBareActivityOperandWithoutAShadow(t *testing.T) {
	fixture := loadExecutionFixture(t, stateActivityFixture+invalidBareActivityOperandFixture)
	_, err := fixture.execute(t, "InvalidActivityOperand", Bindings{
		"root": {ElementValue(fixture.symbol(t, "Machine"))},
	}, Options{})
	failure := executionError(t, err, ErrorInvalidArgument)
	if failure.Parameter != "value" || failure.Actual != "ready" {
		t.Fatalf("invalid operand error = %+v, want value ready", failure)
	}
}
