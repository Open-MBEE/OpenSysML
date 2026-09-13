# Orthogonal Regions Implementation Design

## Overview

Implement orthogonal regions (concurrent substates) for state machines. The notation is standard:
a state whose body is marked `parallel` (`SysML.xtext:1745`, `StateUsage::isParallel`) makes each of
its state substates a region. The bundled libraries have no region performance, so UML 2.5.1
§14.2.3.3 is the reference semantics. The nearest library anchor is that only
`States.sysml` `exclusiveStates` are sequenced (`succession stateSequencing first [0..1]
exclusiveStates then [0..1] exclusiveStates`), so substates outside that set are not ordered
against each other; concurrency across regions is not derived from it.

**Goal:** Support multiple simultaneously active substates within a composite state (AND-composition).

## Current State vs Target

### Current (Single Active State)
```
StateExecutor:
  currentState: *StateNode  // Single active state
  stateStack: []*StateNode  // Hierarchical chain (parent→child)
```

### Target (Active Configuration Set)
```
StateExecutor:
  activeConfig: *StateConfiguration  // Active state configuration
  
StateConfiguration:
  regionStates: map[*StateRegion]*StateNode  // One active state per region
  hierarchyStack: []*StateNode               // Parent chain (unchanged)
```

## Key Changes

### 1. Active Configuration Model

**Replace:**
- `currentState *ast.StateNode` (single)

**With:**
- `activeConfig *StateConfiguration` (multi-region)

```go
type StateConfiguration struct {
    // For states WITH regions: map of region → active state in that region
    regionStates map[*StateRegion]*ast.StateNode
    
    // For states WITHOUT regions: single active state (nil if multi-region)
    simpleState *ast.StateNode
    
    // Hierarchical parent chain (unchanged from current)
    hierarchyStack []*ast.StateNode
}
```

**Rationale:** 
- Simple states work as before (simpleState field)
- Composite states with regions use regionStates map
- Backward compatible: existing single-state logic preserved

### 2. Region Structure Identification

**During extractGraph():**

```go
func (e *StateExecutor) extractGraph() error {
    // ...existing state collection...
    
    // NEW: For each state, check if it has regions
    for _, state := range e.states {
        if len(state.Regions) > 0 {
            // Mark state as composite with regions
            e.compositeStates[state] = state.Regions
            
            // Each region must have initial state
            for _, region := range state.Regions {
                initialState := findInitialInRegion(region)
                if initialState == nil {
                    return fmt.Errorf("region %s has no initial state", region.Name)
                }
                e.regionInitials[region] = initialState
            }
        }
    }
}
```

**New fields needed:**
- `compositeStates map[*StateNode][]*StateRegion` - states with orthogonal regions
- `regionInitials map[*StateRegion]*StateNode` - initial state per region

### 3. Event Processing (Broadcast to All Regions)

**Current (single state):**
```go
func (e *StateExecutor) ProcessNextEvent() error {
    event := e.eventQueue.Dequeue()
    
    // Check transitions from currentState
    for _, trans := range e.transitions[e.currentState] {
        if e.matchesTransition(trans, event) {
            e.fireTransition(trans, event)
            return nil
        }
    }
}
```

**Target (multi-region):**
```go
func (e *StateExecutor) ProcessNextEvent() error {
    event := e.eventQueue.Dequeue()
    
    // If in composite state with regions, broadcast to all
    if len(e.activeConfig.regionStates) > 0 {
        for region, regionState := range e.activeConfig.regionStates {
            // Check transitions from this region's active state
            for _, trans := range e.transitions[regionState] {
                if e.matchesTransition(trans, event) {
                    e.fireTransitionInRegion(region, trans, event)
                    break  // Run-to-completion within region
                }
            }
        }
    } else {
        // Simple state (no regions) - existing logic
        currentState := e.activeConfig.simpleState
        for _, trans := range e.transitions[currentState] {
            if e.matchesTransition(trans, event) {
                e.fireTransition(trans, event)
                return nil
            }
        }
    }
}
```

**Key insight:** Events broadcast to all regions. Each region processes event independently (run-to-completion per region).

