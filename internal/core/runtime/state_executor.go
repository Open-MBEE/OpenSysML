package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// StateExecutor executes state machines using event-driven semantics.
type StateExecutor struct {
	ctx          *Context
	stateMachine *symbols.Symbol
	state        ExecutionState
	
	// State machine execution state
	currentState *ast.StateNode
	currentTime  float64
	eventQueue   *EventQueue
	stateData    map[string]Value // State machine local variables
	
	// Graph structure
	states      []*ast.StateNode
	transitions map[*ast.StateNode][]*ast.TransitionEdge
	
	// Hierarchical state support
	stateStack  []*ast.StateNode            // Active state configuration (for nested states)
	parentState map[*ast.StateNode]*ast.StateNode // Child -> parent mapping
}

// newStateExecutor creates a state executor.
func newStateExecutor(ctx *Context, stateMachine *symbols.Symbol) (*StateExecutor, error) {
	if stateMachine.Kind != symbols.SymbolStateUsage && stateMachine.Kind != symbols.SymbolStateDef {
		return nil, fmt.Errorf("symbol %s is not a state machine", stateMachine.Name)
	}
	
	exec := &StateExecutor{
		ctx:          ctx,
		stateMachine: stateMachine,
		state:        StateReady,
		currentTime:  0.0,
		eventQueue:   NewEventQueue(),
		stateData:    make(map[string]Value),
		transitions:  make(map[*ast.StateNode][]*ast.TransitionEdge),
		stateStack:   make([]*ast.StateNode, 0),
		parentState:  make(map[*ast.StateNode]*ast.StateNode),
	}
	
	// Extract graph structure from AST
	if err := exec.extractGraph(); err != nil {
		return nil, fmt.Errorf("extract graph: %w", err)
	}
	
	return exec, nil
}

