package semantics

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A control action written as a bare statement is of the kind its notation states, as the named
// form is (SysML 8.4.12, Table 32), and its parameters redefine those of that kind (Table 33).
func TestBareControlStatementsSpecializeTheirKind(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package Q {
	part def T {
		attribute x;
		action a {
			assign x := 1;
			for i in (1, 2) { }
			if true { }
			while true { }
		}
	}
	action def D {
		attribute y;
		assign y := 1;
	}
}`)
	want := [][]string{
		{"Actions::Action::assignments", "target -> Actions::Action::assignments::target", "replacementValues -> Actions::AssignmentAction::replacementValues"},
		{"Actions::Action::forLoops", "var -> Actions::ForLoopAction::var", "seq -> Actions::ForLoopAction::seq", "body -> Actions::ForLoopAction::body"},
		{"Actions::Action::ifSubactions", "ifTest -> Actions::IfThenAction::ifTest", "thenClause -> Actions::IfThenAction::thenClause"},
		{"Actions::Action::whileLoops", "whileTest -> Actions::WhileLoopAction::whileTest", "body -> Actions::WhileLoopAction::body"},
	}
	rows := func(owner string) [][]string {
		sym := nestedSym(t, root, owner)
		var nodes []*symbols.Symbol
		sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
			if member.Name == "" && controlStatementOf(member.Decl) != nil {
				nodes = append(nodes, member)
			}
			return true
		})
		var out [][]string
		for _, node := range append(nodes, m.statementNodes(sym)...) {
			var row []string
			for _, base := range m.ImpliedSpecializations(node) {
				row = append(row, m.QualifiedNameOf(base))
			}
			out = append(out, append(row, controlParameterRows(m, node)...))
		}
		return out
	}
	got := rows("Q::T::a")
	if len(got) != len(want) {
		t.Fatalf("a holds %d control statements, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("statement %d = %v, want %v", i, got[i], want[i])
		}
	}
	// Owned by a definition the same statement is a subaction too.
	if got := rows("Q::D"); len(got) != 1 || !slices.Equal(got[0], want[0]) {
		t.Errorf("D's assignment = %v, want %v", got, want[0])
	}
}

// An assignment written in the unnamed body of a loop is a subaction of that body, as one written
// in a declared action is (SysML 8.4.12, Table 32; SysML.xtext ActionBodyParameter), so its target
// redefines that of Action::assignments (Table 33).
func TestAssignmentsInUnnamedLoopBodiesAreSubactions(t *testing.T) {
	m, root := buildModelWithStdlib(t, `package Q {
	part def T {
		attribute x;
		action a {
			while true { assign x := 1; }
			for i in (1, 2) { assign x := 2; }
			loop { assign x := 3; } until true;
			action named while true { assign x := 4; }
		}
	}
}`)
	want := []string{"Actions::Action::assignments", "target -> Actions::Action::assignments::target", "replacementValues -> Actions::AssignmentAction::replacementValues"}
	a := nestedSym(t, root, "Q::T::a")
	loops := append(m.statementNodes(a), nestedSym(t, root, "Q::T::a::named"))
	if len(loops) != 4 {
		t.Fatalf("a holds %d loops, want 4", len(loops))
	}
	for i, loop := range loops {
		m.statementNodes(loop)
		var assignments []*symbols.Symbol
		for scope, owner := range m.statementScopes {
			if owner != loop {
				continue
			}
			scope.ForEachMember(func(member *symbols.Symbol) bool {
				if controlFormOf(member.Decl) == ControlAssignment {
					assignments = append(assignments, member)
				}
				return true
			})
		}
		if len(assignments) != 1 {
			t.Fatalf("loop %d holds %d assignments, want 1", i, len(assignments))
		}
		var got []string
		for _, base := range m.ImpliedSpecializations(assignments[0]) {
			got = append(got, m.QualifiedNameOf(base))
		}
		if got = append(got, controlParameterRows(m, assignments[0])...); !slices.Equal(got, want) {
			t.Errorf("loop %d: assignment = %v, want %v", i, got, want)
		}
	}
}