**As implemented (`state_executor.go`):** the order the regions react in is not the map's
iteration order above, nor declaration order by rule. `broadcastEvent` first selects one
candidate per active leaf against the configuration the event was dequeued for
(`selectTransitions`: the innermost state on the leaf's parent chain with an enabled transition,
so a state the event enters never reacts to it), drops the candidates a nested transition
outranks, then hands the survivors to `dispatchInOrder`. That loop fires them one at a time: before
each firing it discards the candidates whose leaf is no longer active — a reaction may have left
a sibling's leaf — and draws which of the rest fires next through `chooseRegion`, which asks the
run's scheduling policy and, when two or more remain, records a `ChoicePoint` of kind
`ChoiceRegionOrder` ahead of the firing's own notes (in a trace,
`choice on go: states left, right react (unordered; took right first)`). Each
`StateTransitionPerformance` is ordered against its own source and target only
(`StatePerformances.kerml`), and UML 2.5.1 §14.2.3.9.4 leaves the firing order of the transitions
selected for one event undefined, so the order is a genuine opening the run reports rather than a
rule: under `reverse` and `declared` the draw is declaration order and is still reported,
`seed:<n>` varies and replays it, and `explore` enumerates every order. The choice is labelled by
the occurrence dispatched (`on go`, `on change`), not by the trigger of whichever region was
drawn first, so the label reads the same under every policy. Change triggers
(`state_change_trigger.go`, `pollChangeEvents`) select their candidates by the guards that came to
hold and dispatch through the same loop. See
[scheduling policies, choice points and exploration](scheduling.md).

**Deferral against a sibling region.** Before the survivors are dispatched, `selectTransitions`
asks `deferralOutranks`: a state in the active configuration that defers the occurrence
(`deferringStates`, over `StateGraph.Deferred`) holds it back from every candidate whose source is
not that state or a state nested in it (`encloses`). A transition in a sibling region or in an
enclosing state therefore waits: the occurrence is deferred, and recalled — with its arrival
identity and the current clock time, ahead of the events that arrived meanwhile — once the
deferring state is exited, into whatever configuration that exit leaves, where the sibling's
transition then fires (`state_deferral_outranks_sibling_region`,
`state_deferral_outranks_enclosing_state`). Only a transition nested in the deferring state
overrides its deferral and consumes the occurrence (`state_deferral_nested_override`). With two
orthogonal regions each holding a deferring state, the rule applies to each deferring state: the
occurrence fires only if *every* deferring state has an overriding transition nested in it, else it
is deferred whole, a nested override in one region included
(`TestDeferralInEachRegionMustBeOverriddenForTheEventToFire`); a deferring state nested deeper than
the sibling's transition outranks it all the same
(`state_deferral_nested_outranks_sibling_region`). The outcome is determined by the
configuration, so it is never a choice point; a do behavior's `accept` still takes the occurrence
ahead of any deferral.

**Completion of a composite state.** `done` written in a composite state's body is that state's
own end (`States.sysml`: `done` is `StatePerformance::endShot`), so when every region of a
composite state has reached its `done` and its do behavior has ended, the composite *completes*
(`completeIfDone` → `scheduleCompletedComposites`): its nil-trigger transitions are queued as
completion events at the current instant, exactly as a leaf's are, and the machine ends only when
its own top-level regions (`TopRegions`) are all at `done`. A completed composite with no enabled
completion transition stays completed and active — its triggered transitions still fire and the
machine runs on (`state_completion_nested_regions_stay_active`). A region left at `done` leaves
no history record, so a later history transition into it is a default entry.

### 4. State Entry/Exit with Regions

**Entering composite state with regions:**
```go
func (e *StateExecutor) enterState(state *StateNode) error {
    // Execute entry actions (existing)
    e.executeEntryActions(state)
    
    // NEW: If state has regions, enter initial state in each region
    if regions, isComposite := e.compositeStates[state]; isComposite {
        e.activeConfig.regionStates = make(map[*StateRegion]*StateNode)
        for _, region := range regions {
            initialState := e.regionInitials[region]
            e.activeConfig.regionStates[region] = initialState
            e.enterState(initialState)  // Recursive entry
        }
    } else {
        // Simple state (no regions)
        e.activeConfig.simpleState = state
    }
    
    // Execute do behavior (existing)
    e.executeDoActivity(state)
}
```

**Exiting composite state with regions:**
```go
func (e *StateExecutor) exitState(state *StateNode) error {
    // NEW: If state has regions, exit active state in each region
    if len(e.activeConfig.regionStates) > 0 {
        for _, regionState := range e.activeConfig.regionStates {
            e.exitState(regionState)  // Recursive exit
        }
        e.activeConfig.regionStates = nil
    }
    
    // Execute exit actions (existing)
    e.executeExitActions(state)
    e.activeConfig.simpleState = nil
}
```

### 5. Fork/Join Transitions (Cross-Region)

**Fork transition (1 → N):**
- Source: single state in one region (or no region)
- Target: multiple states across different regions

**Join transition (N → 1):**
- Source: specific states in all regions (AND-join)
- Target: single state (possibly exiting composite state)

**Implementation:**