// extractGraph builds state and transition maps from state machine AST.
func (e *StateExecutor) extractGraph() error {
	// Get state machine node
	stateUsage, ok := e.stateMachine.Decl.(*ast.Usage)
	if !ok {
		stateDef, ok := e.stateMachine.Decl.(*ast.Definition)
		if !ok {
			return fmt.Errorf("state machine symbol has invalid node type")
		}
		stateUsage = &ast.Usage{Members: stateDef.Members}
	}
	
	// Extract states and transitions (recursively for hierarchical states)
	for _, member := range stateUsage.Members {
		// Unwrap Membership if present
		actualMember := member
		if membership, ok := member.(*ast.Membership); ok {
			actualMember = membership.Member
		}
		
		switch n := actualMember.(type) {
		case *ast.StateNode:
			e.collectStates(n, nil) // Recursively collect substates
		case *ast.TransitionEdge:
			// Collect transitions (will be mapped after all states collected)
			if err := e.collectTransition(n); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// collectStates recursively collects states and builds parent relationships.
func (e *StateExecutor) collectStates(state *ast.StateNode, parent *ast.StateNode) {
	e.states = append(e.states, state)
	if parent != nil {
		e.parentState[state] = parent
	}
	
	// Recursively collect substates
	for _, substate := range state.Substates {
		if childState, ok := substate.(*ast.StateNode); ok {
			e.collectStates(childState, state)
		}
	}
	
	// Collect states in orthogonal regions
	for _, region := range state.Regions {
		for _, regionState := range region.States {
			if childState, ok := regionState.(*ast.StateNode); ok {
				e.collectStates(childState, state)
			}
		}
	}
}

// collectTransition maps a transition to its source state.
func (e *StateExecutor) collectTransition(trans *ast.TransitionEdge) error {
	sourceState := e.findStateByName(trans.Source)
	if sourceState == nil {
		return fmt.Errorf("transition references undefined source state")
	}
	e.transitions[sourceState] = append(e.transitions[sourceState], trans)
	return nil
}

// getParentChain returns all ancestor states from child to root (inclusive).
// Result is ordered: [child, parent, grandparent, ...]
func (e *StateExecutor) getParentChain(state *ast.StateNode) []*ast.StateNode {
	chain := []*ast.StateNode{state}
	current := state
	for {
		parent, hasParent := e.parentState[current]
		if !hasParent {
			break
		}
		chain = append(chain, parent)
		current = parent
	}
	return chain
}

// getLCA finds the lowest common ancestor of two states.
// Returns nil if states are in different hierarchies.
func (e *StateExecutor) getLCA(state1, state2 *ast.StateNode) *ast.StateNode {
	chain1 := e.getParentChain(state1)
	chain2 := e.getParentChain(state2)
	
	// Build set from chain1
	chain1Set := make(map[*ast.StateNode]bool)
	for _, s := range chain1 {
		chain1Set[s] = true
	}
	
	// Find first common ancestor in chain2
	for _, s := range chain2 {
		if chain1Set[s] {
			return s
		}
	}
	
	return nil // No common ancestor
}

// findStateByName looks up a state by qualified name.
func (e *StateExecutor) findStateByName(qname *ast.QualifiedName) *ast.StateNode {
	if qname == nil || len(qname.Parts) == 0 {
		return nil
	}
	
	targetName := qname.Parts[len(qname.Parts)-1].Text
	for _, state := range e.states {
		if state.Name == targetName {
			return state
		}
	}
	
	return nil
}

// scheduleTransitionEvents schedules TimeEvents for outgoing transitions from current state.
func (e *StateExecutor) scheduleTransitionEvents() error {
	transitions := e.transitions[e.currentState]
	
	for _, trans := range transitions {
		if timeEvent, ok := trans.Trigger.(*ast.TimeEvent); ok {
			// Evaluate duration expression
			ec := NewEvalContext(e.ctx, nil)
			ec.Push(e.stateData)
			durationVal, err := ec.Eval(timeEvent.Duration)
			if err != nil {
				return fmt.Errorf("eval time duration: %w", err)
			}
			
			// Extract numeric duration
			var duration float64
			if durationVal.Kind == ValConst {
				switch durationVal.Const.Kind {
				case semantics.ValInt:
					duration = float64(durationVal.Const.Int)
				case semantics.ValReal:
					duration = durationVal.Const.Real
				default:
					return fmt.Errorf("time duration must be numeric, got %v", durationVal.Const.Kind)
				}
			} else {
				return fmt.Errorf("time duration must be constant, got %v", durationVal.Kind)
			}
			
			// Schedule event (generate unique ID using current queue length)
			e.eventQueue.Push(Event{
				ID:        int64(e.eventQueue.Len() + 1),
				Type:      EventTime,
				Timestamp: e.currentTime + duration,
				Payload:   trans, // Store transition reference
			})
		}
	}
	
	return nil
}

// processNextEvent pops and processes the next event from queue.
func (e *StateExecutor) processNextEvent() error {
	if e.eventQueue.Len() == 0 {
		return fmt.Errorf("no events to process")
	}
	
	event := e.eventQueue.Pop()
	
	// Advance time
	e.currentTime = event.Timestamp
	
	// Process event by type
	switch event.Type {
	case EventTime:
		// Fire transition
		transition, ok := event.Payload.(*ast.TransitionEdge)
		if !ok {
			return fmt.Errorf("invalid TimeEvent payload")
		}
		return e.fireTransition(transition)
	default:
		return fmt.Errorf("unsupported event type: %v", event.Type)
	}
}

// fireTransition executes a state transition.
func (e *StateExecutor) fireTransition(trans *ast.TransitionEdge) error {
	// Find target state
	targetState := e.findStateByName(trans.Target)
	if targetState == nil {
		return fmt.Errorf("transition target state not found")
	}
	
	// Evaluate guard if present
	if trans.Guard != nil {
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData)
		guardVal, err := ec.Eval(trans.Guard)
		if err != nil {
			return fmt.Errorf("eval guard: %w", err)
		}
		
		// Check if boolean true
		guardPass := false
		if guardVal.Kind == ValConst && guardVal.Const.Kind == semantics.ValBool {
			guardPass = guardVal.Const.Bool
		} else {
			return fmt.Errorf("guard must be boolean, got %v", guardVal.Kind)
		}
		
		// Block transition if guard false
		if !guardPass {
			return nil // Remain in current state
		}
	}
	
	// Exit current state hierarchy up to LCA
	lca := e.getLCA(e.currentState, targetState)
	statesToExit := make([]*ast.StateNode, 0)
	current := e.currentState
	for current != nil && current != lca {
		statesToExit = append(statesToExit, current)
		current = e.parentState[current]
	}
	
	// Exit states (deepest to shallowest)
	for _, state := range statesToExit {
		if err := e.exitState(state); err != nil {
			return fmt.Errorf("exit state: %w", err)
		}
	}
	
	// Execute transition effect
	for _, action := range trans.Effect {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}
	
	// Enter target state hierarchy from LCA
	statesToEnter := make([]*ast.StateNode, 0)
	current = targetState
	for current != nil && current != lca {
		statesToEnter = append(statesToEnter, current)
		current = e.parentState[current]
	}
	
	// Reverse statesToEnter (shallowest to deepest)
	for i := len(statesToEnter) - 1; i >= 0; i-- {
		if err := e.enterState(statesToEnter[i]); err != nil {
			return fmt.Errorf("enter state: %w", err)
		}
	}
	
	// Update current state and rebuild stateStack with full active configuration
	e.currentState = targetState
	e.stateStack = e.getParentChain(targetState)
	// Reverse to root→leaf order for stateStack
	for i, j := 0, len(e.stateStack)-1; i < j; i, j = i+1, j-1 {
		e.stateStack[i], e.stateStack[j] = e.stateStack[j], e.stateStack[i]
	}
	
	// Schedule new events
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}
	
	// Check if final state
	if targetState.IsFinal {
		e.state = StateCompleted
	}
	
	return nil
}
// initialize sets current state to initial state and enters it.
func (e *StateExecutor) initialize() error {
	// Find initial state (deepest in hierarchy)
	initialState := e.findDeepestInitialState()
	
	if initialState == nil {
		return fmt.Errorf("no initial state found in state machine %s", e.stateMachine.Name)
	}
	
	// Enter initial state hierarchy (parent to child)
	e.currentState = initialState
	e.state = StateRunning
	
	// Build stateStack with full active configuration (root → leaf)
	chain := e.getParentChain(initialState)
	// Reverse to root→leaf order
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	e.stateStack = chain
	
	// Enter states from root to initial state
	for _, state := range e.stateStack {
		if err := e.enterState(state); err != nil {
			return fmt.Errorf("enter state %s: %w", state.Name, err)
		}
	}
	
	// Schedule events for outgoing transitions
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}
	
	return nil
}

