# Action Executor Demo

This directory contains demonstrations of the **action executor** implementation (Phase 2 complete).

## Files

- **`ACTION-EXECUTOR-DEMO.md`** - Comprehensive demo with 4 example workflows
  - Sequential execution (Initial → Action → Final)
  - Fork/Join (parallel execution with synchronization)
  - Decision/Merge (conditional branching)
  - Complex workflow (combining all patterns)

- **`action-executor-demo.sysml`** - SysML v2 syntax examples (for reference)

## What Works ✅

**Complete token-flow execution engine:**
- InitialNode - spawns tokens
- FinalNode - consumes tokens
- ActionExecutionNode - evaluates expressions
- ForkNode - parallel split (1 → N tokens)
- JoinNode - barrier synchronization (N → 1 token)
- MergeNode - OR-join (first-wins)
- DecisionNode - conditional branching with guards

**Features:**
- Petri-net semantics (token flow)
- Guard evaluation with comparison operators
- Expression evaluation with token data
- Data copying (fork) and merging (join)
- Deterministic execution

**Test Coverage:**
- 16 ActionExecutor tests
- 58 total runtime tests
- All passing ✓

## How to Verify

Run the action executor tests:

```bash
cd internal/core/runtime
go test -v -run TestActionExecutor
```

Expected output: All tests pass (16/16)

## What's Next

**Phase 2 Completion:**
- Task 9: ObjectFlowEdge data routing
- Task 10: Step() and RunToCompletion() APIs
- Task 11: Deadlock detection
- Tasks 12-16: Integration tests

**Phase 3: State Executor**
- Event-driven state machines
- TimeEvent, ChangeEvent, AcceptEvent
- Hierarchical states
- Transition guards

**Phase 4-5: Integration**
- Public APIs (Context.ExecuteAction)
- REPL commands (%action, %step, %continue, etc.)

## References

- **Implementation:** `internal/core/runtime/action_executor.go` (699 lines)
- **Tests:** `internal/core/runtime/action_executor_test.go` (2244 lines, 35 tests)
- **Architecture:** [../docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md) — Complete Tier 5 design
- **API Documentation:** [../docs/API.md](../docs/API.md) — Runtime APIs with examples
- **Demo Files:** [action-executor-demo.sysml](action-executor-demo.sysml), [state-machine-demo.sysml](state-machine-demo.sysml)
- **Examples Guide:** [README.md](README.md) — Complete demo reference

**Status:** Action executor, state executor, REPL debugging, and integration tests all complete. 116 tests passing.
