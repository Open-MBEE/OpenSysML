package runtime

import (
	"testing"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

func TestActionExecutor_Creation(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Create minimal action symbol
	action := &symbols.Symbol{
		Name: "TestAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "TestAction"},
			Members: []ast.Node{},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	
	if exec.ctx != ctx {
		t.Error("expected context to be set")
	}
	
	if exec.action != action {
		t.Error("expected action symbol to be set")
	}
	
	if exec.state != StateReady {
		t.Errorf("expected StateReady, got %v", exec.state)
	}
}

func TestActionExecutor_GraphExtraction(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{Name: "end"}
	edge := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
	}
	
	action := &symbols.Symbol{
		Name: "TestAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "TestAction"},
			Members: []ast.Node{initial, final, edge},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	if len(exec.nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(exec.nodes))
	}
	
	successors, ok := exec.edges[initial]
	if !ok || len(successors) != 1 || successors[0] != final {
		t.Error("expected edge from initial to final")
	}
}

func TestActionExecutor_InitialNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Create action: initial → final
	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{Name: "end"}
	
	action := &symbols.Symbol{
		Name: "SimpleAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "SimpleAction"},
			Members: []ast.Node{
				initial,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	// Initialize should spawn token at initial node
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(exec.tokens))
	}
	
	token := exec.tokens[0]
	if token.Location != initial {
		t.Error("expected token at initial node")
	}
	
	if exec.state != StateRunning {
		t.Errorf("expected StateRunning, got %v", exec.state)
	}
}

func TestActionExecutor_FinalNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{Name: "end"}
	
	action := &symbols.Symbol{
		Name: "SimpleAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "SimpleAction"},
			Members: []ast.Node{
				initial,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	
	// Step from initial to final
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step initial: %v", err)
	}
	
	// Token should move to final
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token after step, got %d", len(exec.tokens))
	}
	
	if exec.tokens[0].Location != final {
		t.Error("expected token at final node")
	}
	
	// Step final node (should consume token and complete)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step final: %v", err)
	}
	
	if len(exec.tokens) != 0 {
		t.Errorf("expected 0 tokens after final, got %d", len(exec.tokens))
	}
	
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
}

func TestActionExecutor_ActionExecutionNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	action := &ast.ActionExecutionNode{
		Name:       "compute",
		Expression: &ast.LiteralInteger{Value: "42"},
	}
	final := &ast.FinalNode{Name: "end"}
	
	// Build action: initial → compute → final
	actionSym := &symbols.Symbol{
		Name: "ComputeAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "ComputeAction"},
			Members: []ast.Node{
				initial,
				action,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // start → compute
	
	// Step compute node
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step compute: %v", err)
	}
	
	// Token should have result in data
	token := exec.tokens[0]
	result, ok := token.Data["result"]
	if !ok {
		t.Error("expected result in token data")
	}
	
	// Check result value
	if result.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", result.Kind)
	}
	
	if result.Const.Kind != semantics.ValInt || result.Const.Int != 42 {
		t.Errorf("expected integer 42, got %v", result.Const)
	}
	
	// Token should move to final node
	if token.Location != final {
		t.Error("expected token at final node")
	}
}

func TestActionExecutor_ForkNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	action1 := &ast.ActionExecutionNode{Name: "task1"}
	action2 := &ast.ActionExecutionNode{Name: "task2"}
	action3 := &ast.ActionExecutionNode{Name: "task3"}
	
	// Build action: initial → fork → [task1, task2, task3]
	actionSym := &symbols.Symbol{
		Name: "ForkAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "ForkAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, action3,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // start → fork
	
	// Step fork (1 token → 3 tokens)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step fork: %v", err)
	}
	
	if len(exec.tokens) != 3 {
		t.Fatalf("expected 3 tokens after fork, got %d", len(exec.tokens))
	}
	
	// Verify all tokens at correct locations
	locations := make(map[ast.Node]bool)
	for _, token := range exec.tokens {
		locations[token.Location] = true
	}
	
	if !locations[action1] || !locations[action2] || !locations[action3] {
		t.Error("expected tokens at task1, task2, task3")
	}
}