// findDeepestInitialState finds initial state, following nested initial states.
func (e *StateExecutor) findDeepestInitialState() *ast.StateNode {
	// Find top-level initial state (no parent or parent not initial)
	var current *ast.StateNode
	for _, state := range e.states {
		if !state.IsInitial {
			continue
		}
		// Check if parent exists and is initial - skip if so (we want root initial)
		parent := e.parentState[state]
		if parent == nil || !parent.IsInitial {
			current = state
			break
		}
	}
	
	if current == nil {
		return nil
	}
	
	// Follow nested initial states down to deepest level
	for {
		foundNested := false
		for _, state := range e.states {
			if state.IsInitial && e.parentState[state] == current {
				current = state
				foundNested = true
				break
			}
		}
		if !foundNested {
			break
		}
	}
	
	return current
}

// enterState executes entry behaviors when entering a state.
func (e *StateExecutor) enterState(state *ast.StateNode) error {
	if state == nil {
		return nil
	}
	
	// Execute entry actions
	for _, action := range state.Entry {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("entry action: %w", err)
		}
	}
	
	return nil
}

// exitState executes exit behaviors when leaving a state.
func (e *StateExecutor) exitState(state *ast.StateNode) error {
	if state == nil {
		return nil
	}
	
	// Execute exit actions
	for _, action := range state.Exit {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("exit action: %w", err)
		}
	}
	
	return nil
}

// executeAction executes a single action (used for entry/exit/effect actions).
func (e *StateExecutor) executeAction(action ast.Node) error {
	actionNode, ok := action.(*ast.ActionExecutionNode)
	if !ok {
		return fmt.Errorf("unsupported action type: %T", action)
	}
	
	if actionNode.Expression != nil {
		// Evaluate inline expression
		ec := NewEvalContext(e.ctx, nil)
		ec.Push(e.stateData) // Make state data available
		result, err := ec.Eval(actionNode.Expression)
		if err != nil {
			return fmt.Errorf("eval expression: %w", err)
		}
		// Store result in state data with action name
		e.stateData[actionNode.Name] = result
	} else if actionNode.ActionRef != nil {
		return fmt.Errorf("nested action invocation not yet implemented")
	}
	
	return nil
}

// pollChangeEvents checks ChangeEvent conditions for outgoing transitions.
// Fires transition immediately if condition becomes true.
func (e *StateExecutor) pollChangeEvents() error {
	transitions := e.transitions[e.currentState]
	
	for _, trans := range transitions {
		if changeEvent, ok := trans.Trigger.(*ast.ChangeEvent); ok {
			// Evaluate condition
			ec := NewEvalContext(e.ctx, nil)
			ec.Push(e.stateData)
			condVal, err := ec.Eval(changeEvent.Condition)
			if err != nil {
				return fmt.Errorf("eval change condition: %w", err)
			}
			
			// Check if boolean true
			isTrueVal := false
			if condVal.Kind == ValConst && condVal.Const.Kind == semantics.ValBool {
				isTrueVal = condVal.Const.Bool
			} else {
				return fmt.Errorf("change condition must be boolean, got %v", condVal.Kind)
			}
			
			// Fire transition if true
			if isTrueVal {
				return e.fireTransition(trans)
			}
		}
	}
	
	return nil
}

// --- Public accessor methods for REPL debugging ---

// CurrentState returns the current active state node.
func (e *StateExecutor) CurrentState() ast.Node {
	return e.currentState
}

// StateStack returns a copy of the state stack (active configuration).
func (e *StateExecutor) StateStack() []*ast.StateNode {
	stack := make([]*ast.StateNode, len(e.stateStack))
	copy(stack, e.stateStack)
	return stack
}

// StateData returns a copy of state machine local data.
func (e *StateExecutor) StateData() map[string]Value {
	data := make(map[string]Value, len(e.stateData))
	for k, v := range e.stateData {
		data[k] = v
	}
	return data
}

// EventQueue returns the event queue (not copied - read-only access).
func (e *StateExecutor) EventQueue() *EventQueue {
	return e.eventQueue
}

// CurrentTime returns the current simulation time.
func (e *StateExecutor) CurrentTime() float64 {
	return e.currentTime
}

// State returns current execution state.
func (e *StateExecutor) State() ExecutionState {
	return e.state
}

// StateMachineSymbol returns the state machine being executed.
func (e *StateExecutor) StateMachineSymbol() *symbols.Symbol {
	return e.stateMachine
}

// ProcessNextEvent processes the next event from the queue (for REPL stepping).
func (e *StateExecutor) ProcessNextEvent() error {
	return e.processNextEvent()
}
