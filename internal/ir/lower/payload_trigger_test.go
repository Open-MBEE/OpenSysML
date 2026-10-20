package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// payloadValue parses `accept : ChangeSignal = <value>;` and returns the value
// the payload parameter was parsed with.
func payloadValue(t *testing.T, value string) ast.Node {
	t.Helper()
	src := "package P { action def A { accept : ChangeSignal = " + value + "; } }"
	p := parser.New(source.New("t.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	var found ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if u, ok := n.(*ast.Usage); ok && u.IsAccept && u.Value != nil {
			found = u.Value
		}
		return found == nil
	})
	if found == nil {
		t.Fatalf("no accept payload value in %q", src)
	}
	return found
}

// An accept payload valued by a library trigger invocation is the event the
// keyword form writes (SysML v2 §7.17.8): TriggerWhen a change event on its
// condition, TriggerAt and TriggerAfter time events; any other value is kept.
func TestPayloadTriggerInvocations(t *testing.T) {
	if evt, ok := payloadTrigger(payloadValue(t, "Triggers::TriggerWhen({ level > threshold })")).(*ast.ChangeEvent); !ok {
		t.Fatalf("TriggerWhen: got %T, want *ast.ChangeEvent", evt)
	} else if _, isOperator := evt.Condition.(*ast.OperatorExpr); !isOperator {
		t.Fatalf("TriggerWhen condition = %T, want the body's result expression", evt.Condition)
	}
	if evt, ok := payloadTrigger(payloadValue(t, "TriggerWhen(ready)")).(*ast.ChangeEvent); !ok {
		t.Fatalf("unqualified TriggerWhen: got %T, want *ast.ChangeEvent", evt)
	}
	if evt, ok := payloadTrigger(payloadValue(t, "TriggerAt(t0)")).(*ast.TimeEvent); !ok || !evt.Absolute {
		t.Fatalf("TriggerAt: got %T (absolute %v), want an absolute *ast.TimeEvent", evt, ok && evt.Absolute)
	}
	if evt, ok := payloadTrigger(payloadValue(t, "Triggers::TriggerAfter(30 [s])")).(*ast.TimeEvent); !ok || evt.Absolute {
		t.Fatalf("TriggerAfter: got %T, want a relative *ast.TimeEvent", evt)
	}
	for _, kept := range []string{"Other::TriggerWhen(ready)", "TriggerWhen(a, b)", "sensor.reading", "Signal()"} {
		value := payloadValue(t, kept)
		if got := payloadTrigger(value); got != value {
			t.Fatalf("payloadTrigger(%s) = %T, want the value itself", kept, got)
		}
	}
}
