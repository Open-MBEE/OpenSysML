package lower

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
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
	if evt, ok := payloadTrigger(payloadValue(t, "Triggers::TriggerWhen({ level > threshold })"), nil, nil).(*ast.ChangeEvent); !ok {
		t.Fatalf("TriggerWhen: got %T, want *ast.ChangeEvent", evt)
	} else if _, isOperator := evt.Condition.(*ast.OperatorExpr); !isOperator {
		t.Fatalf("TriggerWhen condition = %T, want the body's result expression", evt.Condition)
	}
	if evt, ok := payloadTrigger(payloadValue(t, "TriggerWhen(ready)"), nil, nil).(*ast.ChangeEvent); !ok {
		t.Fatalf("unqualified TriggerWhen: got %T, want *ast.ChangeEvent", evt)
	}
	if evt, ok := payloadTrigger(payloadValue(t, "TriggerAt(t0)"), nil, nil).(*ast.TimeEvent); !ok || !evt.Absolute {
		t.Fatalf("TriggerAt: got %T (absolute %v), want an absolute *ast.TimeEvent", evt, ok && evt.Absolute)
	}
	if evt, ok := payloadTrigger(payloadValue(t, "Triggers::TriggerAfter(30 [s])"), nil, nil).(*ast.TimeEvent); !ok || evt.Absolute {
		t.Fatalf("TriggerAfter: got %T, want a relative *ast.TimeEvent", evt)
	}
	for _, kept := range []string{"Other::TriggerWhen(ready)", "TriggerWhen(a, b)", "sensor.reading", "Signal()"} {
		value := payloadValue(t, kept)
		if got := payloadTrigger(value, nil, nil); got != value {
			t.Fatalf("payloadTrigger(%s) = %T, want the value itself", kept, got)
		}
	}
}

// payloadValueIn indexes `package P { <members> action def A { accept : ChangeSignal = <value>; } }`
// over the standard library and returns the payload's value with the scope of A
// and a resolver over the index.
func payloadValueIn(t *testing.T, members, value string) (ast.Node, *symbols.Scope, *resolve.Resolver) {
	t.Helper()
	src := "package P { " + members + " action def A { accept : ChangeSignal = " + value + "; } }"
	p := parser.New(source.New("p.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse errors: %v", p.Diagnostics)
	}
	idx := libs.NewModelIndex()
	idx.AddDocument("p.sysml", root)
	idx.ExpandWildcardImports()
	pkg, ok := idx.DocumentRoot("p.sysml").LookupLocal("P")
	if !ok {
		t.Fatal("package P not indexed")
	}
	a, ok := pkg.Scope.LookupLocal("A")
	if !ok {
		t.Fatal("action def A not indexed")
	}
	var found ast.Node
	ast.Inspect(a.Decl, func(n ast.Node) bool {
		if u, ok := n.(*ast.Usage); ok && u.IsAccept && u.Value != nil {
			found = u.Value
		}
		return found == nil
	})
	if found == nil {
		t.Fatalf("no accept payload value in %q", src)
	}
	return found, a.Scope, resolve.New(idx)
}

// With a resolver, only an invocation of the library's own Triggers function is
// the event: a function of the model's spelled the same, imported or in a
// package of the model named Triggers, keeps its value.
func TestPayloadTriggerResolvesLibraryFunction(t *testing.T) {
	events := []struct{ members, value string }{
		{"private import Triggers::*;", "TriggerWhen(ready)"},
		{"", "Triggers::TriggerWhen(ready)"},
		{"private import Triggers::TriggerAfter;", "TriggerAfter(30 [s])"},
	}
	for _, c := range events {
		value, scope, resolver := payloadValueIn(t, c.members, c.value)
		switch payloadTrigger(value, scope, resolver).(type) {
		case *ast.ChangeEvent, *ast.TimeEvent:
		default:
			t.Fatalf("payloadTrigger(%s) with %q = %T, want an event", c.value, c.members, value)
		}
	}
	kept := []struct{ members, value string }{
		{"function TriggerWhen { in ready; return r; }", "TriggerWhen(ready)"},
		{"package Triggers { function TriggerWhen { in ready; return r; } }", "Triggers::TriggerWhen(ready)"},
		{"", "TriggerWhen(ready)"},
	}
	for _, c := range kept {
		value, scope, resolver := payloadValueIn(t, c.members, c.value)
		if got := payloadTrigger(value, scope, resolver); got != value {
			t.Fatalf("payloadTrigger(%s) with %q = %T, want the value itself", c.value, c.members, got)
		}
	}
}
