package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestActionExecutor_Creation(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

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

	exec, err := newActionExecutor(ctx, action, nil)
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
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{}
	edge := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
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

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	if len(exec.graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(exec.graph.Nodes))
	}

	successors, ok := exec.graph.Edges[initial]
	if !ok || len(successors) != 1 || successors[0].Target != final {
		t.Error("expected edge from initial to final")
	}
}

func TestActionExecutor_InitialNode(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Create action: initial → final
	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "SimpleAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "SimpleAction"},
			Members: []ast.Node{
				initial,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
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
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	initial := &ast.InitialNode{Name: "start"}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "SimpleAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "SimpleAction"},
			Members: []ast.Node{
				initial,
				final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
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
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	initial := &ast.InitialNode{Name: "start"}
	action := &ast.ActionExecutionNode{
		Name:       "compute",
		Expression: &ast.LiteralInteger{Value: "42"},
	}
	final := &ast.FinalNode{}

	// Build action: initial → compute → final
	actionSym := &symbols.Symbol{
		Name: "ComputeAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ComputeAction"},
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
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
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

	// The action's features should hold the result
	result, ok := exec.Data()["result"]
	if !ok {
		t.Error("expected result in action data")
	}

	// Check result value
	if result.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", result.Kind)
	}

	if result.Const.Kind != semantics.ValInt || result.Const.Int != 42 {
		t.Errorf("expected integer 42, got %v", result.Const)
	}

	// Token should move to final node
	if exec.tokens[0].Location != final {
		t.Error("expected token at final node")
	}
}

func TestActionExecutor_ForkNode(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

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
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ForkAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, action3,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
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

// TestActionExecutor_ForkNode_SharedFeatureSpace locks the value space of a
// fork's branches: they read and write the one feature space of the action they
// belong to, so a write in one branch is visible in the others.
func TestActionExecutor_ForkNode_SharedFeatureSpace(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	action1 := &ast.ActionExecutionNode{Name: "task1"}
	action2 := &ast.ActionExecutionNode{Name: "task2"}
	action3 := &ast.ActionExecutionNode{Name: "task3"}

	actionSym := &symbols.Symbol{
		Name: "ForkAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ForkAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, action3,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()

	// Set a feature value before the fork
	exec.root.data["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 100}}

	exec.stepToken(0) // start → fork

	// Step fork (1 token → 3 tokens)
	err = exec.stepToken(0)
	if err != nil {
		t.Fatalf("step fork: %v", err)
	}

	if len(exec.tokens) != 3 {
		t.Fatalf("expected 3 tokens after fork, got %d", len(exec.tokens))
	}

	// The fork copies control, not values: the feature keeps the value it held.
	if got := exec.root.data["x"]; got.Kind != ValConst || got.Const.Int != 100 {
		t.Errorf("expected x = 100 after fork, got %v", got)
	}

	// A write in one branch is a write to the action's feature, so every branch
	// and the action's results see it.
	exec.root.data["x"] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 999}}
	if got := exec.Results()["x"]; got.Const.Int != 999 {
		t.Errorf("expected results to report x = 999, got %v", got)
	}
}

func TestActionExecutor_ForkNode_NoSuccessors(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}

	// Fork node with NO outgoing edges
	actionSym := &symbols.Symbol{
		Name: "BrokenForkAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "BrokenForkAction"},
			Members: []ast.Node{
				initial, fork,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
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

	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Errorf("expected ErrInvalidActionFlow, got %v", err)
	}
}

// A node with no succession out of it ends its flow: each forked branch retires
// there, and the action completes once the last token has.
func TestActionExecutor_NodeWithoutSuccessorsRetiresItsToken(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

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

	// initial → fork → [task1, task2], neither task having a successor.
	actionSym := &symbols.Symbol{
		Name: "EndsWithoutAFinalNode",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "EndsWithoutAFinalNode"},
			Members: []ast.Node{
				initial, fork, task1, task2,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	if exec.State() != StateCompleted {
		t.Errorf("expected StateCompleted, got %s", exec.State())
	}
	if len(exec.tokens) != 0 {
		t.Errorf("expected every token retired, %d left", len(exec.tokens))
	}
	result, ok := exec.Results()["result"]
	if !ok {
		t.Fatal("expected the retired tokens to leave their data as results")
	}
	if result.Kind != ValConst || result.Const.Kind != semantics.ValInt {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestActionExecutor_JoinNode(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

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
	final := &ast.FinalNode{}

	// Build graph: initial → fork → [task1, task2, task3] → join → final
	actionSym := &symbols.Symbol{
		Name: "JoinAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "JoinAction"},
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
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}}},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
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

	// The branches wrote the same feature, so it holds the last write
	result, ok := exec.root.data["result"]
	if !ok {
		t.Error("expected 'result' in the action's data")
	}

	// Last-write-wins: task3's result should be final (value 3)
	if result.Kind != ValConst || result.Const.Kind != semantics.ValInt || result.Const.Int != 3 {
		t.Errorf("expected result=3, got %v", result)
	}
}

func TestActionExecutor_JoinNode_PartialArrival(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

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
	final := &ast.FinalNode{}

	// Build: fork → [task1, task2, task3] → join → final
	actionSym := &symbols.Symbol{
		Name: "JoinAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "JoinAction"},
			Members: []ast.Node{
				initial, fork, task1, task2, task3, join, final,
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task1"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task2"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "task3"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}},
				&ast.SuccessionEdge{Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "merge"}}}, Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}}},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
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

