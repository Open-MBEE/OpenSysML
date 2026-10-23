package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// beforeSchemaProblems reports a before list on a case other than an action
// performed to completion by no object, the only kind a prior performance precedes.
func beforeSchemaProblems(expected ExpectedOutcome) []string {
	if len(expected.Before) > 0 && (expected.Type != "action" || len(expected.Performers) > 0 || expected.Error != "") {
		return []string{"before applies to an action case run to completion by no object"}
	}
	return nil
}

// performBefore performs the actions a case lists before its own on the same runtime,
// as a later `-action` follows an earlier; a prior performance of its action must yield its outputs.
func performBefore(t *testing.T, ctx *Context, idx *symbols.Index, rootScope *symbols.Scope, actionSym *symbols.Symbol, expected ExpectedOutcome) {
	t.Helper()
	for _, name := range expected.Before {
		sym := namedOrFoundSymbol(t, idx, name, rootScope, ast.DefAction, ast.UsageAction)
		outcome, err := ctx.ActionOutcomePerformedBy(sym, nil, nil)
		if err != nil {
			t.Fatalf("performance of %s before the case's: %v", name, err)
		}
		if sym == actionSym {
			validateOutputs(t, ctx, expected.Outputs, outcome.Outputs)
		}
	}
}

// runActionPerformers performs an action once per object performing it and
// checks each performance against what that object expects: its outputs, the
// feature values the body left on the object, or the error it must fail with.
func runActionPerformers(t *testing.T, ctx *Context, idx *symbols.Index, actionSym *symbols.Symbol, performers []Performer) {
	t.Helper()
	for _, performer := range performers {
		self := performerObject(t, ctx, idx, performer.Object)
		t.Run(performer.Object, func(t *testing.T) {
			outcome, err := ctx.ActionOutcomePerformedBy(actionSym, self, nil)
			if performer.Error != "" {
				requireError(t, "performance by "+performer.Object, err, performer.Error)
				return
			}
			if err != nil {
				t.Fatalf("performance by %s failed: %v", performer.Object, err)
			}
			validateTerminated(t, outcome.Terminated, performer.Terminated)
			validateOutputs(t, ctx, performer.Outputs, outcome.Outputs)
			for path, expectedVal := range performer.Slots {
				fv, err := featureValueAtPath(t, ctx, self, path)
				if err != nil {
					t.Errorf("feature value %q of %s: %v", path, performer.Object, err)
					continue
				}
				value, err := fv.ReadValue(path)
				if err != nil {
					t.Errorf("feature value %q of %s: %v", path, performer.Object, err)
					continue
				}
				validateValue(t, ctx, path, expectedVal, value)
			}
		})
	}
}

// The trace runner honours before as the conformance runner does: the trace of
// fall performed after fall is the repeated performance's, not the first's.
func TestTraceOfPerformanceAfterAnotherIsItsOwn(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "conformance", "state_space_performed_twice.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	traceOf := func(before ...string) string {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, string(src)))
		rootScope := idx.DocumentRoot("<test>")
		sym := namedOrFoundSymbol(t, idx, "test::fall", rootScope, ast.DefAction, ast.UsageAction)
		trace := NewTraceRecorder()
		traceAction(t, ctx, idx, rootScope, trace, sym, ExpectedOutcome{Type: "action", Evaluate: "test::fall", Before: before})
		return trace.String()
	}
	first, repeated := traceOf(), traceOf("test::fall")
	if first == repeated {
		t.Fatalf("the trace after a prior performance repeats the first's:\n%s", first)
	}
	if a, b := strings.Count(first, "\n"), strings.Count(repeated, "\n"); a != b {
		t.Errorf("the prior performance left %d lines in the trace of the repeated one (%d against %d)", b-a, b, a)
	}
}
