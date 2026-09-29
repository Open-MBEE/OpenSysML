package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRuntimeRobustnessActionBodyStatements(t *testing.T) {
	t.Run("nested accept without a sender deadlocks with a typed error", func(t *testing.T) {
		content := `package Demo {
    private import ScalarValues::*;
    action def A {
        first start;
        then if true {
            accept msg : Integer;
        }
        then done;
    }
}
`
		ctx, idx, scope := sequenceContext(t, content)
		action := namedOrFoundSymbol(t, idx, "Demo::A", scope, ast.DefAction, ast.UsageAction)
		trace := NewTraceRecorder()
		ctx.SetTrace(trace)
		if _, err := ctx.ExecuteAction(action); !errors.Is(err, ErrAcceptDeadlock) {
			t.Fatalf("ExecuteAction error = %v, want ErrAcceptDeadlock; trace:\n%s", err, trace.String())
		}
	})
}