// TestActionExecutor_MergeNode: a merge passes every arriving token, so two fork
// branches reaching it are two traversals, each going on to the final node.
func TestActionExecutor_MergeNode(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → fork → [merge, merge] → final
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	merge := &ast.MergeNode{Name: "join"}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "MergeAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "MergeAction"},
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
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize() // Token at initial
	exec.stepToken(0) // initial → fork
	exec.stepToken(0) // fork → [2 tokens at merge]

	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens after fork, got %d", len(exec.tokens))
	}

	// Both tokens should be at merge
	for i, tok := range exec.tokens {
		if tok.Location != merge {
			t.Errorf("token %d not at merge node", i)
		}
	}

	// The first arrival traverses to final; the second still stands at the merge.
	if err := exec.stepToken(0); err != nil {
		t.Fatalf("step first merge token: %v", err)
	}
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens (1 advanced, 1 at the merge), got %d", len(exec.tokens))
	}
	mergeIdx := -1
	tokenAtFinal := false
	for i, tok := range exec.tokens {
		if tok.Location == final {
			tokenAtFinal = true
		}
		if tok.Location == merge {
			mergeIdx = i
		}
	}
	if !tokenAtFinal || mergeIdx < 0 {
		t.Fatal("expected one token at final, one at merge")
	}

	// The second arrival traverses too: the merge passes it, it does not discard it.
	if err := exec.stepToken(mergeIdx); err != nil {
		t.Fatalf("step second merge token: %v", err)
	}
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens after both traversed the merge, got %d", len(exec.tokens))
	}
	for i, tok := range exec.tokens {
		if tok.Location != final {
			t.Errorf("token %d at %T, want the final node", i, tok.Location)
		}
	}
}

// TestActionExecutor_MergeNode_ControlOnly: a merge carries control only — both
// branches' tokens pass it and the value their writes left is untouched by the traversals.
func TestActionExecutor_MergeNode_ControlOnly(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

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
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "DataDiscardAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "DataDiscardAction"},
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
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
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

	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens at merge, got %d", len(exec.tokens))
	}

	// Verify both at merge
	for i, tok := range exec.tokens {
		if _, ok := tok.Location.(*ast.MergeNode); !ok {
			t.Fatalf("token %d not at merge: %T", i, tok.Location)
		}
	}

	// Both branches wrote the same feature, so it holds the later write.
	beforeMerge := exec.root.data["result"].Const.Int
	if beforeMerge != 1 && beforeMerge != 2 {
		t.Fatalf("expected result written by one of the branches, got %d", beforeMerge)
	}

	// Each token traverses the merge to final in turn.
	if err := exec.stepToken(0); err != nil {
		t.Fatalf("step first merge token: %v", err)
	}
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens (1 at final, 1 at merge), got %d", len(exec.tokens))
	}
	mergeIdx := -1
	for i, tok := range exec.tokens {
		if _, ok := tok.Location.(*ast.MergeNode); ok {
			mergeIdx = i
		}
	}
	if mergeIdx < 0 {
		t.Fatal("expected the second token still at the merge")
	}
	if err := exec.stepToken(mergeIdx); err != nil {
		t.Fatalf("step second merge token: %v", err)
	}
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens at final after both traversed, got %d", len(exec.tokens))
	}
	for i, tok := range exec.tokens {
		if _, ok := tok.Location.(*ast.FinalNode); !ok {
			t.Errorf("token %d at %T, want the final node", i, tok.Location)
		}
	}

	// Traversing a merge touches no value.
	if got := exec.root.data["result"].Const.Int; got != beforeMerge {
		t.Errorf("expected result to stay %d after the merge, got %d", beforeMerge, got)
	}
}

