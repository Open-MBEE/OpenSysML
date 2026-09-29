package runtime

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/edit"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

const actionBodyAuthoringSource = `package Demo {
    private import ScalarValues::*;
    attribute def Signal { attribute value : Integer; }
    action def A {
        out result : Integer = 0;
        first start;%s
    }
}
`

func actionBodyEditModel(t *testing.T, content string) edit.Model {
	t.Helper()
	sf := source.New("action-body.sysml", []byte(content))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument(sf.Name(), root)
	return edit.Model{
		Source: sf, Root: root, Index: idx,
		SemDiags: passes.Analyze(sf.Name(), root, nil, idx),
		NewIndex: func() *symbols.Index { return libs.NewModelIndex() },
	}
}

func plainActionBodyItem(op edit.Operation) edit.Operation {
	op.SequenceKeyword = ""
	return op
}

func TestActionBodyStatementsMatchWrittenNotationAndExecution(t *testing.T) {
	tests := []struct {
		name    string
		ops     []edit.Operation
		written string
	}{
		{
			name: "send to self and accept",
			ops: []edit.Operation{
				edit.AddSend("Demo::A", "new Signal(value = 7)", "self", ""),
				edit.AddAccept("Demo::A", "msg", "Signal", ""),
				edit.AddAssign("Demo::A", "result", "msg.value"),
			},
			written: `
        then send new Signal(value = 7) to self;
        then accept msg : Signal;
        then assign result := msg.value;`,
		},
		{
			name: "nested if and else",
			ops: []edit.Operation{
				edit.AddIf("Demo::A", "result == 0", []edit.Operation{
					plainActionBodyItem(edit.AddIf("", "result == 0", []edit.Operation{
						plainActionBodyItem(edit.AddAssign("", "result", "1")),
					}, []edit.Operation{
						plainActionBodyItem(edit.AddAssign("", "result", "2")),
					})),
				}, []edit.Operation{
					plainActionBodyItem(edit.AddAssign("", "result", "3")),
				}),
			},
			written: `
        then if result == 0 {
            if result == 0 {
                assign result := 1;
            } else {
                assign result := 2;
            }
        } else {
            assign result := 3;
        }`,
		},
		{
			name: "guarded and default branches",
			ops: []edit.Operation{
				edit.AddGuardedThen("Demo::A", "result == 0", "done"),
				edit.AddElse("Demo::A", "done"),
			},
			written: `
        if result == 0 then done;
        else done;`,
		},
		{
			name: "while until",
			ops: []edit.Operation{
				edit.AddWhile("Demo::A", "result < 2", []edit.Operation{
					plainActionBodyItem(edit.AddAssign("", "result", "result + 1")),
				}, "result == 2"),
			},
			written: `
        then while result < 2 {
            assign result := result + 1;
        } until result == 2;`,
		},
		{
			name: "loop until",
			ops: []edit.Operation{
				edit.AddLoop("Demo::A", []edit.Operation{
					plainActionBodyItem(edit.AddAssign("", "result", "result + 1")),
				}, "result == 2"),
			},
			written: `
        then loop {
            assign result := result + 1;
        } until result == 2;`,
		},
		{
			name: "for over sequence",
			ops: []edit.Operation{
				edit.AddFor("Demo::A", "i", "", "(1, 2, 3)", []edit.Operation{
					plainActionBodyItem(edit.AddAssign("", "result", "result + i")),
				}),
			},
			written: `
        then for i in (1, 2, 3) {
            assign result := result + i;
        }`,
		},
		{
			name: "terminate",
			ops: []edit.Operation{
				edit.AddAssign("Demo::A", "result", "1"),
				edit.AddTerminate("Demo::A", ""),
				edit.AddAssign("Demo::A", "result", "9"),
			},
			written: `
        then assign result := 1;
        then terminate;
        then assign result := 9;`,
		},
		{
			name: "source multiplicities",
			ops: []edit.Operation{
				func() edit.Operation {
					op := edit.AddAssign("Demo::A", "result", "1")
					op.Multiplicity = "[1]"
					return op
				}(),
				func() edit.Operation {
					op := edit.AddThen("Demo::A", "done")
					op.Multiplicity = "[1]"
					return op
				}(),
			},
			written: `
        then [1] assign result := 1;
        [1] then done;`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			direct := fmt.Sprintf(actionBodyAuthoringSource, test.written)
			base := fmt.Sprintf(actionBodyAuthoringSource, "")
			edited, err := edit.Apply(actionBodyEditModel(t, base), test.ops)
			if err != nil {
				t.Fatalf("edit.Apply: %v", err)
			}
			if got := string(edited.Content); got != direct {
				t.Fatalf("edited notation\n%s\nwant direct notation\n%s", got, direct)
			}
			run := func(content string) (Outcome, string) {
				t.Helper()
				ctx, idx, scope := sequenceContext(t, content)
				trace := NewTraceRecorder()
				ctx.SetTrace(trace)
				action := namedOrFoundSymbol(t, idx, "Demo::A", scope, ast.DefAction, ast.UsageAction)
				outputs, err := ctx.ExecuteAction(action)
				if err != nil {
					t.Fatalf("ExecuteAction: %v", err)
				}
				return ctx.ActionOutcome(outputs), trace.String()
			}
			editedOutcome, editedTrace := run(string(edited.Content))
			directOutcome, directTrace := run(direct)
			if editedTrace != directTrace {
				t.Fatalf("traces differ\n=== EDITED ===\n%s\n=== DIRECT ===\n%s", editedTrace, directTrace)
			}
			if got, want := editedOutcome.String(), directOutcome.String(); got != want {
				t.Fatalf("outcome = %s, want %s", got, want)
			}
		})
	}
}

