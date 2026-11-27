package passes

import (
	"fmt"
	"testing"
)

// A `for` loop's variable is a feature of its declared type, so a chain through it reads that type's members.
func TestForLoopVariableReadsItsDeclaredType(t *testing.T) {
	const model = `package Repro {
    part def Component {
        action run;
        attribute mass : ScalarValues::Real;
    }
    part def Assembly {
        part components : Component [1..*];
        attribute total : ScalarValues::Real;
        %s
    }
}`
	for _, member := range []string{
		`action runAll { for component : Component in components { perform component.run; } }`,
		`action runAll for component : Component in components { perform component.run; }`,
		`action runAll { for component : Component in components { assign total := component.mass; } }`,
		`action runAll { for component : Component in components { action nested { perform component.run; } } }`,
	} {
		wantLibraryClean(t, fmt.Sprintf(model, member))
	}
	diags := libraryDiags(t, fmt.Sprintf(model, `action runAll { for component : Component in components { perform component.nosuch; } }`))
	if len(diags) != 1 || diags[0].Source != "name-resolution" || diags[0].Message != "unresolved member: nosuch" {
		t.Errorf("component.nosuch: got %v, want one unresolved-member diagnostic naming nosuch", diags)
	}
}
