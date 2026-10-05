package lower

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func TestConstraintBodyStatementOrderThen(t *testing.T) {
	p := parser.New(source.New("constraint.sysml", []byte(`
		constraint def C {
			attribute y : Integer := 1;
			assign y := y * 10;
			then assign y := y + 2;
			y == 12
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	var body []ast.Node
	for _, member := range root.Members {
		def, ok := unwrapMembership(member).(*ast.Definition)
		if ok && def.Kind == ast.DefConstraint {
			body = def.Members
			break
		}
	}
	if body == nil {
		t.Fatal("no constraint definition")
	}
	var stmts []Statement
	var members []ast.Node
	for _, member := range body {
		if stmt, ok := ConstraintStep(member, nil); ok {
			stmts = append(stmts, stmt)
			members = append(members, member)
		}
	}
	stmts, order := ConstraintBodyWithOrder(nil, members, stmts)
	if len(stmts) != 3 {
		t.Fatalf("lowered %d body steps, want the declaration and two assignments", len(stmts))
	}
	if order.Len() != len(stmts) || !order.before[1][2] {
		t.Fatalf("lowered order = %#v, want assignment 1 before assignment 2", order)
	}
	if got := orders(order); len(got) != 1 {
		t.Fatalf("reached %v, want the stated succession to admit one order", got)
	}
}

func TestConstraintNestedBlocksCarryTheirOwnPrecedence(t *testing.T) {
	p := parser.New(source.New("constraint.sysml", []byte(`
		constraint def C {
			attribute x : Integer := 0;
			if true {
				assign x := x + 1;
				then assign x := x + 2;
			} else {
				assign x := x + 3;
				then assign x := x + 4;
			}
			while false {
				assign x := x + 5;
				then assign x := x + 6;
			}
			for i in 1..2 {
				assign x := x + i;
				then assign x := x + 1;
			}
			x >= 0
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	var def *ast.Definition
	for _, member := range root.Members {
		if candidate, ok := unwrapMembership(member).(*ast.Definition); ok && candidate.Kind == ast.DefConstraint {
			def = candidate
			break
		}
	}
	if def == nil {
		t.Fatal("no constraint definition")
	}
	var stmts []Statement
	for _, member := range def.Members {
		if stmt, ok := ConstraintStep(member, nil); ok {
			stmts = append(stmts, stmt)
		}
	}
	stmts, order := ConstraintBodyWithOrder(nil, def.Members, stmts)
	checked := 0
	var checkedLabels []string
	assertOrder := func(label string, members []ast.Node, statements []Statement, got *StatementOrder) {
		t.Helper()
		if got == nil || got.Len() != len(statements) {
			t.Fatalf("%s has order %v for %d statements", label, got, len(statements))
		}
		before, matched := statementPrecedence(members, statements)
		for i := range before {
			for j, precedes := range before[i] {
				if precedes && !got.before[i][j] {
					t.Fatalf("%s omitted precedence %d before %d", label, i, j)
				}
			}
		}
		for _, sequence := range orders(got) {
			positions := make(map[int]int, len(sequence))
			for position, statement := range sequence {
				positions[statement] = position
			}
			for i := range got.before {
				for j, precedes := range got.before[i] {
					if precedes && positions[i] >= positions[j] {
						t.Fatalf("%s admits %v, violating %d before %d", label, sequence, i, j)
					}
				}
			}
		}
		for _, member := range members {
			if _, succession := unwrapMembership(member).(*ast.SuccessionEdge); succession && len(matched) == 0 {
				t.Fatalf("%s did not match its succession to statements", label)
			}
		}
		checked++
		checkedLabels = append(checkedLabels, label)
	}
	var walk func(string, []ast.Node, []Statement, *StatementOrder)
	walk = func(label string, members []ast.Node, statements []Statement, got *StatementOrder) {
		assertOrder(label, members, statements, got)
		for i, stmt := range statements {
			switch nested := stmt.(type) {
			case If:
				node := nested.Node.(*ast.IfActionNode)
				if node.Then != nil {
					walk(fmt.Sprintf("%s if %d", label, i), node.Then.Body, nested.Then.Steps(), nested.Then.Order)
				}
				if node.Else != nil && nested.Else != nil {
					walk(fmt.Sprintf("%s else %d", label, i), node.Else.Body, nested.Else.Steps(), nested.Else.Order)
				}
			case Loop:
				node := nested.Node.(*ast.WhileLoopActionNode)
				walk(fmt.Sprintf("%s loop %d", label, i), node.Body, nested.Body.Steps(), nested.Body.Order)
			case Block:
				node := nested.Node.(*ast.Usage)
				walk(fmt.Sprintf("%s action %d", label, i), node.Members, nested.Steps(), nested.Order)
			}
		}
	}
	walk("constraint body", def.Members, stmts, order)
	if checked < 5 {
		t.Fatalf("checked %d statement lists %v, want nested if/else/while/for lists", checked, checkedLabels)
	}
}