func TestActionExecutor_ForkNode_DataIsolation(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	action1 := &ast.ActionExecutionNode{Name: "task1"}
	action2 := &ast.ActionExecutionNode{Name: "task2"}
	action3 := &ast.ActionExecutionNode{Name: "task3"}
	
	actionSym := &symbols.Symbol{
		Name: "ForkAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "ForkAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, action3,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	
	// Set token data before fork
	exec.tokens[0].Data["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 100}}
	
	exec.stepToken(0) // start → fork
	
	// Step fork (1 token → 3 tokens)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step fork: %v", err)
	}
	
	// Verify all 3 forked tokens have the value
	for i, token := range exec.tokens {
		val, ok := token.Data["x"]
		if !ok {
			t.Errorf("token %d missing 'x' in data", i)
			continue
		}
		if val.Kind != ValConst || val.Const.Kind != semantics.ValInt || val.Const.Int != 100 {
			t.Errorf("token %d has wrong value: %v", i, val)
		}
	}
	
	// Modify one token's data
	exec.tokens[0].Data["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 999}}
	
	// Verify other tokens unaffected
	if exec.tokens[1].Data["x"].Const.Int != 100 {
		t.Error("token 1 data was affected by token 0 modification")
	}
	if exec.tokens[2].Data["x"].Const.Int != 100 {
		t.Error("token 2 data was affected by token 0 modification")
	}
}

func TestActionExecutor_ForkNode_NoSuccessors(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	
	// Fork node with NO outgoing edges
	actionSym := &symbols.Symbol{
		Name: "BrokenForkAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "BrokenForkAction"},
			Members: []ast.Node{
				initial, fork,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // start → fork
	
	// Step fork (should error: no successors)
	err = exec.stepToken(0)
	if err == nil {
		t.Fatal("expected error for fork with no successors")
	}
	
	expectedMsg := "fork node split has no successors"
	if err.Error() != expectedMsg {
		t.Errorf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestActionExecutor_JoinNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	task1 := &ast.ActionExecutionNode{
		Name:       "task1",
		Expression: &ast.LiteralInteger{Value: "1"},
	}
	task2 := &ast.ActionExecutionNode{
		Name:       "task2",
		Expression: &ast.LiteralInteger{Value: "2"},
	}
	task3 := &ast.ActionExecutionNode{
		Name:       "task3",
		Expression: &ast.LiteralInteger{Value: "3"},
	}
	join := &ast.JoinNode{Name: "merge"}
	final := &ast.FinalNode{Name: "end"}
	
	// Build graph: initial → fork → [task1, task2, task3] → join → final
	actionSym := &symbols.Symbol{
		Name: "JoinAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "JoinAction"},
			Members: []ast.Node{
				initial, fork, task1, task2, task3, join, final,
				// initial → fork
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				// fork → tasks
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
				// tasks → join
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				// join → final
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}}},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // start → fork
	exec.stepToken(0) // fork → [task1, task2, task3] (creates 3 tokens)
	
	// Should have 3 tokens at task nodes
	if len(exec.tokens) != 3 {
		t.Fatalf("expected 3 tokens after fork, got %d", len(exec.tokens))
	}
	
	// Step each task node (moves tokens to join)
	exec.stepToken(0) // task1 → join
	exec.stepToken(1) // task2 → join
	exec.stepToken(2) // task3 → join
	
	// All 3 tokens should be at join node
	if len(exec.tokens) != 3 {
		t.Fatalf("expected 3 tokens at join, got %d", len(exec.tokens))
	}
	
	for i, token := range exec.tokens {
		if token.Location != join {
			t.Errorf("token %d not at join node", i)
		}
	}
	
	// Step join (should synchronize: 3 tokens → 1 token)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step join: %v", err)
	}
	
	// Should have exactly 1 token
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token after join, got %d", len(exec.tokens))
	}
	
	// Token should be at final node
	if exec.tokens[0].Location != final {
		t.Error("expected token at final node")
	}
	
	// Token data should contain merged results from all tasks
	token := exec.tokens[0]
	result, ok := token.Data["result"]
	if !ok {
		t.Error("expected 'result' in merged token data")
	}
	
	// Last-write-wins: task3's result should be final (value 3)
	if result.Kind != ValConst || result.Const.Kind != semantics.ValInt || result.Const.Int != 3 {
		t.Errorf("expected result=3, got %v", result)
	}
}

func TestActionExecutor_JoinNode_PartialArrival(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	task1 := &ast.ActionExecutionNode{
		Name:       "task1",
		Expression: &ast.LiteralInteger{Value: "1"},
	}
	task2 := &ast.ActionExecutionNode{
		Name:       "task2",
		Expression: &ast.LiteralInteger{Value: "2"},
	}
	task3 := &ast.ActionExecutionNode{
		Name:       "task3",
		Expression: &ast.LiteralInteger{Value: "3"},
	}
	join := &ast.JoinNode{Name: "merge"}
	final := &ast.FinalNode{Name: "end"}
	
	// Build: fork → [task1, task2, task3] → join → final
	actionSym := &symbols.Symbol{
		Name: "JoinAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "JoinAction"},
			Members: []ast.Node{
				initial, fork, task1, task2, task3, join, final,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}}},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, actionSym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // start → fork
	exec.stepToken(0) // fork → [task1, task2, task3]
	
	// Step only 2 of 3 tokens to join
	exec.stepToken(0) // task1 → join
	exec.stepToken(1) // task2 → join
	
	// Now 2 tokens at join, 1 still at task3
	tokensAtJoin := 0
	for _, tok := range exec.tokens {
		if _, ok := tok.Location.(*ast.JoinNode); ok {
			tokensAtJoin++
		}
	}
	if tokensAtJoin != 2 {
		t.Fatalf("expected 2 tokens at join before last arrives, got %d", tokensAtJoin)
	}
	
	// Step join (should wait - return nil, no-op)
	err = exec.stepToken(0) // Steps first token at join
	if err != nil {
		t.Fatalf("join should wait silently, got error: %v", err)
	}
	
	// Verify join did NOT fire (still 3 tokens)
	if len(exec.tokens) != 3 {
		t.Errorf("expected 3 tokens (join waiting), got %d", len(exec.tokens))
	}
	
	// Step 3rd token to join
	exec.stepToken(2) // task3 → join
	
	// Now all 3 tokens at join
	tokensAtJoin = 0
	for _, tok := range exec.tokens {
		if _, ok := tok.Location.(*ast.JoinNode); ok {
			tokensAtJoin++
		}
	}
	if tokensAtJoin != 3 {
		t.Fatalf("expected 3 tokens at join after all arrive, got %d", tokensAtJoin)
	}
	
	// Step join again (should fire: 3 → 1)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("join should fire, got error: %v", err)
	}
	
	if len(exec.tokens) != 1 {
		t.Errorf("expected 1 token after all arrived, got %d", len(exec.tokens))
	}
	
	// Token should be at final node
	if exec.tokens[0].Location != final {
		t.Error("expected token at final node after join fires")
	}
}

