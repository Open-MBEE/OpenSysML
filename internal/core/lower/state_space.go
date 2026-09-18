package lower

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// The library declarations state-space dynamics are lowered against, by qualified name.
const (
	ContinuousDynamicsFQN = "StateSpaceRepresentation::ContinuousStateSpaceDynamics"
	DiscreteDynamicsFQN   = "StateSpaceRepresentation::DiscreteStateSpaceDynamics"
	ZeroCrossingEventFQN  = "StateSpaceRepresentation::ZeroCrossingEventDef"
	IntegrateFQN          = "StateSpaceRepresentation::Integrate"
	EulerFQN              = "StateSpaceIntegration::Euler"
	RK4FQN                = "StateSpaceIntegration::RK4"
	FixedStepDynamicsFQN  = "StateSpaceIntegration::FixedStepDynamics"
)

// The protocol's member names, as StateSpaceRepresentation declares them.
const (
	StateSpaceFeature = "stateSpace"
	InputFeature      = "input"
	OutputFeature     = "output"
	TimeStepFeature   = "timeStep"
	StopTimeFeature   = "stopTime"
	TimeFeature       = "time"
	GuardFeature      = "guard"
	TerminalFeature   = "terminal"
	GetDerivativeCalc = "getDerivative"
	GetDifferenceCalc = "getDifference"
	GetOutputCalc     = "getOutput"
	GetNextStateCalc  = "getNextState"
	IntegrateCalc     = "integrate"
)

// ErrUnsupportedStateSpace reports a state-space action whose shape the runner
// cannot execute: a protocol member it does not provide, or one it cannot integrate.
var ErrUnsupportedStateSpace = errors.New("unsupported state-space dynamics")

// StateSpaceKind tells continuous dynamics, stepped by integrating a derivative,
// from discrete ones, stepped by adding a difference.
type StateSpaceKind int

const (
	// NotStateSpace is an action specializing neither dynamics.
	NotStateSpace StateSpaceKind = iota
	ContinuousDynamics
	DiscreteDynamics
)

// String names the kind as the library does.
func (k StateSpaceKind) String() string {
	switch k {
	case ContinuousDynamics:
		return "ContinuousStateSpaceDynamics"
	case DiscreteDynamics:
		return "DiscreteStateSpaceDynamics"
	}
	return "not state-space dynamics"
}

// Integrator is a fixed-step integration scheme the runtime provides for Integrate.
type Integrator int

const (
	// IntegratorUnstated leaves the scheme to the run's default.
	IntegratorUnstated Integrator = iota
	IntegratorEuler
	IntegratorRK4
)

// String names the integrator as the library does.
func (i Integrator) String() string {
	switch i {
	case IntegratorEuler:
		return "Euler"
	case IntegratorRK4:
		return "RK4"
	}
	return "default"
}

// StateSpaceModel answers the semantic questions lowering state-space dynamics
// asks, so an inherited or redefined protocol member is found where it was declared.
type StateSpaceModel interface {
	// LibrarySymbol is the library declaration of this qualified name, nil when unloaded.
	LibrarySymbol(fqn string) *symbols.Symbol
	// Specializes reports whether sym is general or transitively specializes it.
	Specializes(sym, general *symbols.Symbol) bool
	// MembersOf lists sym's effective members, inherited ones masked by its redefinitions.
	MembersOf(sym *symbols.Symbol) []*symbols.Symbol
	// FeatureTypes lists the types a feature has, declared or along its redefinitions.
	FeatureTypes(sym *symbols.Symbol) []*symbols.Symbol
	// ParameterDefault is the value a feature states along its redefinitions, with its scope.
	ParameterDefault(sym *symbols.Symbol) (ast.Node, *symbols.Scope)
	// LibraryDeclared reports a declaration of a library document, not the model's.
	LibraryDeclared(sym *symbols.Symbol) bool
}

// StateSpaceDynamics is the lowering of an action specializing the library's
// StateSpaceDynamics: the calcs the model provides for the protocol, how the
// next state is reached, the features a step reads and writes, and the zero
// crossings it watches.
type StateSpaceDynamics struct {
	Kind StateSpaceKind
	// Action is the action lowered; Scope its body scope, where its features resolve.
	Action *symbols.Symbol
	Scope  *symbols.Scope
	// State, Input and Output are the protocol's stateSpace, input and output
	// features as the action holds them, the nearest declaration of each.
	State, Input, Output *symbols.Symbol
	// TimeStep and StopTime are the features stating the step and the instant the
	// run stops at, nil where the action declares none; Time, when declared,
	// receives the clock's instant at each step.
	TimeStep, StopTime, Time *symbols.Symbol
	// Derivative is the model's getDerivative (continuous), Difference its
	// getDifference (discrete), OutputCalc its getOutput; each concrete.
	Derivative, Difference, OutputCalc *symbols.Symbol
	// NextState is a getNextState the model bodies itself, which the runner calls
	// in place of the library's; nil when the library's runs natively.
	NextState *symbols.Symbol
	// Integrator is the scheme the model binds to the library's integrate, unstated
	// when it leaves the run's default; IntegratorType is that scheme's declaration.
	Integrator     Integrator
	IntegratorType *symbols.Symbol
	// Crossings are the zero-crossing events the dynamics raise, in declaration order.
	Crossings []ZeroCrossing
}