func TestActionExecutor_MergeNode_SingleParent(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// initial → merge → final (degenerate case, merge pass-through)
	initial := &ast.InitialNode{Name: "start"}
	merge := &ast.MergeNode{Name: "join"}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "SingleParentMerge",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "SingleParentMerge"},
			Members: []ast.Node{
				initial, merge, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "join"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
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

func TestActionExecutor_DecisionNode(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → decision → [pathA (if x>10), pathB (if x<=10)] → final
	initial := &ast.InitialNode{Name: "start"}
	decision := &ast.DecisionNode{Name: "check"}
	pathA := &ast.ActionExecutionNode{
		Name:       "pathA",
		Expression: &ast.LiteralString{Value: "A"},
	}
	pathB := &ast.ActionExecutionNode{
		Name:       "pathB",
		Expression: &ast.LiteralString{Value: "B"},
	}
	final := &ast.FinalNode{}

	// Guards: x > 10 and x <= 10
	guardA := &ast.OperatorExpr{
		Operator: ast.OpGt,
		Operands: []ast.Node{
			&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
			&ast.LiteralInteger{Value: "10"},
		},
	}
	guardB := &ast.OperatorExpr{
		Operator: ast.OpLe,
		Operands: []ast.Node{
			&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
			&ast.LiteralInteger{Value: "10"},
		},
	}

	action := &symbols.Symbol{
		Name: "DecisionAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "DecisionAction"},
			Members: []ast.Node{
				initial, decision, pathA, pathB, final,
				// initial → decision
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
				},
				// decision → pathA (guarded)
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathA"}}},
					Guard:  guardA,
				},
				// decision → pathB (guarded)
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathB"}}},
					Guard:  guardB,
				},
				// paths → final
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathA"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathB"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	// Test 1: x=15 → should take pathA
	t.Run("x=15_takes_pathA", func(t *testing.T) {
		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()

		// Set x=15 in the action's data
		exec.root.data["x"] = Value{
			Kind:  ValConst,
			Const: semantics.Value{Kind: semantics.ValInt, Int: 15},
		}

		// Step: initial → decision
		exec.stepToken(0)
		if exec.tokens[0].Location != decision {
			t.Fatal("token should be at decision node")
		}

		// Step: decision → pathA (x>10 is true)
		err = exec.stepToken(0)
		if err != nil {
			t.Fatalf("step decision: %v", err)
		}

		if exec.tokens[0].Location != pathA {
			t.Errorf("expected token at pathA, got %T", exec.tokens[0].Location)
		}
	})

	// Test 2: x=5 → should take pathB
	t.Run("x=5_takes_pathB", func(t *testing.T) {
		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()

		// Set x=5 in the action's data
		exec.root.data["x"] = Value{
			Kind:  ValConst,
			Const: semantics.Value{Kind: semantics.ValInt, Int: 5},
		}

		// Step: initial → decision
		exec.stepToken(0)
		if exec.tokens[0].Location != decision {
			t.Fatal("token should be at decision node")
		}

		// Step: decision → pathB (x<=10 is true)
		err = exec.stepToken(0)
		if err != nil {
			t.Fatalf("step decision: %v", err)
		}

		if exec.tokens[0].Location != pathB {
			t.Errorf("expected token at pathB, got %T", exec.tokens[0].Location)
		}
	})

	// Test 3: No guard matches → error
	t.Run("no_guard_matches", func(t *testing.T) {
		// Build decision with x>100 and x<0 guards (impossible for x=50)
		guardHigh := &ast.OperatorExpr{
			Operator: ast.OpGt,
			Operands: []ast.Node{
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
				&ast.LiteralInteger{Value: "100"},
			},
		}
		guardLow := &ast.OperatorExpr{
			Operator: ast.OpLt,
			Operands: []ast.Node{
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
				&ast.LiteralInteger{Value: "0"},
			},
		}

		strictAction := &symbols.Symbol{
			Name: "StrictDecision",
			Kind: symbols.SymbolActionUsage,
			Decl: &ast.Usage{
				Kind:  ast.UsageAction,
				Ident: ast.Identification{Name: "StrictDecision"},
				Members: []ast.Node{
					initial, decision, pathA, pathB,
					&ast.SuccessionEdge{
						Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
						Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
					},
					&ast.ControlFlowEdge{
						Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
						Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathA"}}},
						Guard:  guardHigh,
					},
					&ast.ControlFlowEdge{
						Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
						Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathB"}}},
						Guard:  guardLow,
					},
				},
			},
		}

		exec, err := newActionExecutor(ctx, strictAction, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()
		exec.root.data["x"] = Value{
			Kind:  ValConst,
			Const: semantics.Value{Kind: semantics.ValInt, Int: 50},
		}

		exec.stepToken(0) // initial → decision

		// Step decision (no guard matches)
		err = exec.stepToken(0)
		if err == nil {
			t.Fatal("expected error when no guard matches")
		}

		if !errors.Is(err, ErrNoEnabledSuccession) {
			t.Errorf("expected ErrNoEnabledSuccession, got %v", err)
		}
	})
}

func TestActionExecutor_DecisionNode_ElseBranch(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → decision → [pathA (if x>10), pathElse (no guard)] → final
	// Test that unguarded edge works as fallback after guarded edges
	initial := &ast.InitialNode{Name: "start"}
	decision := &ast.DecisionNode{Name: "check"}
	pathA := &ast.ActionExecutionNode{
		Name:       "pathA",
		Expression: &ast.LiteralString{Value: "A"},
	}
	pathElse := &ast.ActionExecutionNode{
		Name:       "pathElse",
		Expression: &ast.LiteralString{Value: "Else"},
	}
	final := &ast.FinalNode{}

	// Guard: x > 10
	guardA := &ast.OperatorExpr{
		Operator: ast.OpGt,
		Operands: []ast.Node{
			&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
			&ast.LiteralInteger{Value: "10"},
		},
	}

	action := &symbols.Symbol{
		Name: "ElseBranchAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ElseBranchAction"},
			Members: []ast.Node{
				initial, decision, pathA, pathElse, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
				},
				// decision → pathA (guarded)
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathA"}}},
					Guard:  guardA,
				},
				// decision → pathElse (unguarded - else branch)
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathElse"}}},
					Guard:  nil,
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathA"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathElse"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	// Test 1: x=15 → should take pathA (guard matches)
	t.Run("guard_matches_takes_pathA", func(t *testing.T) {
		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()
		exec.root.data["x"] = Value{
			Kind:  ValConst,
			Const: semantics.Value{Kind: semantics.ValInt, Int: 15},
		}

		exec.stepToken(0) // initial → decision
		if exec.tokens[0].Location != decision {
			t.Fatal("token should be at decision node")
		}

		err = exec.stepToken(0) // decision → pathA or pathElse
		if err != nil {
			t.Fatalf("step decision: %v", err)
		}

		if exec.tokens[0].Location != pathA {
			t.Errorf("expected token at pathA, got %T", exec.tokens[0].Location)
		}
	})

	// Test 2: x=5 → should take pathElse (guard doesn't match, fallback to unguarded)
	t.Run("guard_doesnt_match_takes_else", func(t *testing.T) {
		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()
		exec.root.data["x"] = Value{
			Kind:  ValConst,
			Const: semantics.Value{Kind: semantics.ValInt, Int: 5},
		}

		exec.stepToken(0) // initial → decision
		if exec.tokens[0].Location != decision {
			t.Fatal("token should be at decision node")
		}

		err = exec.stepToken(0) // decision → pathElse (fallback)
		if err != nil {
			t.Fatalf("step decision: %v", err)
		}

		if exec.tokens[0].Location != pathElse {
			t.Errorf("expected token at pathElse, got %T", exec.tokens[0].Location)
		}
	})
}

func TestActionExecutor_DecisionNode_NonBooleanGuard(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → decision → pathA (with integer guard - invalid)
	initial := &ast.InitialNode{Name: "start"}
	decision := &ast.DecisionNode{Name: "check"}
	pathA := &ast.ActionExecutionNode{
		Name:       "pathA",
		Expression: &ast.LiteralString{Value: "A"},
	}

	// Guard that returns integer instead of boolean
	guardInt := &ast.LiteralInteger{Value: "42"}

	action := &symbols.Symbol{
		Name: "NonBoolGuardAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "NonBoolGuardAction"},
			Members: []ast.Node{
				initial, decision, pathA,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
				},
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "check"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "pathA"}}},
					Guard:  guardInt,
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()
	exec.stepToken(0) // initial → decision

	// Step decision - should error on non-boolean guard
	err = exec.stepToken(0)
	if err == nil {
		t.Fatal("expected error for non-boolean guard")
	}

	// Check error message contains expected text
	if !containsText(err.Error(), "guard must evaluate to boolean") {
		t.Errorf("unexpected error: %v", err)
	}
}

