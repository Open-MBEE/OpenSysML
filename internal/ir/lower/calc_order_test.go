package lower

import (
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

func calcOrderFrom(t *testing.T, text string) *StatementOrder {
	t.Helper()
	p := parser.New(source.New("calc.sysml", []byte(text)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	for _, member := range root.Members {
		def, ok := unwrapMembership(member).(*ast.Definition)
		if ok && def.Kind == ast.DefCalc {
			body, order := CalcBodyWithOrder(def, def.Members, nil, nil)
			if len(CalcSteps(body)) != 5 {
				t.Fatalf("calc body has %d steps, want a local, three assignments, and the result", len(CalcSteps(body)))
			}
			return order
		}
	}
	t.Fatal("no calc definition")
	return nil
}

func TestCalcStatementOrderThenAndUnorderedSteps(t *testing.T) {
	order := calcOrderFrom(t, `
		calc def C {
			attribute y : Integer := 1;
			assign y := y * 2 + 1;
			then assign y := y * 3 + 2;
			assign y := y + 4;
			y
		}
	`)
	got := orders(order)
	want := map[string]bool{"01234": true, "01324": true, "03124": true}
	if len(got) != len(want) {
		t.Fatalf("reached %v, want three admitted orders", got)
	}
	for _, run := range got {
		var key string
		for _, step := range run {
			key += fmt.Sprint(step)
		}
		if !want[key] {
			t.Errorf("reached %v, which violates succession precedence or is duplicated", run)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Errorf("did not reach %v", want)
	}
}

func TestCalcNestedStatementOrderThen(t *testing.T) {
	p := parser.New(source.New("nested-calc.sysml", []byte(`
		calc def C {
			attribute y : Integer := 1;
			if true {
				assign y := y * 2;
				then assign y := y + 1;
			}
			attribute i : Integer := 0;
			while i < 1 {
				assign y := y * 3;
				then assign y := y + 1;
				assign i := i + 1;
			}
			y
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	var def *ast.Definition
	for _, member := range root.Members {
		if candidate, ok := unwrapMembership(member).(*ast.Definition); ok && candidate.Kind == ast.DefCalc {
			def = candidate
			break
		}
	}
	if def == nil {
		t.Fatal("no calc definition")
	}
	body, _ := CalcBodyWithOrder(def, def.Members, nil, nil)
	var ifStmt, loopStmt Statement
	for _, stmt := range body {
		switch stmt.(type) {
		case If:
			ifStmt = stmt
		case Loop:
			loopStmt = stmt
		}
	}
	ifStmtBody, ok := ifStmt.(If)
	if !ok || ifStmtBody.Then.Order == nil {
		t.Fatalf("if body order = %#v, want lowered order", ifStmtBody.Then.Order)
	}
	if got := orders(ifStmtBody.Then.Order); len(got) != 1 {
		t.Fatalf("if body reaches %v, want its succession to fix the order", got)
	}
	loop, ok := loopStmt.(Loop)
	if !ok || loop.Body.Order == nil {
		t.Fatalf("loop body order = %#v, want lowered order", loop.Body.Order)
	}
	if got := orders(loop.Body.Order); len(got) != 1 {
		t.Fatalf("loop body reaches %v, want commuting statements to add no order", got)
	}
}

func TestCalcNestedBlocksCarryTheirOwnPrecedence(t *testing.T) {
	p := parser.New(source.New("nested-blocks.sysml", []byte(`
		calc def C {
			attribute y : Integer := 0;
			if true {
				assign y := y + 1;
				then assign y := y + 2;
			} else {
				assign y := y + 3;
				then assign y := y + 4;
			}
			while false {
				assign y := y + 5;
				then assign y := y + 6;
			}
			for i in 1..2 {
				action note {
					assign y := y + 7;
					then assign y := y + 8;
				}
			}
			for j in 1..2 action note {
				assign y := y + 9;
				then assign y := y + 10;
			}
			y
		}
	`)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	var def *ast.Definition
	for _, member := range root.Members {
		if candidate, ok := unwrapMembership(member).(*ast.Definition); ok && candidate.Kind == ast.DefCalc {
			def = candidate
			break
		}
	}
	if def == nil {
		t.Fatal("no calc definition")
	}
	body, order := CalcBodyWithOrder(def, def.Members, nil, nil)
	checked := 0
	assertOrder := func(label string, members []ast.Node, stmts []Statement, order *StatementOrder) {
		t.Helper()
		if order == nil {
			t.Fatalf("%s has no lowered statement order", label)
		}
		if order.Len() != len(stmts) {
			t.Fatalf("%s order has %d entries for %d statements", label, order.Len(), len(stmts))
		}
		before, matched := statementPrecedence(members, stmts)
		hasSuccession := false
		for _, member := range members {
			if _, ok := unwrapMembership(member).(*ast.SuccessionEdge); ok {
				hasSuccession = true
				break
			}
		}
		if hasSuccession && len(matched) == 0 {
			t.Fatalf("%s did not match its succession to statements", label)
		}
		for i := range before {
			for j, precedes := range before[i] {
				if precedes && !order.before[i][j] {
					t.Fatalf("%s order omitted precedence %d before %d", label, i, j)
				}
			}
		}
		for _, sequence := range orders(order) {
			positions := make(map[int]int, len(sequence))
			for position, statement := range sequence {
				positions[statement] = position
			}
			for i := range order.before {
				for j, precedes := range order.before[i] {
					if precedes && positions[i] >= positions[j] {
						t.Fatalf("%s admits %v, violating statement %d before %d", label, sequence, i, j)
					}
				}
			}
		}
		checked++
	}
	var walkStatements func(string, []Statement)
	var walkGraph func(string, *ActionGraph)
	walkBlock := func(label string, block Block, members []ast.Node) {
		t.Helper()
		if block.Order == nil {
			t.Fatalf("%s has no lowered statement order", label)
		}
		if block.Graph == nil {
			assertOrder(label, members, block.Statements, block.Order)
		} else if steps, ok := block.Graph.StatementList(); ok && block.Stated {
			assertOrder(label, members, steps, block.Order)
		}
		if block.Graph != nil {
			walkGraph(label, block.Graph)
			return
		}
		walkStatements(label, block.Statements)
	}
	walkGraph = func(label string, graph *ActionGraph) {
		for _, node := range graph.Nodes {
			stmts := graph.Bodies[node]
			if len(stmts) > 0 {
				nodeMembers := ast.NodeBodyMembers(node)
				if usage, ok := node.(*ast.Usage); ok {
					nodeMembers = usage.Members
				}
				assertOrder(label+" action body", nodeMembers, stmts, graph.StatementOrders[node])
				walkStatements(label+" action body", stmts)
			}
			if subflow := graph.Subflows[node]; subflow != nil && subflow.Graph != nil {
				if steps, ok := subflow.Graph.StatementList(); ok {
					nodeMembers := ast.NodeBodyMembers(node)
					if usage, ok := node.(*ast.Usage); ok {
						nodeMembers = usage.Members
					}
					assertOrder(label+" subflow", nodeMembers, steps, graph.StatementOrders[node])
				}
				walkGraph(label+" subflow", subflow.Graph)
			}
		}
	}
	walkStatements = func(label string, stmts []Statement) {
		for i, stmt := range stmts {
			switch nested := stmt.(type) {
			case If:
				node, ok := nested.Node.(*ast.IfActionNode)
				if !ok {
					t.Fatalf("%s if has node %T", label, nested.Node)
				}
				if node.Then != nil {
					walkBlock(fmt.Sprintf("%s if %d", label, i), nested.Then, node.Then.Body)
				}
				if node.Else != nil && nested.Else != nil {
					walkBlock(fmt.Sprintf("%s else %d", label, i), *nested.Else, node.Else.Body)
				}
			case Loop:
				node, ok := nested.Node.(*ast.WhileLoopActionNode)
				if !ok {
					t.Fatalf("%s loop has node %T", label, nested.Node)
				}
				walkBlock(fmt.Sprintf("%s loop %d", label, i), nested.Body, node.Body)
			case Block:
				if node, ok := nested.Node.(*ast.Usage); ok {
					walkBlock(fmt.Sprintf("%s action %d", label, i), nested, node.Members)
				}
			}
		}
	}
	assertOrder("calc body", def.Members, body, order)
	walkStatements("calc body", body)
	if checked < 7 {
		t.Fatalf("checked %d statement lists, want each nested list", checked)
	}
}