// ZeroCrossing is one event occurrence among the dynamics' zeroCrossingEvents:
// its guard, whose sign change raises the event, and whether that ends the run.
type ZeroCrossing struct {
	Name string
	// Event is the event occurrence usage; EventType the ZeroCrossingEventDef
	// specialization typing it, which the raised event carries.
	Event     *symbols.Symbol
	EventType *symbols.Symbol
	Guard     ast.Node
	// GuardScope is where the guard was written, its names resolving there.
	GuardScope *symbols.Scope
	// Terminal is the expression stating whether the crossing ends the run, nil for never.
	Terminal      ast.Node
	TerminalScope *symbols.Scope
}

// StateSpaceKindOf classifies an action: the dynamics it specializes, if either.
func StateSpaceKindOf(action *symbols.Symbol, model StateSpaceModel) StateSpaceKind {
	if action == nil || model == nil {
		return NotStateSpace
	}
	if action.Kind != symbols.SymbolActionUsage && action.Kind != symbols.SymbolActionDef {
		return NotStateSpace
	}
	if general := model.LibrarySymbol(ContinuousDynamicsFQN); general != nil && model.Specializes(action, general) {
		return ContinuousDynamics
	}
	if general := model.LibrarySymbol(DiscreteDynamicsFQN); general != nil && model.Specializes(action, general) {
		return DiscreteDynamics
	}
	return NotStateSpace
}

// ToStateSpaceDynamics lowers an action specializing one of the library's
// dynamics into what the runner steps: a member the protocol needs that the
// model leaves abstract, or an integrator the runtime does not provide, is an
// ErrUnsupportedStateSpace.
func ToStateSpaceDynamics(action *symbols.Symbol, scope *symbols.Scope, model StateSpaceModel) (*StateSpaceDynamics, error) {
	kind := StateSpaceKindOf(action, model)
	if kind == NotStateSpace {
		return nil, fmt.Errorf("%w: action %s specializes neither %s nor %s",
			ErrUnsupportedStateSpace, action.Name, ContinuousDynamics, DiscreteDynamics)
	}
	dyn := &StateSpaceDynamics{Kind: kind, Action: action, Scope: scope}
	members := memberIndex(model.MembersOf(action))
	var err error
	switch kind {
	case ContinuousDynamics:
		dyn.Derivative, err = providedCalc(action, members, GetDerivativeCalc, model)
	case DiscreteDynamics:
		dyn.Difference, err = providedCalc(action, members, GetDifferenceCalc, model)
	}
	if err != nil {
		return nil, err
	}
	if dyn.OutputCalc, err = providedCalc(action, members, GetOutputCalc, model); err != nil {
		return nil, err
	}
	if err := dyn.lowerNextState(members, model); err != nil {
		return nil, err
	}
	dyn.State, dyn.Input, dyn.Output = members[StateSpaceFeature], members[InputFeature], members[OutputFeature]
	dyn.TimeStep, dyn.StopTime, dyn.Time = members[TimeStepFeature], members[StopTimeFeature], members[TimeFeature]
	if dyn.State == nil {
		return nil, fmt.Errorf("%w: action %s has no %s", ErrUnsupportedStateSpace, action.Name, StateSpaceFeature)
	}
	if kind == ContinuousDynamics {
		if err := dyn.lowerCrossings(members, model); err != nil {
			return nil, err
		}
	}
	return dyn, nil
}

// memberIndex keys members by name, the nearest declaration of each name first.
func memberIndex(members []*symbols.Symbol) map[string]*symbols.Symbol {
	index := make(map[string]*symbols.Symbol, len(members))
	for _, member := range members {
		if member == nil || member.Name == "" {
			continue
		}
		if _, seen := index[member.Name]; !seen {
			index[member.Name] = member
		}
	}
	return index
}

// providedCalc is the concrete calc the model provides under a protocol name; one
// left to the library's abstract declaration is the model's to provide.
func providedCalc(action *symbols.Symbol, members map[string]*symbols.Symbol, name string, model StateSpaceModel) (*symbols.Symbol, error) {
	calc, ok := members[name]
	if !ok || calc == nil {
		return nil, fmt.Errorf("%w: action %s has no %s", ErrUnsupportedStateSpace, action.Name, name)
	}
	if model.LibraryDeclared(calc) || isAbstract(calc.Decl) {
		return nil, fmt.Errorf("%w: action %s leaves %s abstract; a calc redefining it must provide its body",
			ErrUnsupportedStateSpace, action.Name, name)
	}
	if calc.Kind != symbols.SymbolCalcUsage && calc.Kind != symbols.SymbolCalcDef {
		return nil, fmt.Errorf("%w: %s of action %s is not a calc", ErrUnsupportedStateSpace, name, action.Name)
	}
	return calc, nil
}