func containsText(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestActionExecutor_ObjectFlow(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → action1 → action2 → final
	// With ObjectFlowEdge: action1.output → action2.input
	initial := &ast.InitialNode{Name: "start"}
	action1 := &ast.ActionExecutionNode{
		Name:       "action1",
		Expression: &ast.LiteralInteger{Value: "42"}, // Produces value 42
	}
	action2 := &ast.ActionExecutionNode{
		Name:       "action2",
		Expression: &ast.LiteralString{Value: "received"}, // Placeholder expression
	}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "ObjectFlowAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ObjectFlowAction"},
			Members: []ast.Node{
				initial, action1, action2, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "action1"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "action1"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "action2"}}},
				},
				&ast.ObjectFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "action1"}, {Text: "output"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "action2"}, {Text: "input"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "action2"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	// Verify ObjectFlowEdge extracted
	if len(exec.graph.DataFlows) == 0 {
		t.Fatal("expected dataFlows map to be populated")
	}

	flows := exec.graph.DataFlows[action1]
	if len(flows) != 1 {
		t.Fatalf("expected 1 object flow from action1, got %d", len(flows))
	}

	if flows[0].SourcePin != "output" {
		t.Errorf("expected source pin 'output', got %q", flows[0].SourcePin)
	}
	if flows[0].TargetPin != "input" {
		t.Errorf("expected target pin 'input', got %q", flows[0].TargetPin)
	}
	if flows[0].Target != action2 {
		t.Error("expected target to be action2 node")
	}

	// Execute and verify data transfer
	exec.initialize()
	exec.stepToken(0) // initial → action1
	exec.stepToken(0) // action1: execute, store result in "output" pin

	// Verify action1 stored result in "output" pin
	if _, ok := exec.root.data["output"]; !ok {
		t.Fatal("expected action1 to store result in 'output' pin")
	}

	outputVal := exec.root.data["output"]
	if outputVal.Kind != ValConst || outputVal.Const.Kind != semantics.ValInt || outputVal.Const.Int != 42 {
		t.Errorf("expected output=42, got %v", outputVal)
	}

	exec.stepToken(0) // action1 → action2 (control flow + data flow)

	// Verify action2 received data in "input" pin
	if _, ok := exec.root.data["input"]; !ok {
		t.Fatal("expected action2 to receive data in 'input' pin")
	}

	inputVal := exec.root.data["input"]
	if inputVal.Kind != ValConst || inputVal.Const.Kind != semantics.ValInt || inputVal.Const.Int != 42 {
		t.Errorf("expected input=42, got %v", inputVal)
	}
}

func TestActionExecutor_Step(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → action → final
	initial := &ast.InitialNode{Name: "start"}
	action := &ast.ActionExecutionNode{
		Name:       "compute",
		Expression: &ast.LiteralInteger{Value: "100"},
	}
	final := &ast.FinalNode{}

	actionSym := &symbols.Symbol{
		Name: "StepTestAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "StepTestAction"},
			Members: []ast.Node{
				initial, action, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()

	// Step 1: initial → action
	err = exec.Step()
	if err != nil {
		t.Fatalf("step 1: %v", err)
	}
	if exec.tokens[0].Location != action {
		t.Error("expected token at action node after step 1")
	}

	// Step 2: action executes, moves to final
	err = exec.Step()
	if err != nil {
		t.Fatalf("step 2: %v", err)
	}
	if exec.tokens[0].Location != final {
		t.Error("expected token at final node after step 2")
	}

	// Step 3: final consumes token, marks completion
	err = exec.Step()
	if err != nil {
		t.Fatalf("step 3: %v", err)
	}
	if len(exec.tokens) != 0 {
		t.Errorf("expected no tokens after final, got %d", len(exec.tokens))
	}
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
}

func TestActionExecutor_RunToCompletion(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → fork → [action1, action2] → join → final
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	action1 := &ast.ActionExecutionNode{
		Name:       "compute1",
		Expression: &ast.LiteralInteger{Value: "10"},
	}
	action2 := &ast.ActionExecutionNode{
		Name:       "compute2",
		Expression: &ast.LiteralInteger{Value: "20"},
	}
	join := &ast.JoinNode{Name: "sync"}
	final := &ast.FinalNode{}

	actionSym := &symbols.Symbol{
		Name: "RunToCompletionAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "RunToCompletionAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, join, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute1"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute2"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute1"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "compute2"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()

	// Run to completion
	err = exec.RunToCompletion()
	if err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	// Verify completed state
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}

	if len(exec.tokens) != 0 {
		t.Errorf("expected no tokens after completion, got %d", len(exec.tokens))
	}
}

func TestActionExecutor_Deadlock_JoinStarvation(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build deadlock scenario:
	// initial → fork → [path1 → join, path2 → action2]
	// Join expects 2 tokens (has 2 incoming edges), but path2 never reaches join
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	action1 := &ast.ActionExecutionNode{
		Name:       "path1",
		Expression: &ast.LiteralInteger{Value: "1"},
	}
	action2 := &ast.ActionExecutionNode{
		Name:       "path2",
		Expression: &ast.LiteralInteger{Value: "2"},
	}
	join := &ast.JoinNode{Name: "sync"}
	final := &ast.FinalNode{}

	actionSym := &symbols.Symbol{
		Name: "DeadlockAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "DeadlockAction"},
			Members: []ast.Node{
				initial, fork, action1, action2, join, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
				},
				// Fork creates 2 tokens
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "path1"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "path2"}}},
				},
				// Path1 reaches join
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "path1"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
				},
				// Path2 reaches join (creates expectation of 2 incoming tokens)
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "path2"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()

	// Manually step to create deadlock scenario:
	// Remove one token after fork to simulate path failure
	exec.stepToken(0) // initial → fork
	exec.stepToken(0) // fork → [path1, path2]

	// Should now have 2 tokens at path1 and path2
	if len(exec.tokens) != 2 {
		t.Fatalf("expected 2 tokens after fork, got %d", len(exec.tokens))
	}

	// Remove path2 token to simulate failure (creates join starvation)
	// Find path2 token
	path2Idx := -1
	for i, tok := range exec.tokens {
		if n, ok := tok.Location.(*ast.ActionExecutionNode); ok && n.Name == "path2" {
			path2Idx = i
			break
		}
	}
	if path2Idx == -1 {
		t.Fatal("couldn't find path2 token")
	}
	exec.tokens = append(exec.tokens[:path2Idx], exec.tokens[path2Idx+1:]...)

	// Now run to completion - should detect deadlock
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected deadlock error, got nil")
	}

	// Check error message
	if !containsText(err.Error(), "deadlock") {
		t.Errorf("expected deadlock error, got: %v", err)
	}
}

