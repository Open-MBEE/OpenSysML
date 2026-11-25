package main

import "testing"

// strictRedefinitionFile redefines a `length` HotLegPipe does not declare;
// the name resolves through an import, so only strict mode rejects it.
const strictRedefinitionFile = "../../tests/testdata/passes/strict_redefinition_resolves_to_nothing.sysml"

func TestStrictConformanceRejectsRedefinitionResolvingToNothing(t *testing.T) {
	binary := buildCLI(t)

	def := checkPaths(t, binary, "-validate", strictRedefinitionFile)
	wantReport(t, def, 0, ": no errors")
	rejectReport(t, def, "redefinition-resolves-to-nothing")

	strict := checkPaths(t, binary, "-validate", "-strict", strictRedefinitionFile)
	wantReport(t, strict, 2,
		"10:27: error[redefinition-resolves-to-nothing]: member length redefines nothing resolvable in HotLegPipe",
		"did not analyse cleanly")
	rejectReport(t, strict, "warning[", ": no errors")
}
