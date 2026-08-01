package runtime

import (
	"fmt"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// Context carries runtime execution state. One per workspace session.
type Context struct {
	model     *semantics.Model
	resolver  *resolve.Resolver
	nextID    int64
	steps     int64
	maxSteps  int64
	instances map[int64]*Instance
	features  map[*symbols.Symbol][]EffectiveFeature
}

// NewContext creates a runtime context backed by the given semantic model.
// maxSteps sets the runaway guard (step counter limit).
func NewContext(model *semantics.Model, resolver *resolve.Resolver, maxSteps int64) *Context {
	if maxSteps <= 0 {
		panic(fmt.Sprintf("runtime: maxSteps must be > 0, got %d", maxSteps))
	}
	return &Context{
		model:     model,
		resolver:  resolver,
		nextID:    1, // IDs start at 1 (0 = invalid)
		steps:     0,
		maxSteps:  maxSteps,
		instances: make(map[int64]*Instance),
		features:  make(map[*symbols.Symbol][]EffectiveFeature),
	}
}

// Model returns the semantic model this context operates over.
func (ctx *Context) Model() *semantics.Model {
	return ctx.model
}

// allocateID returns the next instance ID and increments the counter.
func (ctx *Context) allocateID() int64 {
	id := ctx.nextID
	ctx.nextID++
	return id
}

// incrementStep increments the step counter and returns ErrStepLimitExceeded if limit reached.
func (ctx *Context) incrementStep() error {
	ctx.steps++
	if ctx.steps > ctx.maxSteps {
		return fmt.Errorf("%w (%d steps)", ErrStepLimitExceeded, ctx.maxSteps)
	}
	return nil
}

// getInstance retrieves an instance by ID.
func (ctx *Context) getInstance(id int64) (*Instance, bool) {
	inst, ok := ctx.instances[id]
	return inst, ok
}

// registerInstance stores an instance in the registry.
func (ctx *Context) registerInstance(inst *Instance) {
	if inst.ID <= 0 {
		panic(fmt.Sprintf("runtime: invalid instance ID %d (must be > 0)", inst.ID))
	}
	if _, exists := ctx.instances[inst.ID]; exists {
		panic(fmt.Sprintf("runtime: duplicate instance ID %d", inst.ID))
	}
	ctx.instances[inst.ID] = inst
}

// EvaluateConstraint evaluates a constraint definition/usage.
// Returns (satisfied, error). If IsAssert=true, violation is an error.
// If IsAssert=false (assume), always returns (true, nil) but logs assumptions.
func (ctx *Context) EvaluateConstraint(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	// Extract constraint members
	var members []ast.Node
	
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind != ast.DefConstraint {
			return false, fmt.Errorf("not a constraint definition: %s", sym.Name)
		}
		members = decl.Members
	case *ast.Usage:
		if decl.Kind != ast.UsageConstraint {
			return false, fmt.Errorf("not a constraint usage: %s", sym.Name)
		}
		members = decl.Members
	default:
		return false, fmt.Errorf("invalid constraint symbol: %s (%T)", sym.Name, sym.Decl)
	}
	
	// Evaluate each constraint member
	for _, member := range members {
		// Unwrap Membership
		node := member
		if membership, ok := member.(*ast.Membership); ok {
			node = membership.Member
		}
		
		// Check for ConstraintMember
		constraintMember, ok := node.(*ast.ConstraintMember)
		if !ok {
			continue // skip non-constraint members
		}
		
		// Evaluate constraint expression
		result, err := ctx.EvalWithScope(constraintMember.Expression, scope)
		if err != nil {
			return false, fmt.Errorf("constraint %s: evaluation failed: %w", sym.Name, err)
		}
		
		// Extract boolean value
		satisfied := false
		if result.Kind == ValConst && result.Const.Kind == semantics.ValBool {
			satisfied = result.Const.Bool
		} else {
			return false, fmt.Errorf("constraint %s: expression must evaluate to boolean, got %v", sym.Name, result.Kind)
		}
		
		// Apply negation
		if constraintMember.IsNegated {
			satisfied = !satisfied
		}
		
		// Handle assert vs assume
		if constraintMember.IsAssert {
			if !satisfied {
				return false, fmt.Errorf("constraint %s: assertion failed", sym.Name)
			}
		}
		// assume: always pass (assumptions are trusted)
	}
	
	return true, nil
}

