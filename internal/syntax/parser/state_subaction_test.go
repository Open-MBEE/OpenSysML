package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// An entry/do/exit action can be given by reference to an action declared
// elsewhere — `StateActionUsage : … | PerformedActionUsage ActionBody` with the
// reference-subsetting form of PerformActionUsageDeclaration (SysML.xtext,
// /* STATES */) — which parses to a performed action usage carrying a
// `references` relationship, exactly like the `perform <ref>;` spelling.
func TestParseStateSubactionByReference(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		target string
	}{
		{"entry", "state def S { action a; state s { entry a; } }", "a"},
		{"do", "state def S { action a; state s { do a; } }", "a"},
		{"exit", "state def S { action a; state s { exit a; } }", "a"},
		{"qualified", "state def S { state s { entry P::a; } }", "P::a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := stateSubactionActions(t, tt.src)
			if len(actions) != 1 {
				t.Fatalf("expected 1 action, got %d", len(actions))
			}
			usage, ok := actions[0].(*ast.Usage)
			if !ok {
				t.Fatalf("expected *ast.Usage, got %T", actions[0])
			}
			if usage.Kind != ast.UsageAction {
				t.Errorf("usage kind = %v, want action", usage.Kind)
			}
			if usage.HasBody {
				t.Error("reference without a body reports HasBody")
			}
			if len(usage.Relationships) != 1 || usage.Relationships[0].Kind != ast.RelReferences {
				t.Fatalf("expected a single reference subsetting, got %v", usage.Relationships)
			}
			qn, ok := usage.Relationships[0].Target.(*ast.QualifiedName)
			if !ok {
				t.Fatalf("expected a qualified name target, got %T", usage.Relationships[0].Target)
			}
			if got := qualifiedText(qn); got != tt.target {
				t.Errorf("reference target = %q, want %q", got, tt.target)
			}
		})
	}
}

// A referenced action may carry an invocation body binding its parameters.
func TestParseStateSubactionByReferenceWithBody(t *testing.T) {
	actions := stateSubactionActions(t, "state def S { action a; state s { entry a { in level = 1; } } }")
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	usage, ok := actions[0].(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage, got %T", actions[0])
	}
	if !usage.HasBody || len(usage.Members) != 1 {
		t.Fatalf("expected the invocation body to be kept, got HasBody=%v members=%d", usage.HasBody, len(usage.Members))
	}
}

// A redefinition after the reference belongs to the same declaration:
// `FeatureSpecializationPart?` of PerformActionUsageDeclaration.
func TestParseStateSubactionByReferenceRedefines(t *testing.T) {
	actions := stateSubactionActions(t, "state def S { action a; action b; state s { entry a :>> b; } }")
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	usage, ok := actions[0].(*ast.Usage)
	if !ok {
		t.Fatalf("expected *ast.Usage, got %T", actions[0])
	}
	if len(usage.Relationships) != 2 {
		t.Fatalf("expected reference and redefinition relationships, got %v", usage.Relationships)
	}
	if usage.Relationships[1].Kind != ast.RelRedefines {
		t.Errorf("second relationship = %v, want redefines", usage.Relationships[1].Kind)
	}
}

// stateSubactionActions parses src and returns the actions of the entry, do or
// exit member found in the nested state's body.
func stateSubactionActions(t *testing.T, src string) []ast.Node {
	t.Helper()
	for _, member := range stateDefMembers(t, src) {
		usage, ok := member.(*ast.Usage)
		if !ok || usage.Kind != ast.UsageState {
			continue
		}
		for _, inner := range usage.Members {
			if mem, ok := inner.(*ast.Membership); ok {
				inner = mem.Member
			}
			switch sub := inner.(type) {
			case *ast.EntryMember:
				return sub.Actions
			case *ast.DoMember:
				return sub.Actions
			case *ast.ExitMember:
				return sub.Actions
			}
		}
	}
	t.Fatalf("no entry/do/exit member in %q", src)
	return nil
}

// qualifiedText renders a qualified name as `A::B`.
func qualifiedText(qn *ast.QualifiedName) string {
	text := ""
	for i, part := range qn.Parts {
		if i > 0 {
			text += "::"
		}
		text += part.Text
	}
	return text
}
