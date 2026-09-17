package queryexec

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/query"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// State rows report where an object's state machines stand: one row per active
// leaf state, read from the session's executors as they are now.

// stateFQN declares the state properties a column expression may reference.
const stateFQN = "DocumentQueries::State"

// State properties, beside the metadata every row answers.
const (
	propertyObject    = "object"
	propertyMachine   = "machine"
	propertyStatePath = "statePath"
	propertyState     = "state"
	propertyRegion    = "region"
	propertyEnclosing = "enclosing"
)

// evaluateStates lists the active leaf states of each source row's object, one
// row per leaf, machines in the object's order and leaves in region order.
func (e *executor) evaluateStates(expression queryplan.Expression) (sequence, error) {
	if err := e.requireRuntime(expression); err != nil {
		return sequence{}, err
	}
	source, err := e.objectArgument(expression, "source")
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	for _, row := range source.values {
		inst, label, _ := row.Object()
		machines := stateMachinesOf(inst)
		if len(machines) == 0 {
			return sequence{}, &Error{
				Kind:      ErrorNoStateMachine,
				Query:     e.definition.Name(),
				Operation: expression.Operation(),
				Parameter: "source",
				Target:    label,
				Origin:    expression.Origin(),
			}
		}
		for _, machine := range machines {
			for _, leaf := range machine.State.ActiveStates() {
				if !e.consumeVisit() {
					return sequence{}, e.budgetError(expression)
				}
				result.values = append(result.values, StateValue(activeState(inst, label, machine, leaf)))
			}
		}
	}
	return result, nil
}

// evaluateInState lists the session's objects whose state machine is in the
// named state, a leaf or a state enclosing one, each object once in session order.
func (e *executor) evaluateInState(expression queryplan.Expression) (sequence, error) {
	if err := e.requireRuntime(expression); err != nil {
		return sequence{}, err
	}
	name, err := e.stringArgument(expression, "name")
	if err != nil {
		return sequence{}, err
	}
	if name == "" {
		return sequence{}, e.invalidArgument(expression, "name", name)
	}
	var result sequence
	declared := false
	err = e.eachSessionObject(expression, func(row Value) {
		inst, _, _ := row.Object()
		matched := false
		for _, machine := range stateMachinesOf(inst) {
			declared = declared || machineDeclaresState(machine.State, name)
			for _, leaf := range machine.State.ActiveStates() {
				if stateNamed(machine.State, leaf, name) {
					matched = true
				}
			}
		}
		if matched {
			result.values = append(result.values, row)
		}
	})
	if err != nil {
		return sequence{}, err
	}
	if !declared {
		return sequence{}, &Error{
			Kind:      ErrorUnknownState,
			Query:     e.definition.Name(),
			Operation: expression.Operation(),
			Parameter: "name",
			Actual:    name,
			Origin:    expression.Origin(),
		}
	}
	return result, nil
}

// objectArgument evaluates an argument whose rows must denote session objects:
// object rows as they are, an element by the session objects it declares.
func (e *executor) objectArgument(expression queryplan.Expression, name string) (sequence, error) {
	value, err := e.rowArgument(expression, name)
	if err != nil {
		return sequence{}, err
	}
	var result sequence
	for _, item := range value.values {
		if _, _, ok := item.Object(); ok {
			result.values = append(result.values, item)
			continue
		}
		if verdict, ok := item.Verdict(); ok {
			return sequence{}, e.verdictRowError(expression, name, verdict)
		}
		if state, ok := item.State(); ok {
			return sequence{}, e.rowKindError(expression, name, ErrorStateRow, state.Label())
		}
		if event, ok := item.Event(); ok {
			return sequence{}, e.rowKindError(expression, name, ErrorEventRow, event.Label())
		}
		sym, _ := item.Element()
		objects, err := e.objectsDeclaredBy(expression, sym)
		if err != nil {
			return sequence{}, err
		}
		if len(objects) == 0 {
			return sequence{}, &Error{
				Kind:      ErrorNotHeld,
				Query:     e.definition.Name(),
				Operation: expression.Operation(),
				Parameter: name,
				Target:    symbols.FQNOf(sym),
				Origin:    expression.Origin(),
			}
		}
		result.values = append(result.values, objects...)
	}
	return result, nil
}

// objectsDeclaredBy lists the session's objects an element declares: those it
// is the declaration of, or that are typed by it or a type conforming to it.
func (e *executor) objectsDeclaredBy(expression queryplan.Expression, sym *symbols.Symbol) ([]Value, error) {
	var out []Value
	err := e.eachSessionObject(expression, func(row Value) {
		inst, _, _ := row.Object()
		if symbols.SameElement(objectDeclaration(inst), sym) || e.objectConforms(inst, sym) {
			out = append(out, row)
		}
	})
	return out, err
}

