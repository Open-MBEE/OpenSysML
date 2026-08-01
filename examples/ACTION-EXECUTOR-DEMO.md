# Action Executor Demo

This file demonstrates the action executor capabilities implemented in Phase 2. Since the public API (Phase 4) and REPL commands (Phase 5) are not yet implemented, this shows the conceptual execution flow using SysML v2 syntax.

## Current Implementation Status

✅ **Complete:**
- Token-flow execution engine
- All control flow nodes (Initial, Final, Fork, Join, Merge, Decision, Action)
- Guard evaluation with comparison operators
- Expression evaluation with token data scoping
- Comprehensive test coverage (15 ActionExecutor tests)

❌ **Not Yet Implemented:**
- Public execution API (`Context.ExecuteAction()`) - Task 32
- REPL commands (`%action`, `%step`, `%continue`) - Tasks 35-46
- ObjectFlow data routing - Task 9
- Step() and RunToCompletion() APIs - Task 10

## Demo Examples

### Demo 1: Sequential Execution (Initial → Action → Final)

```sysml
action def SequentialAction {
    action start : InitialNode;
    action compute : ActionExecutionNode {
        return 42 * 2;  // Evaluates inline expression
    }
    action end : FinalNode;
    
    succession start then compute then end;
}
```

**Execution Flow:**
1. Token spawns at `start` (InitialNode)
2. Token moves to `compute` (ActionExecutionNode)
3. Expression `42 * 2` evaluates, result stored in token data
4. Token moves to `end` (FinalNode)
5. Token consumed, execution completes

**Internal mechanism:** `stepToken(0)` called 3 times to advance token through graph.

---

### Demo 2: Fork/Join (Parallel Execution)

```sysml
action def ParallelAction {
    action start : InitialNode;
    action fork : ForkNode;
    
    action task1 : ActionExecutionNode { return 10; }
    action task2 : ActionExecutionNode { return 20; }
    action task3 : ActionExecutionNode { return 30; }
    
    action join : JoinNode;
    action end : FinalNode;
    
    succession start then fork;
    succession fork then task1;
    succession fork then task2;
    succession fork then task3;
    succession task1 then join;
    succession task2 then join;
    succession task3 then join;
    succession join then end;
}
```

**Execution Flow:**
1. 1 token at `start`
2. Fork: **1 token → 3 concurrent tokens** (task1, task2, task3)
3. Each token evaluates its action independently
4. Join: **Barrier synchronization** - waits for ALL 3 tokens to arrive
5. Join: **3 tokens → 1 merged token** (data merged via last-write-wins)
6. Token continues to `end`, consumed, completes

**Key semantics:**
- ForkNode creates N tokens (one per outgoing edge), data copied to each
- JoinNode waits until tokens on ALL incoming edges arrive (Petri-net AND-join)
- Data merge uses last-write-wins strategy for conflicting keys

---

### Demo 3: Decision/Merge (Conditional Branching)

```sysml
action def ConditionalAction {
    action start : InitialNode;
    action decision : DecisionNode;
    
    action pathA : ActionExecutionNode { return "took path A"; }
    action pathB : ActionExecutionNode { return "took path B"; }
    
    action merge : MergeNode;
    action end : FinalNode;
    
    succession start then decision;
    flow decision to pathA if (x > 10);   // Guard: x > 10
    flow decision to pathB if (x <= 10);  // Guard: x <= 10
    succession pathA then merge;
    succession pathB then merge;
    succession merge then end;
}
```

**Execution Flow with x=15:**
1. Token at `decision` with data `{x: 15}`
2. DecisionNode evaluates guards in order:
   - Guard `x > 10`: `15 > 10` → **true** → take pathA
3. Token executes `pathA`, stores result
4. Token reaches `merge`
5. MergeNode: **first token wins**, marks merge as visited
6. Token continues to `end`

**Execution Flow with x=5:**
1. Token at `decision` with data `{x: 5}`
2. DecisionNode evaluates guards:
   - Guard `x > 10`: `5 > 10` → false
   - Guard `x <= 10`: `5 <= 10` → **true** → take pathB
3. Token executes `pathB`, stores result
4. Token reaches `merge` (**first to arrive wins**)
5. Token continues to `end`

**Key semantics:**
- DecisionNode evaluates guards with token data in scope
- First true guard wins (deterministic, order-dependent)
- Unguarded edges act as "else" branch (evaluated last)
- MergeNode: first token passes, subsequent tokens discarded (OR-join)

---

### Demo 4: Complex Workflow (Fork → Decision → Merge → Join)

```sysml
action def ComplexWorkflow {
    action start : InitialNode;
    action fork : ForkNode;
    
    // Branch 1: Conditional path
    action decision1 : DecisionNode;
    action branch1A : ActionExecutionNode { return "1A"; }
    action branch1B : ActionExecutionNode { return "1B"; }
    action merge1 : MergeNode;
    
    // Branch 2: Conditional path
    action decision2 : DecisionNode;
    action branch2A : ActionExecutionNode { return "2A"; }
    action branch2B : ActionExecutionNode { return "2B"; }
    action merge2 : MergeNode;
    
    action join : JoinNode;
    action final : ActionExecutionNode { return "complete"; }
    action end : FinalNode;
    
    succession start then fork;
    
    // Branch 1
    succession fork then decision1;
    flow decision1 to branch1A if (x > 50);
    flow decision1 to branch1B;  // else
    succession branch1A then merge1;
    succession branch1B then merge1;
    succession merge1 then join;
    
    // Branch 2
    succession fork then decision2;
    flow decision2 to branch2A if (x > 25);
    flow decision2 to branch2B;  // else
    succession branch2A then merge2;
    succession branch2B then merge2;
    succession merge2 then join;
    
    succession join then final;
    succession final then end;
}
```

