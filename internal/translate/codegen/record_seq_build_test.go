package codegen_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

// TestCRecordSequencesBuildUnderStrictDeclarations: a calc holding a record sequence instantiates
// the collection runtime over records, whose argument parser gcc 15 refused as an implicit declaration.
func TestCRecordSequencesBuildUnderStrictDeclarations(t *testing.T) {
	cc := os.Getenv(codegen.CCompilerEnvVar)
	if cc == "" {
		cc = "cc"
	}
	if _, err := exec.LookPath(cc); err != nil {
		t.Skip("no C compiler on PATH")
	}
	for _, flag := range []string{"-Werror=implicit-function-declaration", "-Werror=int-conversion"} {
		found := false
		for _, f := range codegen.CFlags {
			found = found || f == flag
		}
		if !found {
			t.Errorf("CFlags lacks %s, so a missing runtime declaration compiles on an older gcc", flag)
		}
	}
	compiler, pkg := recordCompiler(t)
	program, err := compiler.Compile(recordCalc(t, pkg, "HoldsMany"), codegen.TargetC)
	if err != nil {
		t.Fatal(err)
	}
	if err := codegen.Build(program, codegen.TargetC, filepath.Join(t.TempDir(), "records")); err != nil {
		t.Fatalf("the generated C does not build under CFlags: %v", err)
	}
}
