# Lowering Layer Integration Plan

## Status: COMPLETE ✅

**Phase 1**: Explicit execution IR created
- `internal/ir/lower/` package exists
- `ActionGraph` and `StateGraph` IR types defined
- `ToActionGraph()` and `ToStateGraph()` conversion functions working
- Comprehensive test coverage (5 tests: simple, fork/join, regions, pseudostates)

**Phase 2**: Executors refactored to consume IR
- ActionExecutor uses `graph *lower.ActionGraph`
- StateExecutor uses `graph *lower.StateGraph`
- All field references updated (nodes/edges/guards/dataFlows → graph.*)
- `extractGraph()` functions deleted from both executors
- Pseudostate transitions route through graph (not AST re-parse)

**Phase 3**: Tests validated
- All 23 conformance tests passing
- Transition effects preserved (state_transition_effect validates)
- Orthogonal regions working (state_orthogonal_regions validates)
- Pseudostates working (state_choice_pseudostate, state_junction_pseudostate)

## Implementation Summary

### Task 1: Non-fatal lowering errors
- Lowerers return graphs even without initial nodes
- Executors validate at `initialize()` time
- Preserves constructor-succeeds/initialize-errors contract

### Task 2: Nested state resolution
- Parser uses `parseQualifiedNameRelaxed()` for transition source/target (allows keywords)
- Lowerer handles three state forms: StateNode, SubstateMember, StateRegion
- Transition.Source/Target changed to ast.Node (supports StateNode + PseudostateNode)

### Task 3: Top-level orthogonal regions
- Lowerer collects states from region.States
- RegionInitials populated for top-level regions
- Initial only searched if not a region-based machine

### Task 4: Preserve transition effects
- populateFromGraph() copies Effect field to TransitionEdge
- Transition effects execute correctly (validated by conformance test)

### Task 5: Pseudostate transitions through graph
- findTransitionsFromPseudostate() uses e.graph.Transitions[ps]
- Eliminates AST re-parsing for choice/junction outgoing edges
- Single source of truth for all transitions

### Task 6: Dead code cleanup
- All `extractGraph()` functions deleted
- No unused code (verified with go vet)

### Task 7: Test coverage
- Added TestToActionGraph_ForkJoinMergeDecision
- Added TestToStateGraph_Regions
- Added TestToStateGraph_Pseudostates
- Total: 5 lowering tests, all passing

## Architecture Achieved

### Before: Two disconnected vocabularies
```
Parser emits:          Executors consume:
- TransitionMember     - TransitionEdge
- SubstateMember       - StateNode
- EntryMember          - direct AST fields
```

Executors had ad-hoc `extractGraph()` that tried to bridge the gap, but it was:
- Incomplete (dropped Effect fields)
- Inconsistent (some transitions from graph, some from AST re-parse)
- Untestable (no validation until execution)

### After: Explicit lowering layer
```
Parser → AST → lower.ToActionGraph/ToStateGraph → ActionGraph/StateGraph → Executors
```

Lowering layer:
- **Validates** structure (initial nodes, no dangling edges)
- **Normalizes** syntax variations (StateNode, SubstateMember, StateRegion)
- **Preserves** all semantics (effects, guards, triggers)
- **Testable** independent of executors

## What This Enables

1. **Conformance testing**: Can validate state machine execution end-to-end
2. **Error timing**: Structural errors at lowering time, runtime errors at initialize/step time
3. **Debugging**: Can inspect IR between parse and execute
4. **Evolution**: Parser and executor changes decoupled via stable IR

## Remaining Work

These were not in scope for the lowering layer (future enhancements):
- Convert executor unit tests from hand-built ASTs to parse-and-execute
- Eliminate backward-compatibility bridge (populateFromGraph → direct graph consumption)
- Full finalState/stateVisits conformance validation