// Integration Tests (Tasks 12-16)

func TestActionExecutor_Integration_Sequential(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → compute1 → compute2 → compute3 → final
	// Sequential execution with data flow
	initial := &ast.InitialNode{Name: "start"}
	compute1 := &ast.ActionExecutionNode{
		Name:       "step1",
		Expression: &ast.LiteralInteger{Value: "10"},
	}
	compute2 := &ast.ActionExecutionNode{
		Name:       "step2",
		Expression: &ast.LiteralInteger{Value: "20"},
	}
	compute3 := &ast.ActionExecutionNode{
		Name:       "step3",
		Expression: &ast.LiteralInteger{Value: "30"},
	}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "SequentialAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "SequentialAction"},
			Members: []ast.Node{
				initial, compute1, compute2, compute3, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "step1"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "step1"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "step2"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "step2"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "step3"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "step3"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()

	// Run to completion
	err = exec.RunToCompletion()
	if err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	// Verify completed state
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}

	if len(exec.tokens) != 0 {
		t.Errorf("expected no tokens, got %d", len(exec.tokens))
	}
}

func TestActionExecutor_Integration_ForkJoin(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → fork → [path1, path2, path3] → join → final
	// Parallel execution with synchronization
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	path1 := &ast.ActionExecutionNode{
		Name:       "parallel1",
		Expression: &ast.LiteralInteger{Value: "1"},
	}
	path2 := &ast.ActionExecutionNode{
		Name:       "parallel2",
		Expression: &ast.LiteralInteger{Value: "2"},
	}
	path3 := &ast.ActionExecutionNode{
		Name:       "parallel3",
		Expression: &ast.LiteralInteger{Value: "3"},
	}
	join := &ast.JoinNode{Name: "sync"}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "ForkJoinAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ForkJoinAction"},
			Members: []ast.Node{
				initial, fork, path1, path2, path3, join, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel1"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel2"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "split"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel3"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel1"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel2"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel3"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()

	// Run to completion
	err = exec.RunToCompletion()
	if err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	// Verify completed
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
}

func TestActionExecutor_Integration_DecisionMerge(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build: initial → decision → [pathTrue, pathFalse] → merge → final
	// Conditional branching with merge
	initial := &ast.InitialNode{Name: "start"}
	decision := &ast.DecisionNode{Name: "branch"}
	pathTrue := &ast.ActionExecutionNode{
		Name:       "whenTrue",
		Expression: &ast.LiteralString{Value: "true_path"},
	}
	pathFalse := &ast.ActionExecutionNode{
		Name:       "whenFalse",
		Expression: &ast.LiteralString{Value: "false_path"},
	}
	merge := &ast.MergeNode{Name: "converge"}
	final := &ast.FinalNode{}

	// Guard: x > 5
	guardExpr := &ast.OperatorExpr{
		Operator: ast.OpGt,
		Operands: []ast.Node{
			&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "x"}}}},
			&ast.LiteralInteger{Value: "5"},
		},
	}

	action := &symbols.Symbol{
		Name: "DecisionMergeAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "DecisionMergeAction"},
			Members: []ast.Node{
				initial, decision, pathTrue, pathFalse, merge, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "branch"}}},
				},
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "branch"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "whenTrue"}}},
					Guard:  guardExpr,
				},
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "branch"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "whenFalse"}}},
					Guard:  nil, // Else branch
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "whenTrue"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "converge"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "whenFalse"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "converge"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "converge"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	// Test 1: x=10 (takes true path)
	t.Run("true_path", func(t *testing.T) {
		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()
		exec.root.data["x"] = Value{
			Kind:  ValConst,
			Const: semantics.Value{Kind: semantics.ValInt, Int: 10},
		}

		err = exec.RunToCompletion()
		if err != nil {
			t.Fatalf("run to completion: %v", err)
		}

		if exec.state != StateCompleted {
			t.Errorf("expected StateCompleted, got %v", exec.state)
		}
	})

	// Test 2: x=3 (takes false path)
	t.Run("false_path", func(t *testing.T) {
		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()
		exec.root.data["x"] = Value{
			Kind:  ValConst,
			Const: semantics.Value{Kind: semantics.ValInt, Int: 3},
		}

		err = exec.RunToCompletion()
		if err != nil {
			t.Fatalf("run to completion: %v", err)
		}

		if exec.state != StateCompleted {
			t.Errorf("expected StateCompleted, got %v", exec.state)
		}
	})
}

func TestActionExecutor_Integration_ObjectFlow(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Build pipeline with data flow: initial → producer → consumer → final
	// ObjectFlow: producer.result → consumer.input
	initial := &ast.InitialNode{Name: "start"}
	producer := &ast.ActionExecutionNode{
		Name:       "producer",
		Expression: &ast.LiteralInteger{Value: "999"},
	}
	consumer := &ast.ActionExecutionNode{
		Name:       "consumer",
		Expression: &ast.LiteralString{Value: "processed"}, // Could read input in real scenario
	}
	final := &ast.FinalNode{}

	action := &symbols.Symbol{
		Name: "ObjectFlowAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ObjectFlowAction"},
			Members: []ast.Node{
				initial, producer, consumer, final,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "producer"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "producer"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "consumer"}}},
				},
				&ast.ObjectFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "producer"}, {Text: "result"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "consumer"}, {Text: "input"}}},
				},
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "consumer"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
				},
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	exec.initialize()
	err = exec.RunToCompletion()
	if err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}
}

