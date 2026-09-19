package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func parseF62(t *testing.T, src string) (*ast.RootNamespace, []Diagnostic) {
	t.Helper()
	p := New(source.New("f62.sysml", []byte(src)))
	root := p.ParseFile()
	return root, p.Diagnostics
}

// TestF62F63NodeBodiesParse locks the shapes the pilot's ActionBody-terminated
// productions allow: a body on any node, and a dotted transition end.
func TestF62F63NodeBodiesParse(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string // a substring of the AST dump the body must produce
	}{
		{
			"fork_body",
			"action def A { action a; then fork F { attribute local = 1; } }",
			`(ForkNode name="F"`,
		},
		{
			"join_body",
			"action def A { action a; then join J { attribute local = 1; } }",
			`(JoinNode name="J"`,
		},
		{
			"merge_body",
			"action def A { action a; then merge M { attribute local = 1; } }",
			`(MergeNode name="M"`,
		},
		{
			"quoted_decision_name",
			"action def A { decide 'test x'; }",
			`(DecisionNode name="test x")`,
		},
		{
			"initial_node_body",
			"action def A { first start { attribute local = 1; } }",
			`(InitialNode name="start"`,
		},
		{
			"typed_for_variable",
			"action def A { for n : ScalarValues::Integer in (1, 2, 3) { } }",
			`(WhileLoopActionNode kind="for" variable="n"`,
		},
		{
			"dotted_transition_target",
			"state def S { state a { state b; } transition first x then a.b; state x; }",
			`target="a.b"`,
		},
		{
			"transition_body",
			"state def S { attribute v = 0; state a; state b; transition first a then b { assign v := 1; } }",
			`(TransitionMember source="a" target="b"`,
		},
		{
			"send_body_payload",
			"action def A { part r; action s { send to r { in :>> payload = 1; } } }",
			`(SendStatement`,
		},
		{
			"send_via_port_to_receiver",
			"action def A { port p; part r; attribute x = 1; action s { send x via p to r; } }",
			`(SendStatement via=true`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, diags := parseF62(t, tt.src)
			for _, d := range diags {
				t.Errorf("unexpected diagnostic: %s", d.Message)
			}
			dump := ast.Dump(root)
			if !strings.Contains(dump, tt.want) {
				t.Errorf("dump does not contain %q:\n%s", tt.want, dump)
			}
		})
	}
}

// TestF62F63NodeBodyMembersAreRetained checks the members a node body declares
// reach the tree: a body that parses but drops its members is worse than the
// diagnostic it replaces.
func TestF62F63NodeBodyMembersAreRetained(t *testing.T) {
	src := `action def A {
		attribute v = 0;
		first start { assign v := 1; }
		then fork F { assign v := 2; }
		then join J { assign v := 3; }
	}`
	root, diags := parseF62(t, src)
	for _, d := range diags {
		t.Errorf("unexpected diagnostic: %s", d.Message)
	}
	dump := ast.Dump(root)
	if got := strings.Count(dump, "AssignmentActionNode"); got != 3 {
		t.Errorf("node bodies kept %d assignments, want 3:\n%s", got, dump)
	}
}

// TestF62F63Negative keeps the malformed forms the newly optional bodies could
// have made silently acceptable reported.
func TestF62F63Negative(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"fork_unclosed_body", "action def A { action a; then fork F { attribute local = 1; }"},
		{"fork_body_bad_member", "action def A { action a; then fork F { @@@ } }"},
		{"decide_no_name_or_end", "action def A { decide }"},
		{"initial_node_body_bad_member", "action def A { first start { @@@ } }"},
		{"for_typed_variable_no_type", "action def A { for n : in (1, 2) { } }"},
		{"for_typed_variable_no_sequence", "action def A { for n : ScalarValues::Integer in { } }"},
		{"transition_dotted_target_dangling", "state def S { state a; transition first a then a.; }"},
		{"transition_body_unclosed", "state def S { state a; state b; transition first a then b { "},
		{"send_via_no_port", "action def A { action s { send x via ; } }"},
		{"send_via_port_to_nothing", "action def A { port p; action s { send x via p to ; } }"},
		{"send_no_target_or_body", "action def A { action s { send x } }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := parseF62(t, tt.src)
			if len(diags) == 0 {
				t.Errorf("expected a diagnostic for %q", tt.src)
			}
		})
	}
}

// TestF62F63NoPanicOnTruncatedNodeBodies parses every prefix of each newly
// accepted form: the parser must return a tree and diagnostics, never panic.
func TestF62F63NoPanicOnTruncatedNodeBodies(t *testing.T) {
	sources := []string{
		"action def A { action a; then fork F { attribute local = 1; } }",
		"action def A { decide 'test x'; if x == 1 then A1; else A2; action A1; action A2; }",
		"action def A { for n : ScalarValues::Integer in (1, 2, 3) { assign v := n; } }",
		"state def S { state a { state b; } transition first a then a.b { assign v := 1; } }",
		"action def A { port p; part r; action s { send x via p to r { in :>> payload = 1; } } }",
	}
	for _, src := range sources {
		for i := 0; i <= len(src); i++ {
			prefix := src[:i]
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic on %q: %v", prefix, r)
					}
				}()
				p := New(source.New("truncated.sysml", []byte(prefix)))
				if p.ParseFile() == nil {
					t.Fatalf("nil tree for %q", prefix)
				}
			}()
		}
	}
}