// EvaluateRequirement evaluates a requirement definition/usage.
// Returns (satisfied, error). Validates subject/actor types and evaluates assume/require expressions.
// Assume members always pass (trusted), require members must evaluate to true.
func (ctx *Context) EvaluateRequirement(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	// Extract requirement members
	var members []ast.Node
	
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		if decl.Kind != ast.DefRequirement {
			return false, fmt.Errorf("not a requirement definition: %s", sym.Name)
		}
		members = decl.Members
	case *ast.Usage:
		if decl.Kind != ast.UsageRequirement {
			return false, fmt.Errorf("not a requirement usage: %s", sym.Name)
		}
		members = decl.Members
	default:
		return false, fmt.Errorf("invalid requirement symbol: %s (%T)", sym.Name, sym.Decl)
	}
	
	// Evaluate each requirement member
	for _, member := range members {
		// Unwrap Membership
		node := member
		if membership, ok := member.(*ast.Membership); ok {
			node = membership.Member
		}
		
		// Handle different requirement member types
		switch rm := node.(type) {
		case *ast.SubjectMember:
			// Subject: validate that subject binding exists in scope
			// For now, just check if name is resolvable
			if rm.Name != "" {
				_, ok := scope.LookupLocal(rm.Name)
				if !ok {
					return false, fmt.Errorf("requirement %s: subject '%s' not found in scope", sym.Name, rm.Name)
				}
			}
			
		case *ast.ActorMember:
			// Actor: validate that actor binding exists in scope
			// For now, just check if name is resolvable
			if rm.Name != "" {
				_, ok := scope.LookupLocal(rm.Name)
				if !ok {
					return false, fmt.Errorf("requirement %s: actor '%s' not found in scope", sym.Name, rm.Name)
				}
			}
			
		case *ast.AssumeMember:
			// Assume: always pass (trusted assumption)
			// Optionally could evaluate to check it's a valid expression
			continue
			
		case *ast.RequireMember:
			// Require: must evaluate to true
			result, err := ctx.EvalWithScope(rm.Expression, scope)
			if err != nil {
				return false, fmt.Errorf("requirement %s: require evaluation failed: %w", sym.Name, err)
			}
			
			// Extract boolean value
			satisfied := false
			if result.Kind == ValConst && result.Const.Kind == semantics.ValBool {
				satisfied = result.Const.Bool
			} else {
				return false, fmt.Errorf("requirement %s: require expression must evaluate to boolean, got %v", sym.Name, result.Kind)
			}
			
			if !satisfied {
				return false, fmt.Errorf("requirement %s: require condition failed", sym.Name)
			}
		}
	}
	
	return true, nil
}

