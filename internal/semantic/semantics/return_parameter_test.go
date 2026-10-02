package semantics

import "testing"

func TestResultParameterOwnerValid(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package P {
		use case def UC {}
		action def Inherited :> UC;
		action def Declared { return own : Integer; }
	}`)

	inherited := nestedSym(t, root, "P::Inherited")
	var foundCaseResult bool
	for _, param := range m.BehaviorParametersOf(inherited) {
		if !param.IsResult || param.Symbol.Name != "result" {
			continue
		}
		owner := param.Symbol.OwnerScope.Owner()
		if owner == nil || owner.Name != "Case" {
			t.Fatalf("inherited result owner = %v, want Cases::Case", owner)
		}
		if !ResultParameterOwnerValid(param.Symbol) {
			t.Fatal("Cases::Case::result should be valid when inherited by a use-case-specializing action")
		}
		foundCaseResult = true
		break
	}
	if !foundCaseResult {
		t.Fatal("inherited Cases::Case::result not found")
	}

	declared := nestedSym(t, root, "P::Declared")
	for _, param := range m.BehaviorParametersOf(declared) {
		if param.IsResult && param.Symbol.Name == "own" {
			if ResultParameterOwnerValid(param.Symbol) {
				t.Fatal("a return declared in an action def must have an invalid owner")
			}
			return
		}
	}
	t.Fatal("declared return parameter own not found")
}
