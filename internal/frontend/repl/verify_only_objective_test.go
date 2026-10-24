package repl

import "testing"

// TestVerifyOnlyObjectiveIsDecidedByTheBody checks that an objective stating
// only `verify r;` is satisfied, so the case's status is its body's verdict.
func TestVerifyOnlyObjectiveIsDecidedByTheBody(t *testing.T) {
	s := loadSource(t, statedSatisfyModel)
	wants(t, run(t, s, "%analysis ProductPkg::costTest"),
		"✓ ProductPkg::costTest", "objective costObjective: satisfied", "✓ Verification ProductPkg::costTest verdict: pass")
	wants(t, run(t, s, "%analysis ProductPkg::costlyTest"),
		"✗ ProductPkg::costlyTest", "objective costObjective: satisfied", "✗ Verification ProductPkg::costlyTest verdict: fail")
}