// InvokeCalc invokes a calculation with given arguments and returns the result.
// Arguments should be in the same order as the calc's input parameters.
func (ctx *Context) InvokeCalc(sym *symbols.Symbol, args []Value, scope *symbols.Scope) (Value, error) {
	if sym == nil || sym.Decl == nil {
		return Value{}, fmt.Errorf("calc invocation: invalid symbol")
	}
	
	var members []ast.Node
	switch decl := sym.Decl.(type) {
	case *ast.Definition:
		members = decl.Members
	case *ast.Usage:
		members = decl.Members
	default:
		return Value{}, fmt.Errorf("calc invocation: %q is not a calc definition or usage", sym.Name)
	}
	
	if members == nil {
		return Value{}, fmt.Errorf("calc invocation: %q has no body", sym.Name)
	}
	
	// Extract parameters (usages with Direction = DirIn or DirInOut)
	var params []*ast.Usage
	for _, m := range members {
		// Unwrap Membership if present
		node := m
		if membership, ok := m.(*ast.Membership); ok {
			node = membership.Member
		}
		
		if usage, ok := node.(*ast.Usage); ok {
			if usage.Direction == ast.DirIn || usage.Direction == ast.DirInOut {
				params = append(params, usage)
			}
		}
	}
	
	// Validate argument count
	if len(args) != len(params) {
		return Value{}, fmt.Errorf("calc invocation: expected %d arguments, got %d", len(params), len(args))
	}
	
	// Create parameter bindings
	bindings := make(map[string]Value)
	for i, param := range params {
		// Use Name (not ShortName which might be empty for simple usages)
		name := param.Ident.Name
		if name == "" && param.Ident.ShortName != "" {
			name = param.Ident.ShortName
		}
		bindings[name] = args[i]
	}
	
	// Extract return member (ResultMember with Expression)
	var returnExpr ast.Node
	for _, m := range members {
		// Unwrap Membership if present
		node := m
		if membership, ok := m.(*ast.Membership); ok {
			node = membership.Member
		}
		
		if rm, ok := node.(*ast.ResultMember); ok && rm.Expression != nil {
			returnExpr = rm.Expression
			break
		}
	}
	
	if returnExpr == nil {
		return Value{}, fmt.Errorf("calc invocation: %q has no return expression", sym.Name)
	}
	
	// Evaluate return expression with parameter bindings
	ec := &EvalContext{
		ctx:    ctx,
		frames: []map[string]Value{bindings},
		scope:  scope,
	}
	
	return ec.Eval(returnExpr)
}

// ExecuteAction executes an action definition/usage to completion.
// Returns final token data from the action's execution.
func (ctx *Context) ExecuteAction(action *symbols.Symbol) (map[string]Value, error) {
	// Create executor
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		return nil, fmt.Errorf("create action executor: %w", err)
	}
	
	// Initialize execution (spawns initial token)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize action: %w", err)
	}
	
	// Run to completion
	if err := exec.RunToCompletion(); err != nil {
		return nil, fmt.Errorf("execute action: %w", err)
	}
	
	// Return accumulated results from final nodes
	return exec.results, nil
}

// ExecuteState executes a state machine, processing events until completion or suspension.
// Returns final state data from the state machine's execution.
// Execution stops when:
// - A final state is reached (StateCompleted)
// - Event queue is empty (StateSuspended)
// - Max event processing steps exceeded (error)
func (ctx *Context) ExecuteState(stateMachine *symbols.Symbol) (map[string]Value, error) {
	// Create executor
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		return nil, fmt.Errorf("create state executor: %w", err)
	}
	
	// Initialize execution (enters initial state)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize state machine: %w", err)
	}
	
	// Process events until completion or suspension
	const maxEvents = 10000
	eventCount := 0
	
	for exec.state == StateRunning {
		// Check for pending events
		if exec.eventQueue.Len() == 0 {
			exec.state = StateSuspended
			break
		}
		
		// Check step limit
		if eventCount >= maxEvents {
			return nil, fmt.Errorf("state machine exceeded max events (%d), possible infinite loop", maxEvents)
		}
		
		// Process next event
		if err := exec.processNextEvent(); err != nil {
			return nil, fmt.Errorf("process event: %w", err)
		}
		
		eventCount++
	}
	
	// Return state machine data
	return exec.stateData, nil
}

// CreateActionExecutor creates an action executor without starting execution.
// For REPL debugging - allows step-by-step execution control.
func (ctx *Context) CreateActionExecutor(action *symbols.Symbol) (*ActionExecutor, error) {
	exec, err := newActionExecutor(ctx, action)
	if err != nil {
		return nil, fmt.Errorf("create action executor: %w", err)
	}
	
	// Initialize (spawns initial token)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize action: %w", err)
	}
	
	return exec, nil
}

// CreateStateExecutor creates a state executor without starting execution.
// For REPL debugging - allows step-by-step execution control.
func (ctx *Context) CreateStateExecutor(stateMachine *symbols.Symbol) (*StateExecutor, error) {
	exec, err := newStateExecutor(ctx, stateMachine)
	if err != nil {
		return nil, fmt.Errorf("create state executor: %w", err)
	}
	
	// Initialize (enters initial state, schedules initial events)
	if err := exec.initialize(); err != nil {
		return nil, fmt.Errorf("initialize state machine: %w", err)
	}
	
	return exec, nil
}