func TestActionExecutor_MergeNode(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// Build: initial → fork → [merge, merge] → final
	// Fork with 2 edges both targeting merge
	// First token to reach merge wins, others discarded
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	merge := &ast.MergeNode{Name: "join"}
	final := &ast.FinalNode{Name: "end"}
	
	action := &symbols.Symbol{
		Name: "MergeAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "MergeAction"},
			Members: []ast.Node{
				initial, fork, merge, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
				},
				// Fork creates 2 tokens both going to merge
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()           // Token at initial
	exec.stepToken(0)            // initial → fork
	exec.stepToken(0)            // fork → [2 tokens at merge]
	
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens after fork, got %d", len(exec.tokens))
	}
	
	// Both tokens should be at merge
	for i, tok := range exec.tokens {
		if tok.Location != merge {
			t.Errorf("token %d not at merge node", i)
		}
	}
	
	// First token reaches merge (should pass through)
	exec.stepToken(0)
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens (1 advanced, 1 waiting), got %d", len(exec.tokens))
	}
	
	// Verify first token advanced to final
	tokenAtFinal := false
	tokenAtMerge := false
	mergeIdx := -1
	for i, tok := range exec.tokens {
		if tok.Location == final {
			tokenAtFinal = true
		}
		if tok.Location == merge {
			tokenAtMerge = true
			mergeIdx = i
		}
	}
	if !tokenAtFinal || !tokenAtMerge {
		t.Error("expected one token at final, one at merge")
	}
	
	// Second token reaches merge (should be discarded)
	err = exec.stepToken(mergeIdx)
	if err != nil {
		t.Fatalf("step merge: %v", err)
	}
	
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token after merge discard, got %d", len(exec.tokens))
	}
	
	// Verify final token at expected location
	if exec.tokens[0].Location != final {
		t.Error("expected token at final node")
	}
}