func TestGuardedAndDefaultActionBranchesExecuteLikeWritten(t *testing.T) {
	for _, tc := range []struct {
		name  string
		guard string
	}{
		{name: "guarded", guard: "result == 0"},
		{name: "default", guard: "result < 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := []edit.Operation{
				edit.AddThenMember("Demo::A", "decide", "choice", ""),
				edit.AddGuardedThen("Demo::A", tc.guard, "done"),
				edit.AddElse("Demo::A", "done"),
			}
			direct := fmt.Sprintf(actionBodyAuthoringSource,
				"\n        then decide choice;\n        if "+tc.guard+" then done;\n        else done;")
			edited, err := edit.Apply(
				actionBodyEditModel(t, fmt.Sprintf(actionBodyAuthoringSource, "")), ops,
			)
			if err != nil {
				t.Fatalf("edit.Apply: %v", err)
			}
			if got := string(edited.Content); got != direct {
				t.Fatalf("edited notation\n%s\nwant direct notation\n%s", got, direct)
			}
			run := func(content string) (Outcome, string) {
				t.Helper()
				ctx, idx, scope := sequenceContext(t, content)
				trace := NewTraceRecorder()
				ctx.SetTrace(trace)
				action := namedOrFoundSymbol(t, idx, "Demo::A", scope, ast.DefAction, ast.UsageAction)
				outputs, err := ctx.ExecuteAction(action)
				if err != nil {
					t.Fatalf("ExecuteAction: %v", err)
				}
				return ctx.ActionOutcome(outputs), trace.String()
			}
			editedOutcome, editedTrace := run(string(edited.Content))
			directOutcome, directTrace := run(direct)
			if editedTrace != directTrace {
				t.Fatalf("traces differ\n=== EDITED ===\n%s\n=== DIRECT ===\n%s", editedTrace, directTrace)
			}
			if got, want := editedOutcome.String(), directOutcome.String(); got != want {
				t.Fatalf("outcome = %s, want %s", got, want)
			}
		})
	}
}
