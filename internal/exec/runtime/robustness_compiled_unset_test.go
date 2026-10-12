package runtime_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/codegen"
)

// TestRuntimeRobustnessCompiledUnset covers the boundary of a compiled
// program at an unset feature value: a scalar one leaves the program, while
// one held in a collection or read through is refused with a typed error.
func TestRuntimeRobustnessCompiledUnset(t *testing.T) {
	compiler, pkg := recordCompiler(t)
	for _, target := range []codegen.Target{codegen.TargetGo, codegen.TargetC} {
		t.Run(string(target), func(t *testing.T) {
			if _, err := compiler.Compile(recordCalc(t, pkg, "UnsetRead"), target); err != nil {
				t.Errorf("UnsetRead: %v; want it compiled", err)
			}
			for _, c := range []struct{ calc, want string }{
				{"UnsetInSeq", "a value that may be <unset> as an element of a sequence"},
				{"UnsetChain", "a feature chain reading x off a test::Point that may be <unset>"},
			} {
				_, err := compiler.Compile(recordCalc(t, pkg, c.calc), target)
				var unsupported *codegen.UnsupportedError
				if !errors.As(err, &unsupported) || !errors.Is(err, codegen.ErrUnsupported) || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v; want a typed refusal naming %q", c.calc, err, c.want)
				}
			}
		})
	}
}
