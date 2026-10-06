package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// TestRuntimeRobustnessStateIsActive covers the boundaries of `x.isActive`: a
// write is refused, a non-state operand or a missing import leaves it
// unresolved, and a machine not yet started or already ended reads false.
func TestRuntimeRobustnessStateIsActive(t *testing.T) {
	t.Run("write_is_refused", testStateIsActiveWriteRefused)
	t.Run("non_state_operand_is_unresolved", testStateIsActiveNonStateOperand)
	t.Run("without_the_import_is_unresolved", testStateIsActiveWithoutImport)
	t.Run("before_start_and_after_end_read_false", testStateIsActiveOutsideTheRun)
}

const stateIsActiveWriteFixture = `
	package test {
		private import ScalarValues::*;
		private import StateActivity::*;
		state def M {
			entry; then a;
			state a {
				entry assign a.isActive := true;
			}
		}
	}
`

func testStateIsActiveWriteRefused(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, stateIsActiveWriteFixture))
	m := findSymbolByName(idx.DocumentRoot("<test>"), "M", ast.DefState)
	_, _, err := ctx.ExecuteStateWithEvents(m, nil)
	if !errors.Is(err, ErrReadOnlyFeature) || !strings.Contains(err.Error(), "a.isActive is derived") {
		t.Fatalf("assign a.isActive := true: %v, want ErrReadOnlyFeature naming the derived feature", err)
	}
}

const stateIsActiveNonStateFixture = `
	package test {
		private import ScalarValues::*;
		private import StateActivity::*;
		state def M {
			attribute n : Integer = 0;
			entry; then a;
			state a {
				entry assign n := if n.isActive ? 1 else 0;
			}
		}
	}
`

// A feature chain's member is the extension feature only when the operand is a
// state: an Integer attribute has no isActive, as the name-resolution tier
// reports, and the run refuses the read rather than answering false.
func testStateIsActiveNonStateOperand(t *testing.T) {
	wantUnresolvedIsActive(t, stateIsActiveNonStateFixture)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, stateIsActiveNonStateFixture))
	m := findSymbolByName(idx.DocumentRoot("<test>"), "M", ast.DefState)
	if _, _, err := ctx.ExecuteStateWithEvents(m, nil); err == nil || !strings.Contains(err.Error(), "cannot chain through non-instance member") {
		t.Fatalf("n.isActive on an Integer: %v, want the chain read refused", err)
	}
}

const stateIsActiveWithoutImportFixture = `
	package test {
		private import ScalarValues::*;
		state def M {
			attribute n : Integer = 0;
			entry; then a;
			state a {
				entry assign n := if a.isActive ? 1 else 0;
			}
		}
	}
`

// Without the StateActivity import the member is unresolved, exactly as the
// standard library leaves it: the standard diagnostic, and ErrUnresolvedReference
// from the run.
func testStateIsActiveWithoutImport(t *testing.T) {
	wantUnresolvedIsActive(t, stateIsActiveWithoutImportFixture)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, stateIsActiveWithoutImportFixture))
	m := findSymbolByName(idx.DocumentRoot("<test>"), "M", ast.DefState)
	_, _, err := ctx.ExecuteStateWithEvents(m, nil)
	if !errors.Is(err, ErrUnresolvedReference) || !strings.Contains(err.Error(), "a has no member isActive") {
		t.Fatalf("a.isActive without the import: %v, want ErrUnresolvedReference", err)
	}
}

// wantUnresolvedIsActive asserts the name-resolution tier reports isActive as an
// unresolved member of the fixture and nothing else as an error.
func wantUnresolvedIsActive(t *testing.T, src string) {
	t.Helper()
	file := parseAndBuild(t, src)
	idx := libs.NewModelIndex()
	idx.AddDocument("<test>", file)
	idx.ExpandWildcardImports()
	diags := passes.Analyze("<test>", file, nil, idx)
	var unresolved int
	for _, d := range diags {
		if d.Severity != diag.SeverityError {
			continue
		}
		if !strings.Contains(d.Message, "unresolved member: isActive") {
			t.Errorf("unexpected error %s", d.Message)
			continue
		}
		unresolved++
	}
	if unresolved != 1 {
		t.Fatalf("got %d `unresolved member: isActive` errors in %+v, want 1", unresolved, diags)
	}
}

const stateIsActiveOutsideFixture = `
	package test {
		private import ScalarValues::*;
		private import StateActivity::*;
		item def Go;
		state def M {
			entry; then a;
			state a;
			transition first a accept Go then done;
		}
	}
`

// A state is active only while its machine runs: a.isActive reads false before
// the machine's executor has started and again once the machine has completed.
func testStateIsActiveOutsideTheRun(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, stateIsActiveOutsideFixture))
	m := findSymbolByName(idx.DocumentRoot("<test>"), "M", ast.DefState)
	read := func(when string) {
		t.Helper()
		expr, ok := parser.ParseOneExpression("<expr>", "a.isActive")
		if !ok {
			t.Fatal("parse a.isActive")
		}
		val, err := ctx.EvalWithScope(expr, m.Scope)
		if err != nil {
			t.Fatalf("%s: a.isActive: %v", when, err)
		}
		if val.Kind != ValConst || val.Const.Kind != semantics.ValBool || val.Const.Bool {
			t.Errorf("%s: a.isActive = %s, want false", when, describeValue(val))
		}
	}
	exec, err := ctx.CreateStateExecutor(m)
	if err != nil {
		t.Fatalf("CreateStateExecutor: %v", err)
	}
	if exec.Activity(nil) {
		t.Error("Activity(nil) = true, want false")
	}
	read("before start")
	if _, _, err := ctx.ExecuteStateWithEvents(m, []string{"Go"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	read("after end")
}
