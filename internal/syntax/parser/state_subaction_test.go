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

// A braced block — `entry { … }`, `do { … }`, `exit { … }` and a transition's
// `do { … }` — is one anonymous action usage, the tree its `action` spelling
// gives, with the kind keyword unwritten so the spelling stays recoverable.
func TestParseBracedStateBlockIsOneAnonymousAction(t *testing.T) {
	tests := []struct {
		name, braced, spelled, prefix string
		members                       int
	}{
		{"entry", "entry { attribute k : Integer = 2; assign x := k; send x to r; }", "entry action { attribute k : Integer = 2; assign x := k; send x to r; }", "entry", 3},
		{"do", "do { assign x := 1; assign x := 2; }", "do action { assign x := 1; assign x := 2; }", "do", 2},
		{"exit", "exit { assign x := 1; }", "exit action { assign x := 1; }", "exit", 1},
		{"entry_do", "entry do { assign x := 1; }", "entry action { assign x := 1; }", "entry", 1},
		{"empty_entry", "entry { }", "entry action { }", "entry", 0},
		{"empty_do", "do { }", "do action { }", "do", 0},
		{"empty_exit", "exit { }", "exit action { }", "exit", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrap := func(body string) string {
				return "state def S { attribute x; part r; state s { " + body + " } }"
			}
			braced := stateSubactionActions(t, wrap(tt.braced))
			spelled := stateSubactionActions(t, wrap(tt.spelled))
			if len(braced) != 1 {
				t.Fatalf("expected the one anonymous action, got %d nodes", len(braced))
			}
			usage := anonymousActionOf(t, braced[0])
			if usage.PrefixKeyword != tt.prefix || usage.Keyword != "" {
				t.Errorf("prefix=%q keyword=%q, want prefix=%q and no kind keyword", usage.PrefixKeyword, usage.Keyword, tt.prefix)
			}
			if !usage.HasBody || len(usage.Members) != tt.members {
				t.Errorf("HasBody=%v members=%d, want a body of %d members", usage.HasBody, len(usage.Members), tt.members)
			}
			if got, want := ast.Dump(braced[0]), ast.Dump(spelled[0]); got != want {
				t.Errorf("braced block tree differs from its action spelling:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// A transition's `do { … }` is likewise the one anonymous action `do action { … }` declares.
func TestParseBracedTransitionEffectIsOneAnonymousAction(t *testing.T) {
	tests := []struct {
		name, braced, spelled string
		members               int
	}{
		{"statements", "do { assign x := 1; send x to r; }", "do action { assign x := 1; send x to r; }", 2},
		{"empty", "do { }", "do action { }", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrap := func(effect string) string {
				return "state def S { attribute x; part r; state a; state b; transition first a accept x " + effect + " then b; }"
			}
			braced := transitionEffectOf(t, wrap(tt.braced))
			spelled := transitionEffectOf(t, wrap(tt.spelled))
			if !braced.HasEffect || len(braced.Effect) != 1 {
				t.Fatalf("expected the one anonymous effect action, got HasEffect=%v and %d nodes", braced.HasEffect, len(braced.Effect))
			}
			usage := anonymousActionOf(t, braced.Effect[0])
			if usage.PrefixKeyword != "" || usage.Keyword != "" {
				t.Errorf("prefix=%q keyword=%q, want neither", usage.PrefixKeyword, usage.Keyword)
			}
			if !usage.HasBody || len(usage.Members) != tt.members {
				t.Errorf("HasBody=%v members=%d, want a body of %d members", usage.HasBody, len(usage.Members), tt.members)
			}
			if got, want := ast.Dump(braced.Effect[0]), ast.Dump(spelled.Effect[0]); got != want {
				t.Errorf("braced effect tree differs from its action spelling:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// anonymousActionOf returns the nameless action usage the membership holds.
func anonymousActionOf(t *testing.T, node ast.Node) *ast.Usage {
	t.Helper()
	member, ok := node.(*ast.Membership)
	if !ok {
		t.Fatalf("expected *ast.Membership, got %T", node)
	}
	usage, ok := member.Member.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageAction || usage.Ident.Name != "" {
		t.Fatalf("expected an anonymous action usage, got %s", ast.Dump(member.Member))
	}
	return usage
}

// transitionEffectOf parses src and returns the transition found in the state
// definition's body.
func transitionEffectOf(t *testing.T, src string) *ast.TransitionMember {
	t.Helper()
	for _, member := range stateDefMembers(t, src) {
		if transition, ok := member.(*ast.TransitionMember); ok {
			return transition
		}
	}
	t.Fatalf("no transition in %q", src)
	return nil
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