```go
type TransitionEdge struct {
    // ... existing fields ...
    
    // NEW: Multi-target for fork transitions
    Targets []*QualifiedName  // Multiple targets (fork)
    
    // NEW: Multi-source for join transitions
    Sources []*QualifiedName  // Multiple sources (join, all must be active)
}

func (e *StateExecutor) fireTransition(trans *TransitionEdge, event *Event) error {
    // FORK: Single source, multiple targets
    if len(trans.Targets) > 1 {
        sourceState := e.resolveState(trans.Source)
        e.exitState(sourceState)
        
        for _, targetName := range trans.Targets {
            targetState := e.resolveState(targetName)
            targetRegion := e.findRegionForState(targetState)
            e.activeConfig.regionStates[targetRegion] = targetState
            e.enterState(targetState)
        }
        return nil
    }
    
    // JOIN: Multiple sources, single target (check all active)
    if len(trans.Sources) > 1 {
        // Check all source states are active in their respective regions
        for _, sourceName := range trans.Sources {
            sourceState := e.resolveState(sourceName)
            sourceRegion := e.findRegionForState(sourceState)
            if e.activeConfig.regionStates[sourceRegion] != sourceState {
                return nil  // Join condition not met
            }
        }
        
        // All sources active, perform join
        for _, sourceName := range trans.Sources {
            sourceState := e.resolveState(sourceName)
            e.exitState(sourceState)
        }
        
        targetState := e.resolveState(trans.Target)
        e.enterState(targetState)
        return nil
    }
    
    // Regular transition (existing logic)
    // ...
}
```

## Notation and lowering

The notation carries no region member: orthogonality is the `parallel` marker before a state body,
and each state substate of a parallel body is one region, in declaration order.

```sysml
state working parallel {
    state left { entry; then building; state building; }
    state right { entry; then checking; state checking; }
}
```

`internal/core/lower/state_graph.go` synthesizes one `ast.StateRegion` per state substate
(`parallelRegions`, `isParallelRegionMember`), named after that substate, and fills `RegionOf`,
`RegionOwner`, `RegionInitials` and `CompositeStates` from it. The parallel state itself keeps what
it declares directly — its behaviors, `defer`, its pseudostates and the edges between them — so a
choice its regions branch through is owned by the parallel state, not by a region.

## Test Cases

### Conformance Test: state_orthogonal_regions.sysml

```sysml
state def TrafficLight parallel {
    state pedestrian {
        entry; then start;
        state start;
        state Walk;
        state DontWalk;
        first start then Walk;
        transition first Walk when timer > 5 then DontWalk;
    }
    
    state vehicle {
        entry; then start;
        state start;
        state Green;
        state Yellow;
        state Red;
        first start then Green;
        transition first Green when timer > 30 then Yellow;
        transition first Yellow when timer > 3 then Red;
        transition first Red when timer > 20 then Green;
    }
}
```

**Expected behavior:**
- Both regions active simultaneously
- `Walk` + `Green` initially active
- Events processed independently in each region
- No coordination between regions (unless explicit join transition)

### Conformance Test: state_fork_join.sysml

```sysml
state def Parallel {
    entry; then start;
    state start;
    state Sequential;
    
    state Composite parallel {
        state A {
            entry; then startA;
            state startA;
            state TaskA;
            first startA then TaskA;
        }
        
        state B {
            entry; then startB;
            state startB;
            state TaskB;
            first startB then TaskB;
        }
    }
    
    state Done;
    
    // FORK: Sequential → Composite (enters both regions)
    transition first Sequential when fork then Composite;
    
    // JOIN: Both regions must be in specific states
    transition join {
        from TaskA, TaskB;
        to Done;
        when allDone;
    }
}
```

## Implementation Plan

1. **Phase 1:** Synthesize a region per state substate of a `parallel` body when lowering
2. **Phase 2:** Add StateConfiguration struct + activeConfig field
3. **Phase 3:** Refactor enterState/exitState to handle regions
4. **Phase 4:** Update ProcessNextEvent to broadcast events
5. **Phase 5:** Implement fork/join transitions
6. **Phase 6:** Create conformance tests
7. **Phase 7:** Update documentation

## Backward Compatibility

**Guaranteed:**
- States without regions work exactly as before (use simpleState field)
- Existing tests continue to pass (no regions defined)
- Hierarchical states (parent/child) unchanged
- stateStack for history tracking preserved

## UML 2.5.1 Compliance (reference semantics for this extension)

**Implemented:**
- §14.2.3.3.1: Composite states with orthogonal regions
- §14.2.3.3.2: Active configuration (one state per region)
- §14.2.3.3.3: Event broadcast to all regions
- §14.2.3.3.4: Fork transitions (1→N)
- §14.2.3.3.5: Join transitions (N→1 with AND condition)

**Not implemented (future):**
- Transition priorities within regions beyond the nesting rule (a nested transition and a nested
  override of a deferral outrank an enclosing state's)
- Inter-region communication (requires ports)
- Region-specific event queues (single queue sufficient for most cases)
