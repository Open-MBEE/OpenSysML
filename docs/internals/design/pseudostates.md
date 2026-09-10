# Pseudostates Design - Choice and Junction

**Status:** Implemented — choice, junction, fork, join, entry/exit points and history, including pseudostates reached from inside an orthogonal region  
**Semantic Reference:** choice and junction are KerML performances — `ControlPerformances::DecisionPerformance` (`outgoingHBLink: HappensBefore[1]`) and `MergePerformance` (`incomingHBLink: HappensBefore[1]`), with `StatePerformances::StatePerformance specializes DecisionPerformance`. Fork/join in a state body, history and entry/exit points have no SysML v2 notation and no KerML performance, so UML 2.5.1 §14.2.3.4 (Pseudostates) is their reference semantics only; the notation for all of them is an OpenSysML extension.

## Overview

Pseudostates are transient vertices in state machines that enable complex control flow. This document covers **choice** and **junction** pseudostates in detail, and the routing and history semantics of the remaining kinds below. The textual notation for every kind is tabulated in `docs/reference/grammar/README.md`.

### Choice vs Junction

**Choice (Dynamic Conditional Branch):**
- Guards evaluated **when entered** (runtime decision)
- Used for dynamic branching based on current conditions
- Outgoing transitions evaluated in order until one guard succeeds
- Must have at least one guard that evaluates to true (or else clause)

**Junction (Static Merge/Branch):**
- Guards evaluated **before entering** (static connectors)
- Used to merge multiple incoming transitions or split paths
- All outgoing guards must be mutually exclusive and complete
- Deterministic - no runtime evaluation order

### Semantics

KerML gives the branching itself: a `DecisionPerformance` "represents the selection of one of
the Successions that have the DecisionPerformance behavior as their source", and
`outgoingHBLink: HappensBefore[1]` makes that exactly one — a branching point is left by exactly
one succession, choice and junction alike. Since `StatePerformance specializes
DecisionPerformance`, a state is left the same way. What KerML does not distinguish is *when* the
guards are read, which is the choice/junction difference below; UML 2.5.1 §14.2.3.4.3-4 is the
reference for that distinction:

**Choice:**
> "A choice vertex is a dynamic conditional branch. The guards on the outgoing transitions are evaluated only when the choice is entered, at run time."

**Junction:**
> "A junction vertex is a static conditional branch. The guards on the outgoing transitions are evaluated when the incoming transition(s) are evaluated."

## Syntax

### SysML v2 / KerML Syntax

**Choice:**
```sysml
state def TrafficController {
    state Green;
    state Yellow;
    state Red;
    
    choice priorityCheck;
    
    transition first Green then priorityCheck;
    transition first priorityCheck if (emergencyVehicle) then Yellow;
    transition first priorityCheck if (not emergencyVehicle) then Red;
}
```

**Junction:**
```sysml
state def SafetyMonitor {
    state Nominal;
    state Warning;
    state Critical;
    
    junction statusEval;
    
    transition first statusEval if (temp < 50) then Nominal;
    transition first statusEval if (temp >= 50 and temp < 100) then Warning;
    transition first statusEval if (temp >= 100) then Critical;
}
```

**History, entry/exit points and deferral** (an OpenSysML extension — the OMG
textual notation has no production for pseudostates or for deferral; see
`docs/reference/grammar/README.md`):
```sysml
state def Player {
    state playing {
        state track;
        state paused;

        defer Skip;            // retained while `playing` is active
        history resume;        // shallow, UML's H; `deep history` is H*
        entry point start;
        exit point stop;
    }
    state stopped;

    transition first stopped when Resume then resume;
    transition first resume then track;  // default history transition
}
```

## Implementation Plan

### Phase 1: Parser Support

**Add to `parseStateMember()` in behavior.go:**
- Detect `choice` keyword
- Detect `junction` keyword
- Parse pseudostate name
- Create `PseudostateNode` with appropriate `Kind`

**Changes:**
- Add `choice` and `junction` to state member keyword dispatch
- Add `parseChoicePseudostate()` function
- Add `parseJunctionPseudostate()` function

### Phase 2: State Executor Graph Extraction

**Update `extractGraph()` in state_executor.go:**
- Collect `PseudostateNode` instances
- Add to `states` map (pseudostates are vertices like states)
- Extract transitions targeting pseudostates

**Data structures:**
- Add `pseudostates map[string]*ast.PseudostateNode`
- Transitions already support guard expressions via `TransitionMember.Guard`

### Phase 3: Runtime Evaluation

**Choice Evaluation:**
```go
func (e *StateExecutor) evaluateChoice(choiceName string, event Event) (string, error) {
    // Get all outgoing transitions from choice
    outgoing := e.getOutgoingTransitions(choiceName)
    
    // Evaluate guards in order (runtime)
    for _, trans := range outgoing {
        if trans.Guard == nil {
            return trans.Target, nil // else clause
        }
        
        satisfied, err := e.evaluateGuard(trans.Guard)
        if err != nil {
            return "", err
        }
        if satisfied {
            return trans.Target, nil
        }
    }
    
    return "", fmt.Errorf("no guard satisfied at choice %s", choiceName)
}
```

