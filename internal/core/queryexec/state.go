package queryexec

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// State is one row of States or InState: an active leaf state of one object's
// state machine, as the session stands. Immutable once built.
type State struct {
	object    *runtime.Instance
	label     string
	machine   string
	symbol    *symbols.Symbol
	name      string
	path      string
	region    string
	enclosing []string
	behavior  *symbols.Symbol
}

// Object is the object whose machine is in the state, under the label the
// session reaches it by.
func (s State) Object() (*runtime.Instance, string) { return s.object, s.label }

// Machine names the state machine the object exhibits that is in the state.
func (s State) Machine() string { return s.machine }

// Declaration is the state as declared, nil when lowering synthesized it.
func (s State) Declaration() *symbols.Symbol { return s.symbol }

// Name is the active leaf state's own name.
func (s State) Name() string { return s.name }

// Path is the leaf's name qualified by the states enclosing it (`on.run`).
func (s State) Path() string { return s.path }

// Region names the orthogonal region the leaf runs in, "" outside any region.
func (s State) Region() string { return s.region }

// Enclosing names the composite states the leaf is nested in, outermost first;
// each is active with the leaf.
func (s State) Enclosing() []string { return append([]string(nil), s.enclosing...) }

// Label names the row: the state's path on its object's machine (`lamp.lp in on.dim`).
func (s State) Label() string {
	return s.label + "." + s.machine + " in " + s.path
}

// StateValue constructs a state value; its declaration is the state.
func StateValue(state State) Value {
	return Value{kind: ValueState, state: &state, origin: state.symbol.Origin()}
}

// State returns the value's state and whether it is a state value.
func (v Value) State() (State, bool) {
	if v.kind != ValueState || v.state == nil {
		return State{}, false
	}
	return *v.state, true
}