func TestActionExecutor_Integration_ErrorCases(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Test 1: Initial node missing
	t.Run("missing_initial_node", func(t *testing.T) {
		action := &symbols.Symbol{
			Name: "NoInitialAction",
			Kind: symbols.SymbolActionUsage,
			Decl: &ast.Usage{
				Kind:  ast.UsageAction,
				Ident: ast.Identification{Name: "NoInitialAction"},
				Members: []ast.Node{
					&ast.FinalNode{},
				},
			},
		}

		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		err = exec.initialize()
		if err == nil {
			t.Fatal("expected error for missing initial node")
		}

		if !containsText(err.Error(), "no initial node") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Test 2: Undefined edge reference
	t.Run("undefined_edge_reference", func(t *testing.T) {
		initial := &ast.InitialNode{Name: "start"}

		action := &symbols.Symbol{
			Name: "BadEdgeAction",
			Kind: symbols.SymbolActionUsage,
			Decl: &ast.Usage{
				Kind:  ast.UsageAction,
				Ident: ast.Identification{Name: "BadEdgeAction"},
				Members: []ast.Node{
					initial,
					&ast.SuccessionEdge{
						Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
						Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "nonexistent"}}},
					},
				},
			},
		}

		_, err := newActionExecutor(ctx, action, nil)
		if err == nil {
			t.Fatal("expected error for undefined edge reference")
		}

		if !containsText(err.Error(), "undefined") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Test 3: Initial node with no successors
	t.Run("initial_no_successors", func(t *testing.T) {
		initial := &ast.InitialNode{Name: "start"}

		action := &symbols.Symbol{
			Name: "DeadEndAction",
			Kind: symbols.SymbolActionUsage,
			Decl: &ast.Usage{
				Kind:    ast.UsageAction,
				Ident:   ast.Identification{Name: "DeadEndAction"},
				Members: []ast.Node{initial},
			},
		}

		exec, err := newActionExecutor(ctx, action, nil)
		if err != nil {
			t.Fatalf("create executor: %v", err)
		}

		exec.initialize()
		err = exec.stepToken(0)
		if err == nil {
			t.Fatal("expected error for initial node with no successors")
		}

		if !containsText(err.Error(), "no successors") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// Task 48: Parallel processing workflow - fork/join + data merge
func TestActionExecutor_Integration_ParallelProcessing(t *testing.T) {
	ctx := NewContext(NewModel(semantics.NewModel(nil), nil), 1000)

	// Workflow: initial → fork → [processA, processB, processC] → join → aggregate → final
	// Each processor adds to input value, join merges all data, aggregate sums

	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "parallel"}

	// Each processor outputs to unique pin
	processA := &ast.ActionExecutionNode{
		Name: "processA",
		Expression: &ast.OperatorExpr{
			Operator: ast.OpAdd,
			Operands: []ast.Node{
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "input"}}}},
				&ast.LiteralInteger{Value: "10"},
			},
		},
	}
	processB := &ast.ActionExecutionNode{
		Name: "processB",
		Expression: &ast.OperatorExpr{
			Operator: ast.OpAdd,
			Operands: []ast.Node{
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "input"}}}},
				&ast.LiteralInteger{Value: "20"},
			},
		},
	}
	processC := &ast.ActionExecutionNode{
		Name: "processC",
		Expression: &ast.OperatorExpr{
			Operator: ast.OpAdd,
			Operands: []ast.Node{
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "input"}}}},
				&ast.LiteralInteger{Value: "30"},
			},
		},
	}

	join := &ast.JoinNode{Name: "sync"}

	// Aggregate sums all processor results
	aggregate := &ast.ActionExecutionNode{
		Name: "aggregate",
		Expression: &ast.OperatorExpr{
			Operator: ast.OpAdd,
			Operands: []ast.Node{
				&ast.OperatorExpr{
					Operator: ast.OpAdd,
					Operands: []ast.Node{
						&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processA"}}}},
						&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processB"}}}},
					},
				},
				&ast.FeatureReference{Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processC"}}}},
			},
		},
	}
	final := &ast.FinalNode{}

	// Control flow edges
	edge1 := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel"}}},
	}
	edge2a := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processA"}}},
	}
	edge2b := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processB"}}},
	}
	edge2c := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "parallel"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processC"}}},
	}
	edge3a := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processA"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
	}
	edge3b := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processB"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
	}
	edge3c := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processC"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
	}
	edge4 := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "sync"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "aggregate"}}},
	}
	edge5 := &ast.SuccessionEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "aggregate"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "done"}}},
	}

	// Object flows - each processor writes to named output
	flowA := &ast.ObjectFlowEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processA"}, {Text: "processA"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "aggregate"}, {Text: "processA"}}},
	}
	flowB := &ast.ObjectFlowEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processB"}, {Text: "processB"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "aggregate"}, {Text: "processB"}}},
	}
	flowC := &ast.ObjectFlowEdge{
		Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "processC"}, {Text: "processC"}}},
		Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "aggregate"}, {Text: "processC"}}},
	}

	action := &symbols.Symbol{
		Name: "ParallelProcessing",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "ParallelProcessing"},
			Members: []ast.Node{
				initial, fork, processA, processB, processC, join, aggregate, final,
				edge1, edge2a, edge2b, edge2c, edge3a, edge3b, edge3c, edge4, edge5,
				flowA, flowB, flowC,
			},
		},
	}

	exec, err := newActionExecutor(ctx, action, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	// Set input value
	err = exec.initialize()
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}

	exec.root.data["input"] = Value{
		Kind:  ValConst,
		Const: semantics.Value{Kind: semantics.ValInt, Int: 100},
	}

	// Run to completion
	err = exec.RunToCompletion()
	if err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	// Verify final state
	if exec.state != StateCompleted {
		t.Errorf("expected StateCompleted, got %v", exec.state)
	}

	// Check results - aggregate uses default "result" key since no ObjectFlow specified
	results := exec.Results()
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}

	aggregateVal, ok := results["result"]
	if !ok {
		t.Fatal("expected result from aggregate")
	}

	if aggregateVal.Kind != ValConst || aggregateVal.Const.Kind != semantics.ValInt {
		t.Fatalf("expected int result, got %v", aggregateVal)
	}

	expected := int64(360) // (100+10) + (100+20) + (100+30)
	if aggregateVal.Const.Int != expected {
		t.Errorf("expected aggregate=%d, got %d", expected, aggregateVal.Const.Int)
	}
}

