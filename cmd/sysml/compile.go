//go:build !sysml_prod && !sysml_nocodegen

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

func init() {
	codegenFeature.link(func(fs *flag.FlagSet) {
		fs.StringVar(&compileCalc, "compile", "", "Compile this calc def to a native executable named by -o, as -compile Pkg::Fib")
		fs.StringVar(&compileTarget, "target", "c", "Backend -compile generates code for: c or go")
		fs.BoolVar(&compileSource, "source", false, "With -compile, write the generated source to -o instead of building it")
	})
}

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
	program, err := repl.CompileCalc(sess, compileCalc, func(model *semantics.Model, resolver *resolve.Resolver, entry *symbols.Symbol) (*codegen.Program, error) {
		return codegen.New(model, resolver).Compile(entry, target)
	})
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