// stateMachinesOf lists the state machines an object exhibits, in its order.
func stateMachinesOf(inst *runtime.Instance) []*runtime.ObjectBehavior {
	var out []*runtime.ObjectBehavior
	for _, b := range inst.Behaviors() {
		if b.Kind == lower.ExhibitedState && b.State != nil {
			out = append(out, b)
		}
	}
	return out
}

// activeState describes one active leaf of a machine as a row.
func activeState(inst *runtime.Instance, label string, machine *runtime.ObjectBehavior, leaf *ast.StateNode) State {
	exec := machine.State
	enclosing := exec.EnclosingStates(leaf)
	names := make([]string, 0, len(enclosing))
	for _, s := range enclosing {
		names = append(names, s.Name)
	}
	return State{
		object:    inst,
		label:     label,
		machine:   machineName(machine),
		symbol:    stateSymbol(exec.Graph(), leaf),
		name:      leaf.Name,
		path:      exec.StatePath(leaf),
		region:    regionName(exec.Graph(), leaf),
		enclosing: names,
		behavior:  machine.Symbol,
	}
}

// regionName names the innermost orthogonal region state stands in, its own or an
// ancestor's, "" for a state outside every region.
func regionName(graph *lower.StateGraph, state *ast.StateNode) string {
	for current := state; current != nil; current = graph.ParentState[current] {
		if region := graph.RegionOf[current]; region != nil {
			return region.Name
		}
		if region := graph.HiddenRegionOf[current]; region != nil {
			return region.Name
		}
	}
	return ""
}

// machineName names a machine as the object exhibits it, or as declared when
// it is exhibited anonymously.
func machineName(machine *runtime.ObjectBehavior) string {
	if machine.Name != "" {
		return machine.Name
	}
	if sym := machine.State.StateMachineSymbol(); sym != nil {
		return sym.Name
	}
	return ""
}

// stateSymbol is the symbol declaring a state of the graph, nil for one
// lowering synthesized without a declaration.
func stateSymbol(graph *lower.StateGraph, state *ast.StateNode) *symbols.Symbol {
	decl := graph.DeclOf(state)
	scope := graph.StateScopes[state]
	if scope == nil {
		return nil
	}
	if owner := scope.Owner(); owner != nil && owner.Decl == decl {
		return owner
	}
	for s := scope; s != nil; s = s.Parent() {
		if sym := s.MemberDeclaring(decl); sym != nil {
			return sym
		}
	}
	return nil
}

// stateNamed reports whether name denotes leaf or a state enclosing it, by
// the state's own name or by its path (`on.run`).
func stateNamed(exec *runtime.StateExecutor, leaf *ast.StateNode, name string) bool {
	chain := append(exec.EnclosingStates(leaf), leaf)
	parts := make([]string, 0, len(chain))
	for _, state := range chain {
		parts = append(parts, state.Name)
		if state.Name == name || strings.Join(parts, ".") == name {
			return true
		}
	}
	return false
}

// machineDeclaresState reports whether any state of a machine, active or not,
// answers to name.
func machineDeclaresState(exec *runtime.StateExecutor, name string) bool {
	graph := exec.Graph()
	for _, state := range graph.States {
		if graph.HiddenStates[state] {
			continue
		}
		if stateNamed(exec, state, name) {
			return true
		}
	}
	return false
}

// rowKindError reports a row of a kind an operation's parameter does not take.
func (e *executor) rowKindError(expression queryplan.Expression, name string, kind ErrorKind, target string) error {
	return &Error{
		Kind:      kind,
		Query:     e.definition.Name(),
		Operation: expression.Operation(),
		Parameter: name,
		Target:    target,
		Origin:    expression.Origin(),
	}
}

// statePropertyValues reads a property of a state row: the row's own
// properties first, then the metadata of the state as declared.
func (e *executor) statePropertyValues(row Value, property string) ([]Value, bool, error) {
	state, _ := row.State()
	origin := row.Origin()
	text := func(values ...string) []Value {
		out := make([]Value, 0, len(values))
		for _, value := range values {
			out = append(out, valueAt(StringValue(value), origin))
		}
		return out
	}
	switch property {
	case propertyObject:
		return []Value{valueAt(ObjectValue(state.object, state.label), origin)}, true, nil
	case propertyPath:
		return text(state.label), true, nil
	case propertyMachine:
		return text(state.machine), true, nil
	case query.PropertyName:
		return text(state.name), true, nil
	case propertyStatePath:
		return text(state.path), true, nil
	case propertyState:
		if state.symbol == nil {
			return nil, true, nil
		}
		return []Value{valueAt(ElementValue(state.symbol), origin)}, true, nil
	case propertyRegion:
		return text(state.region), true, nil
	case propertyEnclosing:
		return text(state.enclosing...), true, nil
	}
	if state.symbol == nil {
		return nil, false, nil
	}
	return e.propertyValues(ElementValue(state.symbol), property)
}
