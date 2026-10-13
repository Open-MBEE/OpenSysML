package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessOperations covers operation calls: unhandled operations, arguments of
// the wrong type and parameters left unbound.
func TestRuntimeRobustnessOperations(t *testing.T) {
	t.Run("call_of_unhandled_operation", testCallOfUnhandledOperation)
	t.Run("call_argument_of_wrong_type", testCallArgumentOfWrongType)
	t.Run("operation_invoked_with_unbound_parameters", testOperationInvokedWithUnboundParameters)
}

// testCallOfUnhandledOperation: an invocation no trigger names is discarded by
// run-to-completion, leaving the machine where it was rather than hanging.
func testCallOfUnhandledOperation(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state waiting;
			state moving;
			succession first init then waiting;
			transition first waiting accept go() then moving;
		}
	}`)
	exec.InvokeOperation("halt", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	current, ok := exec.CurrentState().(*ast.StateNode)
	if !ok || current.Name != "waiting" {
		t.Errorf("expected the unhandled call to leave the machine in waiting, got %v", exec.CurrentState())
	}
}

// testCallArgumentOfWrongType: an argument the guard cannot compare reports
// rather than firing or dropping the transition on a wrong comparison.
func testCallArgumentOfWrongType(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state waiting;
			state moving;
			succession first init then waiting;
			transition first waiting accept setSpeed(value) if value > 0 then moving;
		}
	}`)
	exec.InvokeOperation("setSpeed", map[string]Value{
		"value": NewStringValue("fast"),
	})
	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected an error: the guard compares a String argument with 0")
	}
	if !strings.Contains(err.Error(), "string") {
		t.Errorf("expected the offending operand kind in the message, got: %v", err)
	}
}

// testOperationInvokedWithUnboundParameters: an operation invoked without a value
// for a parameter, or with an argument naming none, is reported rather than run
// against values the invocation never stated.
func testOperationInvokedWithUnboundParameters(t *testing.T) {
	src := `
	package test {
		part def Adder {
			attribute total : Integer = 0;
			action add {
				in addend : Integer[1];
				assign total := total + addend;
			}
		}
	}`
	ctx, inst, err := instantiateInSource(t, src, "test::Adder")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	if _, err := ctx.InvokeOperation(inst, "add", nil); !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("invoke without an argument = %v, want ErrUnboundParameter", err)
	}
	args := map[string]Value{"addend": integerValue(1), "extra": integerValue(2)}
	if _, err := ctx.InvokeOperation(inst, "add", args); !errors.Is(err, ErrUnboundParameter) {
		t.Errorf("invoke with an unknown argument = %v, want ErrUnboundParameter", err)
	}
	if _, err := ctx.InvokeOperation(inst, "missing", nil); !errors.Is(err, ErrNoSuchBehavior) {
		t.Errorf("invoke of an unknown operation = %v, want ErrNoSuchBehavior", err)
	}
	if _, err := ctx.InvokeOperation(inst, "total", nil); !errors.Is(err, ErrNotABehavior) {
		t.Errorf("invoke of an attribute = %v, want ErrNotABehavior", err)
	}
}
