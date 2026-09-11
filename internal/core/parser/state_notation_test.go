package parser

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// stateDefMembers parses src and returns the members of its first state
// definition, with memberships unwrapped.
func stateDefMembers(t *testing.T, src string) []ast.Node {
	t.Helper()
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("expected no diagnostics, got %v", p.Diagnostics)
	}

	var members []ast.Node
	var walk func(nodes []ast.Node)
	walk = func(nodes []ast.Node) {
		for _, node := range nodes {
			if membership, ok := node.(*ast.Membership); ok {
				node = membership.Member
			}
			switch n := node.(type) {
			case *ast.Package:
				walk(n.Members)
			case *ast.Definition:
				if n.Kind == ast.DefState && members == nil {
					members = unwrapAll(n.Members)
				}
			case *ast.Usage:
				if n.Kind == ast.UsageState && members == nil {
					members = unwrapAll(n.Members)
				}
			}
		}
	}
	walk(root.Members)
	if members == nil {
		t.Fatal("no state declaration found")
	}
	return members
}

func unwrapAll(nodes []ast.Node) []ast.Node {
	out := make([]ast.Node, 0, len(nodes))
	for _, node := range nodes {
		if membership, ok := node.(*ast.Membership); ok {
			node = membership.Member
		}
		out = append(out, node)
	}
	return out
}

// The history keywords each produce the pseudostate kind they name, with
// `history` on its own meaning shallow history.
func TestHistoryPseudostateParsing(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	state def Controller {
		state Idle;
		history resume;
		shallow history resumeShallow;
		deep history resumeDeep;
	}
}`)

	got := make(map[string]ast.PseudostateKind)
	for _, member := range members {
		if ps, ok := member.(*ast.PseudostateNode); ok {
			got[ps.Name] = ps.Kind
		}
	}

	want := map[string]ast.PseudostateKind{
		"resume":        ast.PseudostateShallowHistory,
		"resumeShallow": ast.PseudostateShallowHistory,
		"resumeDeep":    ast.PseudostateDeepHistory,
	}
	for name, kind := range want {
		if got[name] != kind {
			t.Errorf("pseudostate %q: expected kind %v, got %v", name, kind, got[name])
		}
	}
	if len(got) != len(want) {
		t.Errorf("expected %d pseudostates, got %d: %v", len(want), len(got), got)
	}
}

// `defer` collects the events of one declaration in order, parsing each of them
// exactly like a transition trigger.
func TestDeferMemberParsing(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	state def Controller {
		defer Ping, setSpeed(value);
		defer Pong;
	}
}`)

	var defers []*ast.DeferMember
	for _, member := range members {
		if d, ok := member.(*ast.DeferMember); ok {
			defers = append(defers, d)
		}
	}
	if len(defers) != 2 {
		t.Fatalf("expected 2 defer members, got %d", len(defers))
	}
	if len(defers[0].Triggers) != 2 {
		t.Fatalf("expected the first defer to carry 2 events, got %d", len(defers[0].Triggers))
	}
	if name, ok := defers[0].Triggers[0].(*ast.QualifiedName); !ok || ast.SimpleName(name) != "Ping" {
		t.Errorf("expected the first deferred event to be the signal Ping, got %T", defers[0].Triggers[0])
	}
	call, ok := defers[0].Triggers[1].(*ast.CallEvent)
	if !ok {
		t.Fatalf("expected the second deferred event to be a call event, got %T", defers[0].Triggers[1])
	}
	if ast.SimpleName(call.Operation) != "setSpeed" {
		t.Errorf("expected the deferred call to be setSpeed, got %q", ast.SimpleName(call.Operation))
	}
	if len(defers[1].Triggers) != 1 {
		t.Errorf("expected the second defer to carry 1 event, got %d", len(defers[1].Triggers))
	}
}