**Junction Evaluation:**
- Similar to choice but guards evaluated before entering junction
- Used in `fireTransition()` when source transition leads to junction
- All guards must be mutually exclusive (validation)

### Phase 4: Transition Firing Integration

**Update `fireTransition()` in state_executor.go:**

```go
// After exiting source state
target := transition.Target

// Check if target is pseudostate
if ps, ok := e.pseudostates[target]; ok {
    switch ps.Kind {
    case ast.PseudostateChoice:
        // Evaluate guards at runtime
        nextTarget, err := e.evaluateChoice(target, event)
        if err != nil {
            return err
        }
        target = nextTarget
        
    case ast.PseudostateJunction:
        // Guards already evaluated, pick deterministic path
        nextTarget, err := e.evaluateJunction(target)
        if err != nil {
            return err
        }
        target = nextTarget
    }
}

// Enter final target state
e.enterState(target)
```

### Phase 5: Conformance Tests

**Test: state_choice_pseudostate.sysml**
```sysml
package ChoiceTest {
    state def Controller {
        attribute priority : Integer = 0;
        
        state Idle;
        state HighPriority;
        state LowPriority;
        
        choice checkPriority;
        
        entry; then init;
        state init;
        transition init then Idle;
        transition first Idle accept Start then checkPriority;
        transition first checkPriority if (priority > 5) then HighPriority;
        transition first checkPriority if (priority <= 5) then LowPriority;
    }
    
    state sm : Controller;
}
```

**Expected output:**
```json
{
  "finalState": "LowPriority",
  "stateVisits": ["Idle", "LowPriority"],
  "outputs": {}
}
```

**Test: state_junction_pseudostate.sysml**
```sysml
package JunctionTest {
    state def Monitor {
        attribute temp : Integer = 75;
        
        state Nominal;
        state Warning;
        state Critical;
        
        junction statusEval;
        
        entry; then init;
        state init;
        transition init then statusEval;
        transition first statusEval if (temp < 50) then Nominal;
        transition first statusEval if (temp >= 50 and temp < 100) then Warning;
        transition first statusEval if (temp >= 100) then Critical;
    }
    
    state sm : Monitor;
}
```

**Expected output:**
```json
{
  "finalState": "Warning",
  "stateVisits": ["Warning"],
  "outputs": {}
}
```

### Phase 6: Documentation

**Update SPEC_COMPLIANCE.md:**
- Move choice/junction from "Advanced" to "Implemented"
- State Machines: 14/14 → 16/16 features
- Add test coverage: 21 → 23 conformance cases

## Validation Rules

1. **Choice:**
   - Must have at least one outgoing transition
   - At least one guard must be satisfiable (or else clause)
   - Guards evaluated in definition order

2. **Junction:**
   - Outgoing guards must be mutually exclusive
   - Guards must be complete (cover all cases)
   - No runtime evaluation order (deterministic)

3. **Both:**
   - Cannot have entry/exit/do behaviors
   - Must be transient (no dwelling)
   - Incoming transitions can have guards
   - Outgoing transitions must have guards (except else clause)

## Backward Compatibility

✅ No breaking changes:
- Existing state machines continue to work
- Pseudostates are additive features
- No changes to state transition semantics

## References

- KerML `ControlPerformances::DecisionPerformance` / `MergePerformance` (stdlib `Kernel Libraries/Kernel Semantic Library/ControlPerformances.kerml`) — one outgoing / one incoming `HappensBefore` link
- KerML `StatePerformances::StatePerformance specializes DecisionPerformance`
- UML 2.5.1 §14.2.3.4.3-4 (Choice / Junction Pseudostates) — for guard evaluation timing only
- SysML v2 Pilot Implementation (state machine examples)

## Fork and Join

Fork and join are declared like choice and junction:

```sysml
state Machine {
    entry; then init;
    state init;
    state idle;
    state running parallel {
        state left  { entry; then ls; state ls; state working;  succession first ls then working; }
        state right { entry; then rs; state rs; state watching; succession first rs then watching; }
    }
    fork split;
    join sync;

    succession first init then idle;
    transition first idle then split;
    transition first split then working;  // one branch per region
    transition first split then watching;
    transition first working then sync;
    transition first watching then sync;
    transition first sync then done;
}
```

**Fork** (`fireForkTransition`): every outgoing branch is taken at once. Branches
must be unguarded and must target states in distinct orthogonal regions of one
composite state; that composite state is entered and each region's active state
becomes its branch target instead of the region's initial state.