// isAbstract reports a declaration written `abstract`.
func isAbstract(decl ast.Node) bool {
	switch n := decl.(type) {
	case *ast.Usage:
		return n.IsAbstract
	case *ast.Definition:
		return n.IsAbstract
	}
	return false
}

// lowerNextState settles how a step reaches the next state: the library's
// getNextState runs natively, with the integrator its integrate is bound to;
// a getNextState the model bodies itself is called as written.
func (dyn *StateSpaceDynamics) lowerNextState(members map[string]*symbols.Symbol, model StateSpaceModel) error {
	next, ok := members[GetNextStateCalc]
	if !ok || next == nil || model.LibraryDeclared(next) {
		return nil
	}
	if computesResult(next) {
		dyn.NextState = next
		return nil
	}
	integrate := memberIndex(model.MembersOf(next))[IntegrateCalc]
	if integrate == nil || model.LibraryDeclared(integrate) {
		return nil
	}
	return dyn.lowerIntegrator(integrate, model)
}

// computesResult reports a calc whose own body returns a value, as opposed to
// one redeclared only to bind or retype its members.
func computesResult(calc *symbols.Symbol) bool {
	scope := calc.Scope
	if scope == nil {
		scope = calc.OwnerScope
	}
	return Returns(CalcBody(calc.Decl, ast.DeclMembers(calc.Decl), scope))
}

// lowerIntegrator reads the scheme the model binds integrate to: one of those the
// runtime provides, or an ErrUnsupportedStateSpace naming what it bound instead.
func (dyn *StateSpaceDynamics) lowerIntegrator(integrate *symbols.Symbol, model StateSpaceModel) error {
	euler, rk4 := model.LibrarySymbol(EulerFQN), model.LibrarySymbol(RK4FQN)
	for _, typ := range model.FeatureTypes(integrate) {
		switch {
		case euler != nil && model.Specializes(typ, euler):
			dyn.Integrator, dyn.IntegratorType = IntegratorEuler, typ
			return nil
		case rk4 != nil && model.Specializes(typ, rk4):
			dyn.Integrator, dyn.IntegratorType = IntegratorRK4, typ
			return nil
		}
	}
	types := model.FeatureTypes(integrate)
	if general := model.LibrarySymbol(IntegrateFQN); len(types) == 0 || (len(types) == 1 && types[0] == general) {
		return nil
	}
	names := make([]string, 0, len(types))
	for _, typ := range types {
		names = append(names, typ.Name)
	}
	return fmt.Errorf("%w: action %s binds integrate to %s; the runtime integrates by %s or %s",
		ErrUnsupportedStateSpace, dyn.Action.Name, strings.Join(names, ", "), EulerFQN, RK4FQN)
}

// lowerCrossings collects the event occurrences typed by a ZeroCrossingEventDef,
// each with the guard its declaration binds.
func (dyn *StateSpaceDynamics) lowerCrossings(members map[string]*symbols.Symbol, model StateSpaceModel) error {
	crossingEvent := model.LibrarySymbol(ZeroCrossingEventFQN)
	if crossingEvent == nil {
		return nil
	}
	for _, member := range model.MembersOf(dyn.Action) {
		if member == nil || model.LibraryDeclared(member) || !isEventOccurrence(member.Decl) {
			continue
		}
		eventType := typeSpecializing(member, crossingEvent, model)
		if eventType == nil {
			continue
		}
		crossing := ZeroCrossing{Name: member.Name, Event: member, EventType: eventType}
		fields := memberIndex(model.MembersOf(member))
		if guard := fields[GuardFeature]; guard != nil {
			crossing.Guard, crossing.GuardScope = model.ParameterDefault(guard)
		}
		if crossing.Guard == nil {
			return fmt.Errorf("%w: zero crossing %s of action %s binds no guard",
				ErrUnsupportedStateSpace, member.Name, dyn.Action.Name)
		}
		if terminal := fields[TerminalFeature]; terminal != nil {
			crossing.Terminal, crossing.TerminalScope = model.ParameterDefault(terminal)
		}
		dyn.Crossings = append(dyn.Crossings, crossing)
	}
	return nil
}

// isEventOccurrence reports an `event occurrence` usage.
func isEventOccurrence(decl ast.Node) bool {
	usage, ok := decl.(*ast.Usage)
	return ok && usage.IsEvent
}

// typeSpecializing is the first of a feature's types that is general or
// specializes it, nil when none does.
func typeSpecializing(feature, general *symbols.Symbol, model StateSpaceModel) *symbols.Symbol {
	for _, typ := range model.FeatureTypes(feature) {
		if model.Specializes(typ, general) {
			return typ
		}
	}
	return nil
}