// A statement written as a transition's `do` effect is terminated by the
// transition's own `then` clause, so no semicolon is expected before it.
func TestTransitionEffectStatementNeedsNoSemicolonBeforeThen(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	attribute def Warning;
	state def Controller {
		attribute level : Integer;
		state idle;
		state alerting;
		transition first idle accept w : Warning do assign level := 1 then alerting;
		transition first alerting accept w : Warning do perform notify then idle;
	}
}`)

	var transitions []*ast.TransitionMember
	for _, member := range members {
		if trans, ok := member.(*ast.TransitionMember); ok {
			transitions = append(transitions, trans)
		}
	}
	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}
	for i, want := range []string{"alerting", "idle"} {
		if len(transitions[i].Effect) != 1 {
			t.Errorf("transition %d: expected 1 effect, got %d", i, len(transitions[i].Effect))
		}
		if got := ast.SimpleName(transitions[i].Target); got != want {
			t.Errorf("transition %d: expected target %q, got %q", i, want, got)
		}
	}
	if _, ok := transitions[0].Effect[0].(*ast.AssignmentActionNode); !ok {
		t.Errorf("expected an assignment effect, got %T", transitions[0].Effect[0])
	}
	effect := transitions[1].Effect[0]
	if membership, ok := effect.(*ast.Membership); ok {
		effect = membership.Member
	}
	if usage, ok := effect.(*ast.Usage); !ok || usage.Keyword != "perform" {
		t.Errorf("expected a performed action effect, got %T", effect)
	}
}

// A transition is recognized wherever a transition can be written, not only
// directly inside a state body, and whether its ends are simple or qualified
// names.
func TestTransitionOutsideAStateBody(t *testing.T) {
	for _, src := range []string{
		`package Test { action def A { state s1; state s2; transition first s1 then s2; } }`,
		`package Test { state def S { state a; state b; } action def A { transition first S::a then S::b; } }`,
	} {
		p := New(source.New("test.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Errorf("%s: expected no diagnostics, got %v", src, p.Diagnostics)
			continue
		}
		var found bool
		var walk func([]ast.Node)
		walk = func(nodes []ast.Node) {
			for _, node := range nodes {
				if membership, ok := node.(*ast.Membership); ok {
					node = membership.Member
				}
				switch n := node.(type) {
				case *ast.Package:
					walk(n.Members)
				case *ast.Definition:
					walk(n.Members)
				case *ast.TransitionMember:
					found = true
					if ast.SimpleName(n.Source) == "" || ast.SimpleName(n.Target) == "" {
						t.Errorf("%s: transition ends were not read: %+v", src, n)
					}
				}
			}
		}
		walk(root.Members)
		if !found {
			t.Errorf("%s: no transition member was produced", src)
		}
	}
}

// `first` is optional before a transition's source, so the clauses that follow
// it read the same whether it was written or not (SysML `TransitionUsage`).
func TestTransitionSourceWithoutFirstKeyword(t *testing.T) {
	members := stateDefMembers(t, `
package Test {
	attribute def Warning;
	state def Controller {
		entry action 'initial';
		transition 'initial' then off;
		state off;
		state on;
		transition off accept w : Warning if true do perform notify then on;
	}
}`)

	var transitions []*ast.TransitionMember
	for _, member := range members {
		if trans, ok := member.(*ast.TransitionMember); ok {
			transitions = append(transitions, trans)
		}
	}
	if len(transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(transitions))
	}
	for i, want := range [][2]string{{"initial", "off"}, {"off", "on"}} {
		if got := ast.SimpleName(transitions[i].Source); got != want[0] {
			t.Errorf("transition %d: expected source %q, got %q", i, want[0], got)
		}
		if got := ast.SimpleName(transitions[i].Target); got != want[1] {
			t.Errorf("transition %d: expected target %q, got %q", i, want[1], got)
		}
	}
	if transitions[1].Trigger == nil || transitions[1].Guard == nil || len(transitions[1].Effect) != 1 {
		t.Errorf("expected the trigger, guard and effect to be read: %+v", transitions[1])
	}
	if transitions[0].Name != "" || transitions[1].Name != "" {
		t.Errorf("a bare source is the source, not the transition's name: %q, %q",
			transitions[0].Name, transitions[1].Name)
	}
}

// A transition whose target the parser could not read names no edge, so it is an
// error node: a TransitionMember with no target would be dereferenced when the
// machine is lowered.
func TestTargetlessTransitionIsAnErrorNode(t *testing.T) {
	members := stateDefMembersWithErrors(t, `state def S { state a; transition first a accept Ping; }`)
	for _, member := range members {
		if trans, ok := member.(*ast.TransitionMember); ok {
			t.Fatalf("expected no transition member, got one with target %v", trans.Target)
		}
	}
	var errors int
	for _, member := range members {
		if _, ok := member.(*ast.ErrorNode); ok {
			errors++
		}
	}
	if errors != 1 {
		t.Errorf("expected one error node among %v", members)
	}
}

// stateDefMembersWithErrors parses src, which is expected to be malformed, and
// returns the members of its first state definition.
func stateDefMembersWithErrors(t *testing.T, src string) []ast.Node {
	t.Helper()
	p := New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic for the malformed transition")
	}
	for _, node := range unwrapAll(root.Members) {
		if def, ok := node.(*ast.Definition); ok && def.Kind == ast.DefState {
			return unwrapAll(def.Members)
		}
	}
	t.Fatal("no state definition found")
	return nil
}