**Join** (`fireJoinTransition`): the transition only proceeds once the source
state of every incoming branch is active. A branch that arrives early leaves its
source state active and waits, so the join synchronizes the regions before the
composite state is exited.

**Entry/exit points** (`ast.PseudostateEntry`, `ast.PseudostateExit`) are routed
like a junction — the transition continues along the point's own outgoing
transition. They are declared `entry point <name>;` and `exit point <name>;` in a
state body.

**History** (`ast.PseudostateShallowHistory`, `ast.PseudostateDeepHistory`) is
owned by the composite state it restores — `lower.StateGraph.PseudostateOwner`
records that ownership — and re-enters the configuration that state was last left
in. The configuration is recorded on the way out — the substate that was active by
`exitState`, and the active state of each orthogonal region by `recordRegionHistory`
as that region is left, whether by `exitState`, `leaveRegion` or `exitRegionTo`. A
region left with no active state has its record dropped (`forgetRegionHistory`),
since there is nothing to restore.

`fireHistoryTransition` reads that record before leaving the source
configuration, since a transition out of the owner's own substates would
otherwise overwrite it, then:

- a **shallow** history enters the recorded substate, or re-enters the owner with
  one branch per region holding that region's recorded state;
- a **deep** history keeps following the record downwards (`deepestRecorded`),
  collecting a branch for every nested region it passes, so the innermost recorded
  state is the one entered;
- when the owner has never been exited there is nothing to restore, so the
  history's own outgoing transition supplies the target — UML's default history
  transition, UML being the reference semantics for history, which SysML v2 and
  KerML have no counterpart for.

Entering a branch nested below a region runs the entry behaviors of the states
above it inside that region, so a restored deep configuration is not entered
sideways.

History is declared `history <name>;` (shallow, as UML's `H`), `shallow history
<name>;` or `deep history <name>;` in the body of the composite state it
restores.

## Pseudostates Reached From Inside an Orthogonal Region

`state_region_transition.go` implements transitions whose source is the active
state of an orthogonal region, which is how a choice, junction or entry/exit
point declared beside the regions is reached from inside one. Where the branch
the pseudostate routes along ends decides how much of the configuration moves,
per UML's least common ancestor rule:

```sysml
state def RegionChoice parallel {
    attribute mode : Integer = 2;

    state left  { entry; then lstart; state lstart; state lidle; state lfast; state lslow;
                  succession first lstart then lidle; transition first lidle then pick; }
    state right { entry; then rstart; state rstart; state rwatch; succession first rstart then rwatch; }

    choice pick;

    transition first pick if mode == 2 then lfast;  // stays in region left
    transition first pick then lslow;  // else branch
}
```

- **`pseudostateTarget`** follows the chain of transient pseudostates (choice,
  junction, entry point, exit point) from the transition's target until a state
  is reached, so `exit point → junction → state` enters that state. A chain that
  routes back into a pseudostate it already passed, a branch with no satisfied
  guard, and a branch into a fork, join or history all return typed errors rather
  than leaving the machine resting on a pseudostate.
- **`moveBetweenRegions`** handles a branch ending inside the source region or
  inside a region concurrent with it, which `concurrentRegionsFor` and
  `siblingRegionContaining` classify. KerML `StateTransitionPerformance` orders
  only `guard then transitionLinkSource.exit`, so `exitRegionTo` exits the source
  and its descendants down to the boundary the target region keeps: the state
  owning the regions is neither exited nor re-entered, and the regions holding
  neither endpoint keep the states they were in, entry behaviors included. A
  target nested inside a composite state the target region is already running
  moves that inner region (`innermostActiveRegion`); a source nested deeper than
  the target's region leaves its region set up to the level the two share, which
  `enclosingRegion` finds even when the region's owning state is a substate.
- **`leaveRegion`** handles a branch ending outside every enclosing region set.
  The least common ancestor is then above the region set, so the whole set is
  left: every region is exited in declaration order — recording its
  configuration, so a later history transition restores it — before the composite
  state and its ancestors up to the least common ancestor are exited and the
  target is entered, entering that target's own regions on the way.
- **`leaveTopRegions`** does the same for the machine's own regions, which no
  state owns. `lower.StateGraph.TopRegions` carries their declaration order, so
  the order regions are entered and exited in is the declared one and never map
  iteration order; the order their selected reactions to one event fire in is a
  choice point the run's scheduling policy draws
  ([orthogonal regions](orthogonal-regions.md), [scheduling](scheduling.md)).

Fork, join and history reached from inside a region are fired whole
(`fireTransition`), since they rewrite the configuration rather than move one
region.

### Known limitations

- A junction's guards are evaluated when it is reached, like a choice's, rather
  than statically together with its incoming transition. The two differ only for
  guards over data an effect on the incoming transition changes.
- Entry points, exit points and history are an OpenSysML extension to the OMG
  textual notation, which has no production for any pseudostate; see
  `docs/reference/grammar/README.md`.
