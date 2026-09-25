# The action executor

How a token moves through a lowered `ActionGraph`. The behavior as a user sees it is
[guide chapter 6](../../guide/06-behavior.md).

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

**Implementation:** `internal/exec/runtime/action_executor.go`, lowered by
`internal/ir/lower`. The tests that pin each node kind are named in
[internals/testing.md](../testing.md).
