package main

import (
	"os"
	"path/filepath"
	"testing"
)

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

// A typed usage that also redefines a usage contributes the redefined usage's
// members to its nested declarations, so strict validation stays silent.
func TestStrictConformanceAcceptsMembersOfRedefinedTyping(t *testing.T) {
	const src = `package C {
		part def Wheel { attribute radius; }
		part def Car { part wheel : Wheel { attribute pressure; } }
		part def SportsCar :> Car { part :>> wheel : Wheel { attribute :>> pressure; } }
	}`
	path := filepath.Join(t.TempDir(), "sportscar.sysml")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := buildCLI(t)
	strict := checkPaths(t, binary, "-validate", "-strict", path)
	wantReport(t, strict, 0, ": no errors")
	rejectReport(t, strict, "redefinition-resolves-to-nothing")
}
