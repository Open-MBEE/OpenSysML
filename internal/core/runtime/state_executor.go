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
	stateStack []*ast.StateNode // Active state configuration (for nested states)
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
	
	// Extract states and transitions
	for _, member := range stateUsage.Members {
		// Unwrap Membership if present
		actualMember := member
		if membership, ok := member.(*ast.Membership); ok {
			actualMember = membership.Member
		}
		
		switch n := actualMember.(type) {
		case *ast.StateNode:
			e.states = append(e.states, n)
		case *ast.TransitionEdge:
			// Map transition to source state
			sourceState := e.findStateByName(n.Source)
			if sourceState == nil {
				return fmt.Errorf("transition references undefined source state")
			}
			e.transitions[sourceState] = append(e.transitions[sourceState], n)
		}
	}
	
	return nil
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
	
	// Exit current state
	if err := e.exitState(e.currentState); err != nil {
		return fmt.Errorf("exit state: %w", err)
	}
	
	// Execute transition effect
	for _, action := range trans.Effect {
		if err := e.executeAction(action); err != nil {
			return fmt.Errorf("transition effect: %w", err)
		}
	}
	
	// Update current state
	e.currentState = targetState
	e.stateStack[len(e.stateStack)-1] = targetState
	
	// Enter target state
	if err := e.enterState(targetState); err != nil {
		return fmt.Errorf("enter state: %w", err)
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
	// Find initial state
	var initialState *ast.StateNode
	for _, state := range e.states {
		if state.IsInitial {
			initialState = state
			break
		}
	}
	
	if initialState == nil {
		return fmt.Errorf("no initial state found in state machine %s", e.stateMachine.Name)
	}
	
	// Enter initial state
	e.currentState = initialState
	e.stateStack = append(e.stateStack, initialState)
	e.state = StateRunning
	
	// Execute entry behavior
	if err := e.enterState(initialState); err != nil {
		return fmt.Errorf("enter initial state: %w", err)
	}
	
	// Schedule events for outgoing transitions
	if err := e.scheduleTransitionEvents(); err != nil {
		return fmt.Errorf("schedule events: %w", err)
	}
	
	return nil
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