func TestActionExecutor_MergeNode_DataDiscard(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// fork → [path1 sets x=1, path2 sets x=2] → merge → final
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	action1 := &ast.ActionExecutionNode{
		Name:       "task1",
		Expression: &ast.LiteralInteger{Value: "1"},
	}
	action2 := &ast.ActionExecutionNode{
		Name:       "task2",
		Expression: &ast.LiteralInteger{Value: "2"},
	}
	merge := &ast.MergeNode{Name: "join"}
	final := &ast.FinalNode{Name: "end"}
	
	action := &symbols.Symbol{
		Name: "DataDiscardAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "DataDiscardAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, merge, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // initial → fork
	exec.stepToken(0) // fork → [task1, task2]
	
	// After fork, 2 tokens at action nodes
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens after fork, got %d", len(exec.tokens))
	}
	
	// Step both action nodes (by index, being careful about ordering)
	// After fork, tokens are at [task_a, task_b] in some order
	exec.stepToken(0) // step first task → moves to merge
	exec.stepToken(1) // step second task → moves to merge
	
	// Now 2 tokens at merge with different data
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens at merge, got %d", len(exec.tokens))
	}
	
	// Verify both at merge
	for i, tok := range exec.tokens {
		if _, ok := tok.Location.(*ast.MergeNode); !ok {
			t.Fatalf("token %d not at merge: %T", i, tok.Location)
		}
	}
	
	// Record both tokens' data
	firstData := exec.tokens[0].Data["result"].Const.Int
	secondData := exec.tokens[1].Data["result"].Const.Int
	
	// Verify they're different
	if firstData == secondData {
		t.Fatal("expected tokens to have different data")
	}
	
	// Step first token through merge (wins, moves to final)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step first merge token: %v", err)
	}
	
	// Should have 2 tokens: 1 at final (winner), 1 at merge (loser)
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens (1 at final, 1 at merge), got %d", len(exec.tokens))
	}
	
	// Find which token is at final (winner)
	var winnerIdx int
	var winnerData int64
	for i, tok := range exec.tokens {
		if _, ok := tok.Location.(*ast.FinalNode); ok {
			winnerIdx = i
			winnerData = tok.Data["result"].Const.Int
			break
		}
	}
	
	// Winner data should match first token's data
	if winnerData != firstData {
		t.Errorf("expected winner to have first token's data (%d), got %d", firstData, winnerData)
	}
	
	// Step second token through merge (discarded)
	loserIdx := 1 - winnerIdx
	exec.stepToken(loserIdx)
	
	// Should have 1 token at final with winner's data
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token after discard, got %d", len(exec.tokens))
	}
	
	finalData := exec.tokens[0].Data["result"].Const.Int
	if finalData != winnerData {
		t.Errorf("expected final token to have winner's data (%d), got %d", winnerData, finalData)
	}
}

func TestActionExecutor_MergeNode_SingleParent(t *testing.T) {
	ctx := NewContext(semantics.NewModel(nil), nil, 1000)
	
	// initial → merge → final (degenerate case, merge pass-through)
	initial := &ast.InitialNode{Name: "start"}
	merge := &ast.MergeNode{Name: "join"}
	final := &ast.FinalNode{Name: "end"}
	
	action := &symbols.Symbol{
		Name: "SingleParentMerge",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:    ast.UsageAction,
			Ident:   ast.Identification{Name: "SingleParentMerge"},
			Members: []ast.Node{
				initial, merge, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "end"}}},
				},
			},
		},
	}
	
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	
	exec.initialize()
	exec.stepToken(0) // initial → merge
	exec.stepToken(0) // merge → final
	
	if len(exec.tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(exec.tokens))
	}
	
	if exec.tokens[0].Location != final {
		t.Error("token should pass through merge to final")
	}
}