// guardedSuccessionExecutor builds `start → s1 → s2` where the succession out of
// s1 carries guard, which is the `GuardedSuccession` spelling of the graph.
func guardedSuccessionExecutor(t *testing.T, guard ast.Node) *ActionExecutor {
	t.Helper()

	initial := &ast.InitialNode{Name: "start"}
	s1 := &ast.ActionExecutionNode{Name: "s1", Expression: &ast.LiteralInteger{Value: "1"}}
	s2 := &ast.ActionExecutionNode{Name: "s2", Expression: &ast.LiteralInteger{Value: "2"}}

	actionSym := &symbols.Symbol{
		Name: "GuardedSuccessionAction",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "GuardedSuccessionAction"},
			Members: []ast.Node{
				initial, s1, s2,
				&ast.SuccessionEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "start"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "s1"}}},
				},
				&ast.ControlFlowEdge{
					Source: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "s1"}}},
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "s2"}}},
					Guard:  guard,
				},
			},
		},
	}

	exec, err := newActionExecutor(NewContext(NewModel(semantics.NewModel(nil), nil), 1000), actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return exec
}

// A guard on a succession out of a node that is not a decision is evaluated: it
// holds, so the token passes to the successor.
func TestActionExecutor_GuardedSuccession_GuardHolds(t *testing.T) {
	exec := guardedSuccessionExecutor(t, &ast.LiteralBool{Value: true})

	if err := exec.stepToken(0); err != nil { // start → s1
		t.Fatalf("step initial: %v", err)
	}
	if err := exec.stepToken(0); err != nil { // s1 → s2
		t.Fatalf("step s1: %v", err)
	}

	if len(exec.tokens) != 1 {
		t.Fatalf("expected the token to live on, %d left", len(exec.tokens))
	}
	if name := ActionNodeName(exec.tokens[0].Location); name != "s2" {
		t.Errorf("token at %s, want s2", name)
	}
}

// The same succession with a guard that does not hold: the succession is pruned,
// so s2 does not run and the flow ends at s1 rather than failing.
func TestActionExecutor_GuardedSuccession_GuardFails(t *testing.T) {
	exec := guardedSuccessionExecutor(t, &ast.LiteralBool{Value: false})

	if err := exec.stepToken(0); err != nil { // start → s1
		t.Fatalf("step initial: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	if exec.State() != StateCompleted {
		t.Errorf("state = %s, want StateCompleted", exec.State())
	}
	if len(exec.tokens) != 0 {
		t.Errorf("expected every token retired, %d left", len(exec.tokens))
	}
}

// A guard that does not evaluate to a boolean is reported rather than read as
// either verdict.
func TestActionExecutor_GuardedSuccession_NonBooleanGuard(t *testing.T) {
	exec := guardedSuccessionExecutor(t, &ast.LiteralInteger{Value: "42"})

	if err := exec.stepToken(0); err != nil { // start → s1
		t.Fatalf("step initial: %v", err)
	}
	err := exec.stepToken(0)
	if err == nil {
		t.Fatal("a non-boolean guard was accepted")
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("error = %v, want it to wrap ErrTypeMismatch", err)
	}
	if !containsText(err.Error(), "guard must evaluate to boolean") {
		t.Errorf("unexpected error: %v", err)
	}
}

// A guard that cannot be evaluated is reported at the step that reaches it,
// naming the node the succession leaves.
func TestActionExecutor_GuardedSuccession_GuardErrors(t *testing.T) {
	exec := guardedSuccessionExecutor(t, &ast.FeatureReference{
		Name: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "missing"}}},
	})

	if err := exec.stepToken(0); err != nil { // start → s1
		t.Fatalf("step initial: %v", err)
	}
	err := exec.stepToken(0)
	if err == nil {
		t.Fatal("a guard that cannot be evaluated was accepted")
	}
	if !containsText(err.Error(), "eval guard of node s1") {
		t.Errorf("unexpected error: %v", err)
	}
}

// A guard on the succession the flow starts with is evaluated too: the initial
// node's token retires where the guard does not hold.
func TestActionExecutor_GuardedSuccession_OutOfInitialNode(t *testing.T) {
	// `first start if false then s1;`
	initial := &ast.InitialNode{
		Name:      "start",
		Successor: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "s1"}}},
		Guard:     &ast.LiteralBool{Value: false},
	}
	s1 := &ast.ActionExecutionNode{Name: "s1", Expression: &ast.LiteralInteger{Value: "1"}}

	actionSym := &symbols.Symbol{
		Name: "GuardedStart",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "GuardedStart"},
			Members: []ast.Node{
				initial, s1,
			},
		},
	}

	exec, err := newActionExecutor(NewContext(NewModel(semantics.NewModel(nil), nil), 1000), actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}

	if exec.State() != StateCompleted {
		t.Errorf("state = %s, want StateCompleted", exec.State())
	}
	if _, ran := exec.root.data["result"]; ran {
		t.Error("s1 ran although the succession into it was pruned")
	}
}