**Execution Flow with x=60:**
1. Fork: 1 token → 2 concurrent tokens (branch1, branch2)
2. **Branch 1:** decision1 evaluates `60 > 50` → true → branch1A → merge1
3. **Branch 2:** decision2 evaluates `60 > 25` → true → branch2A → merge2
4. Join: waits for both branches to reach join
5. Join: 2 tokens → 1 merged token
6. Final action executes, token to end, consumed

**Execution Flow with x=30:**
1. Fork: 1 token → 2 concurrent tokens
2. **Branch 1:** decision1: `30 > 50` → false → else branch → branch1B → merge1
3. **Branch 2:** decision2: `30 > 25` → true → branch2A → merge2
4. Join: both branches synchronized
5. Complete

**Execution Flow with x=10:**
1. Fork: 1 token → 2 concurrent tokens
2. **Branch 1:** decision1: `10 > 50` → false → branch1B → merge1
3. **Branch 2:** decision2: `10 > 25` → false → branch2B → merge2
4. Join: both branches synchronized
5. Complete

**Key semantics:**
- Combines all control flow patterns
- Fork creates concurrent execution paths
- Each path independently evaluates decisions
- Merge (OR-join) within each branch
- Join (AND-join) synchronizes parallel branches
- Demonstrates deterministic, compositional semantics

---

## Implementation Details

### Token Flow Architecture

```
Token {
    ID:       int64           // Unique token identifier
    Location: ast.Node        // Current node in graph
    Data:     map[string]Value // Token carries data through execution
}
```

**Execution states:**
- `StateReady` - Executor created, not started
- `StateRunning` - Execution in progress (tokens active)
- `StateCompleted` - All tokens consumed at FinalNode
- `StateSuspended` - Execution paused (for debugging)

### Guard Evaluation

Guards use the runtime expression evaluator with token data:

```go
// Token data: {x: 15, y: 20}
ec := NewEvalContext(ctx)
ec.Push(token.Data)  // Token data becomes evaluation scope
result, _ := ec.Eval(guardExpression)  // Evaluate guard
if result.Kind == ValConst && result.Const.Bool {
    // Guard is true, take this edge
}
```

Supported comparison operators:
- `<` (less than)
- `<=` (less than or equal)
- `>` (greater than)
- `>=` (greater than or equal)
- Integer and real comparisons with automatic coercion

### Test Coverage

Current test suite (15 ActionExecutor tests, all passing):
- `TestActionExecutor_Creation` - Executor initialization
- `TestActionExecutor_GraphExtraction` - AST → graph conversion
- `TestActionExecutor_InitialNode` - Token spawning
- `TestActionExecutor_FinalNode` - Token consumption
- `TestActionExecutor_ActionExecutionNode` - Expression evaluation
- `TestActionExecutor_ForkNode` - Parallel split
- `TestActionExecutor_ForkNode_DataIsolation` - Data copying
- `TestActionExecutor_ForkNode_NoSuccessors` - Error case
- `TestActionExecutor_JoinNode` - Barrier synchronization
- `TestActionExecutor_JoinNode_PartialArrival` - Waiting behavior
- `TestActionExecutor_MergeNode` - OR-join semantics
- `TestActionExecutor_MergeNode_DataDiscard` - Losing token discard
- `TestActionExecutor_MergeNode_SingleParent` - Degenerate case
- `TestActionExecutor_DecisionNode` - Guard evaluation
- `TestActionExecutor_DecisionNode_ElseBranch` - Unguarded fallback

---

## How to Use (When APIs Available)

### Future API (Task 32 - Context.ExecuteAction):

```go
ctx := runtime.NewContext(model, resolver, 1000)
result, err := ctx.ExecuteAction(actionSymbol, initialData)
if err != nil {
    // Handle error
}
// Use result
```

### Future REPL Commands (Tasks 35-46):

```
%load examples/action-executor-demo.sysml
%action SequentialAction
%step         # Step token through graph
%tokens       # View active tokens
%continue     # Run to completion
%state        # View execution state
```

---

## References

- **Implementation:** `internal/core/runtime/action_executor.go` (699 lines)
- **Tests:** `internal/core/runtime/action_executor_test.go` (2244 lines, 35 tests passing)
- **Architecture:** See [../docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) for complete design
- **API Documentation:** See [../docs/API.md](../docs/API.md) for runtime APIs
- **More Examples:** See [state-machine-demo.sysml](state-machine-demo.sysml) and [combined-behavioral-demo.sysml](combined-behavioral-demo.sysml)

---

**Status:** Action executor fully functional with comprehensive test coverage, REPL debugging commands, and integration tests complete.
