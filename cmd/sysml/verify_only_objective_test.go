package main

import "testing"

// TestVerifyOnlyObjectiveExitsByTheBodyVerdict checks that -analysis of a
// verification case whose objective states only `verify r;` exits 0 when the
// body passes and 1 when it fails: the objective is satisfied, not undecided.
func TestVerifyOnlyObjectiveExitsByTheBodyVerdict(t *testing.T) {
	binary := buildCLI(t)
	wantReport(t, checkFiles(t, binary, statedSatisfyFiles, "-analysis", "ProductPkg::costTest"), 0,
		"✓ ProductPkg::costTest", "objective costObjective: satisfied",
		"✓ Verification ProductPkg::costTest verdict: pass")
	wantReport(t, checkFiles(t, binary, statedSatisfyFiles, "-analysis", "ProductPkg::costlyTest"), 1,
		"✗ ProductPkg::costlyTest", "objective costObjective: satisfied",
		"✗ Verification ProductPkg::costlyTest verdict: fail")
}