// A guard on a branch out of a fork prunes that branch: the branches whose guard
// holds, and the unguarded ones, still each get a token.
func TestActionExecutor_GuardedSuccession_PrunesAForkBranch(t *testing.T) {
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	task1 := &ast.ActionExecutionNode{Name: "task1", Expression: &ast.LiteralInteger{Value: "1"}}
	task2 := &ast.ActionExecutionNode{Name: "task2", Expression: &ast.LiteralInteger{Value: "2"}}
	task3 := &ast.ActionExecutionNode{Name: "task3", Expression: &ast.LiteralInteger{Value: "3"}}

	edge := func(source, target string, guard ast.Node) ast.Node {
		ends := func() (*ast.QualifiedName, *ast.QualifiedName) {
			return &ast.QualifiedName{Parts: []ast.NameSegment{{Text: source}}},
				&ast.QualifiedName{Parts: []ast.NameSegment{{Text: target}}}
		}
		from, to := ends()
		if guard == nil {
			return &ast.SuccessionEdge{Source: from, Target: to}
		}
		return &ast.ControlFlowEdge{Source: from, Target: to, Guard: guard}
	}

	actionSym := &symbols.Symbol{
		Name: "GuardedFork",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "GuardedFork"},
			Members: []ast.Node{
				initial, fork, task1, task2, task3,
				edge("start", "split", nil),
				edge("split", "task1", &ast.LiteralBool{Value: true}),
				edge("split", "task2", &ast.LiteralBool{Value: false}),
				edge("split", "task3", nil),
			},
		},
	}

	exec, err := newActionExecutor(NewContext(NewModel(semantics.NewModel(nil), nil), 1000), actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.stepToken(0); err != nil { // start → split
		t.Fatalf("step initial: %v", err)
	}
	if err := exec.stepToken(0); err != nil { // split → branches
		t.Fatalf("step fork: %v", err)
	}

	locations := make(map[ast.Node]bool, len(exec.tokens))
	for _, token := range exec.tokens {
		locations[token.Location] = true
	}
	if len(exec.tokens) != 2 || !locations[task1] || !locations[task3] {
		t.Errorf("fork produced %d tokens at %v, want one each at task1 and task3", len(exec.tokens), locations)
	}
}

// Two successions out of one node whose guards both hold state no order between
// them, so the ambiguity is reported rather than resolved.
func TestActionExecutor_GuardedSuccession_TwoGuardsHold(t *testing.T) {
	initial := &ast.InitialNode{Name: "start"}
	s1 := &ast.ActionExecutionNode{Name: "s1", Expression: &ast.LiteralInteger{Value: "1"}}
	s2 := &ast.ActionExecutionNode{Name: "s2", Expression: &ast.LiteralInteger{Value: "2"}}
	s3 := &ast.ActionExecutionNode{Name: "s3", Expression: &ast.LiteralInteger{Value: "3"}}

	edge := func(source, target string, guard ast.Node) ast.Node {
		ends := func() (*ast.QualifiedName, *ast.QualifiedName) {
			return &ast.QualifiedName{Parts: []ast.NameSegment{{Text: source}}},
				&ast.QualifiedName{Parts: []ast.NameSegment{{Text: target}}}
		}
		from, to := ends()
		if guard == nil {
			return &ast.SuccessionEdge{Source: from, Target: to}
		}
		return &ast.ControlFlowEdge{Source: from, Target: to, Guard: guard}
	}

	actionSym := &symbols.Symbol{
		Name: "AmbiguousGuards",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "AmbiguousGuards"},
			Members: []ast.Node{
				initial, s1, s2, s3,
				edge("start", "s1", nil),
				edge("s1", "s2", &ast.LiteralBool{Value: true}),
				edge("s1", "s3", &ast.LiteralBool{Value: true}),
			},
		},
	}

	exec, err := newActionExecutor(NewContext(NewModel(semantics.NewModel(nil), nil), 1000), actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.stepToken(0); err != nil { // start → s1
		t.Fatalf("step initial: %v", err)
	}

	err = exec.stepToken(0)
	if err == nil {
		t.Fatal("two successions holding at once were traversed without an error")
	}
	if !containsText(err.Error(), "has multiple successors") {
		t.Errorf("unexpected error: %v", err)
	}
}

// A token whose succession out of a merge is pruned is retired, and a later token
// still traverses: the guard drops the link, not the merge.
func TestActionExecutor_GuardedSuccession_PrunedMergeStaysOpen(t *testing.T) {
	initial := &ast.InitialNode{Name: "start"}
	fork := &ast.ForkNode{Name: "split"}
	low := &ast.ActionExecutionNode{Name: "low", Expression: &ast.LiteralInteger{Value: "1"}}
	high := &ast.ActionExecutionNode{Name: "high", Expression: &ast.LiteralInteger{Value: "2"}}
	merge := &ast.MergeNode{Name: "join"}
	after := &ast.ActionExecutionNode{Name: "after", Expression: &ast.LiteralString{Value: "ran"}}

	name := func(text string) *ast.QualifiedName {
		return &ast.QualifiedName{Parts: []ast.NameSegment{{Text: text}}}
	}
	// The succession out of the merge holds only once `high` has written result.
	guard := &ast.OperatorExpr{
		Operator: ast.OpGt,
		Operands: []ast.Node{
			&ast.FeatureReference{Name: name("result")},
			&ast.LiteralInteger{Value: "1"},
		},
	}

	actionSym := &symbols.Symbol{
		Name: "GuardedMerge",
		Kind: symbols.SymbolActionUsage,
		Decl: &ast.Usage{
			Kind:  ast.UsageAction,
			Ident: ast.Identification{Name: "GuardedMerge"},
			Members: []ast.Node{
				initial, fork, low, high, merge, after,
				&ast.SuccessionEdge{Source: name("start"), Target: name("split")},
				&ast.SuccessionEdge{Source: name("split"), Target: name("low")},
				&ast.SuccessionEdge{Source: name("split"), Target: name("high")},
				&ast.SuccessionEdge{Source: name("low"), Target: name("join")},
				&ast.SuccessionEdge{Source: name("high"), Target: name("join")},
				&ast.ControlFlowEdge{Source: name("join"), Target: name("after"), Guard: guard},
			},
		},
	}

	exec, err := newActionExecutor(NewContext(NewModel(semantics.NewModel(nil), nil), 1000), actionSym, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	step := func(at ast.Node) {
		t.Helper()
		for i := range exec.tokens {
			if exec.tokens[i].Location == at {
				if err := exec.stepToken(i); err != nil {
					t.Fatalf("step %s: %v", ActionNodeName(at), err)
				}
				return
			}
		}
		t.Fatalf("no token at %s", ActionNodeName(at))
	}

	step(initial) // start → split
	step(fork)    // split → low, high
	step(low)     // result = 1, low → join
	step(merge)   // guard 1 > 1 does not hold: this arrival is retired
	step(high)    // result = 2, high → join
	step(merge)   // guard 2 > 1 holds: join → after
	step(after)

	if value, ran := exec.root.data["result"]; !ran || value.Str() != "ran" {
		t.Errorf("after did not run: result = %v", value)
	}
}
