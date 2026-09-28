package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestTransitionAcceptBareNameUsageTyping(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		rejects bool
	}{
		{
			name: "inherited start",
			src: `state def S {
	state A;
	state B;
	transition first A accept start then B;
}`,
			rejects: true,
		},
		{
			name: "inherited done",
			src: `state def S {
	state A;
	state B;
	transition first A accept done then B;
}`,
			rejects: true,
		},
		{
			name: "owned item usage",
			src: `item def Go;

state def S {
	state A;
	state B;
	item go : Go;
	transition first A accept go then B;
}`,
			rejects: true,
		},
		{
			name: "via port",
			src: `port def Wire;

state def S {
	state A;
	state B;
	port p : Wire;
	transition first A accept start via p then B;
}`,
			rejects: true,
		},
		{
			name: "nested state usage",
			src: `part def Owner {
	state running {
		state A;
		state B;
		transition first A accept running then B;
	}
}`,
			rejects: true,
		},
		{
			name: "item definition",
			src: `item def Go;

state def S {
	state A;
	state B;
	transition first A accept Go then B;
}`,
		},
		{
			name: "qualified item definition",
			src: `package Pkg {
	item def start;
}

state def S {
	state A;
	state B;
	transition first A accept Pkg::start then B;
}`,
		},
		{
			name: "named payload",
			src: `item def Go;

state def S {
	state A;
	state B;
	item g : Go;
	transition first A accept g : Go then B;
}`,
		},
		{
			name: "unresolved runtime event",
			src: `state def S {
	state A;
	state B;
	transition first A accept tick then B;
}`,
		},
		{
			name: "call event",
			src: `state def S {
	state A;
	state B;
	transition first A accept op() then B;
}`,
		},
		{
			name: "alias to item definition",
			src: `item def Go;
alias GoAlias for Go;

state def S {
	state A;
	state B;
	transition first A accept GoAlias then B;
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diags := libraryTypeDiags(t, tc.src)
			if !tc.rejects {
				if len(diags) != 0 {
					t.Fatalf("expected no type diagnostics, got %v", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("expected one type diagnostic, got %v", diags)
			}
			d := diags[0]
			if d.Severity != diag.SeverityError || d.Source != "type" || d.Code != "usage-typing" {
				t.Errorf("got severity %v, source %q, code %q", d.Severity, d.Source, d.Code)
			}
			if d.Message != msgUsageTyping {
				t.Errorf("got message %q, want %q", d.Message, msgUsageTyping)
			}
			wantSpan := transitionAcceptTriggerSpan(t, tc.src)
			if d.Span != wantSpan {
				t.Errorf("got span %+v, want qualified-name span %+v", d.Span, wantSpan)
			}
		})
	}
}

func transitionAcceptTriggerSpan(t *testing.T, src string) source.Span {
	t.Helper()
	root := parser.New(source.New("<test>", []byte(src))).ParseFile()
	var span source.Span
	ast.Inspect(root, func(node ast.Node) bool {
		transition, ok := node.(*ast.TransitionMember)
		if !ok {
			return true
		}
		qn, ok := transition.Trigger.(*ast.QualifiedName)
		if !ok {
			return true
		}
		span = qn.Span()
		return false
	})
	if span == (source.Span{}) {
		t.Fatal("fixture has no bare qualified-name transition trigger")
	}
	return span
}
