package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

// runCompile compiles the calc -compile names into the executable, or with
// -source the source file, -o names.
func runCompile(files []string) error {
	target := codegen.Target(compileTarget)
	if !slices.Contains(codegen.Targets(), target) {
		return fmt.Errorf("unknown target %q; -target takes c or go", compileTarget)
	}
	if len(files) == 0 {
		return errors.New("no model to compile; name the file the calc is declared in, as `sysml model.sysml -compile Pkg::Fib -o fib`")
	}
	sess := newSession()
	report, err := sess.LoadPathsReport(files)
	if err != nil {
		return err
	}
	writeLines(os.Stderr, report.Loaded)
	writeLines(os.Stderr, report.Found)
	writeLines(os.Stderr, report.Declared)
	if report.Errors {
		return fmt.Errorf("%s did not analyse cleanly; nothing was compiled", strings.Join(files, ", "))
	}
	program, err := sess.CompileCalc(compileCalc)
	if err != nil {
		return err
	}
	if compileSource {
		src, err := codegen.Source(program, target)
		if err != nil {
			return err
		}
		return os.WriteFile(outputPath, src, 0o600)
	}
	return codegen.Build(program, target, outputPath)
}
