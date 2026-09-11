package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

var (
	// ErrStepLimitExceeded is returned when the evaluation step counter exceeds maxSteps.
	ErrStepLimitExceeded = errors.New("evaluation step limit exceeded")

	// ErrElementLimitExceeded is returned when the collection elements one run
	// materializes exceed maxElements. It is a bound on memory rather than on
	// work, so it is its own error and its own budget.
	ErrElementLimitExceeded = errors.New("collection element limit exceeded")

	// ErrUnresolvedReference is returned when a feature reference cannot be resolved.
	ErrUnresolvedReference = errors.New("unresolved reference")

	// ErrAmbiguousReference is returned when a qualified name names several elements.
	ErrAmbiguousReference = errors.New("ambiguous reference")

	// ErrTypeMismatch is returned when an operation receives a value of unexpected type.
	ErrTypeMismatch = errors.New("type mismatch")

	// ErrDivisionByZero is returned when a division or remainder has a zero
	// divisor. It is the answer to the expression, not a missing declaration.
	ErrDivisionByZero = semantics.ErrDivisionByZero

	// ErrMultiplicityViolation is returned when a feature value access/assignment violates multiplicity bounds.
	ErrMultiplicityViolation = errors.New("multiplicity violation")

	// ErrUniquenessViolation is returned when a value written to a unique feature repeats one of its values.
	ErrUniquenessViolation = errors.New("uniqueness violation")

	// ErrUninitializedFeatureValue is returned when accessing a feature value that has no value and no default.
	ErrUninitializedFeatureValue = errors.New("uninitialized feature value")

	// ErrBindingConflict is returned when two binding ends hold unequal values.
	ErrBindingConflict = errors.New("binding conflict")

	// ErrBindingCycle is returned when a binding component has no value.
	ErrBindingCycle = errors.New("binding cycle")

	// ErrBindingEnd is returned when a binding endpoint cannot be resolved to a feature.
	ErrBindingEnd = errors.New("binding end cannot be resolved")

	// ErrNotACalc is returned when a calc invocation targets a symbol that is
	// not a calc definition or usage.
	ErrNotACalc = errors.New("not a calc")

	// ErrNotAFunction is returned when a value that is no function is called,
	// or is bound where a calc-typed feature needs one.
	ErrNotAFunction = errors.New("not a function")

	// ErrNotAConstraint is returned when a symbol asked to be evaluated as a
	// constraint declares something else. It is a usage error about the request,
	// not a verdict about the model, so callers can tell the two apart.
	ErrNotAConstraint = errors.New("not a constraint")

	// ErrNotARequirement is returned when a symbol asked to be evaluated as a
	// requirement declares something else. Like ErrNotAConstraint it reports the
	// request, not the model.
	ErrNotARequirement = errors.New("not a requirement")

	// ErrNotAnAnalysis is returned when a symbol asked for its objectives or
	// asked to be run is not a case whose body runs. Like ErrNotAConstraint it
	// reports the request, not the model.
	ErrNotAnAnalysis = errors.New("not an analysis case")

	// ErrNotAVerification is returned when a symbol asked for a verification
	// verdict is not a verification case definition or usage.
	ErrNotAVerification = errors.New("not a verification case")

	// ErrCalcArity is returned when a calc invocation passes more arguments than
	// the calc declares input parameters.
	ErrCalcArity = errors.New("calc argument count mismatch")

	// ErrUnboundParameter is returned when a calc input parameter receives
	// neither an argument nor a declared default.
	ErrUnboundParameter = errors.New("unbound parameter")

	// ErrUnknownParameter is returned when a named argument does not name any
	// input parameter of the invoked calc.
	ErrUnknownParameter = errors.New("unknown parameter")

	// ErrUnknownActionInput is returned when a supplied input names no parameter
	// or attribute of the action performed.
	ErrUnknownActionInput = errors.New("unknown action input")

	// ErrOutputActionInput is returned when a supplied input names a parameter
	// the action only writes back (`out`), which a caller does not seed.
	ErrOutputActionInput = errors.New("output action parameter given as input")

	// ErrNoResultExpression is returned when a calc body declares no return
	// expression, directly or by inheritance.
	ErrNoResultExpression = errors.New("no result expression")

	// ErrConflictingResultExpressions is returned when a calc or constraint owns or
	// inherits more than one result expression (KerML 8.3.4.8); no body is chosen.
	ErrConflictingResultExpressions = errors.New("more than one result expression, owned or inherited")

	// ErrUnsupportedOperator is returned when an operator has no runtime
	// evaluation, so an expression naming it fails rather than yielding nothing.
	ErrUnsupportedOperator = errors.New("unsupported operator")

	// ErrUnresolvedType is returned when a type classification operand names no
	// resolvable type.
	ErrUnresolvedType = errors.New("unresolved type")

	// ErrUndeterminedValueType is returned when a value classification has no
	// direct runtime type to compare.
	ErrUndeterminedValueType = errors.New("value type cannot be determined")

	// ErrUndecidedClassification is returned when a cast reaches a value whose
	// classification by a type narrower than the value's own the value does not
	// settle, so the cast fails rather than dropping a value that may be one.
	ErrUndecidedClassification = errors.New("classification of a value cannot be decided")

	// ErrCalcNoReturn is returned when a calc body runs to its end without
	// returning: it computed no result, which is not the same as a null one.
	ErrCalcNoReturn = errors.New("calculation returned no value")

	// ErrCalcSideEffect is returned when a calc body states an effect on the
	// world outside it — send, perform, accept, terminate. A calculation
	// computes a value, so an effect is rejected rather than performed.
	ErrCalcSideEffect = errors.New("side effect in a calculation body")

	// ErrCalcExternalAssignment is returned when a calc body assigns to a name it
	// does not declare itself, which would make the calculation impure.
	ErrCalcExternalAssignment = errors.New("assignment outside the calculation body")

	// ErrStatementNotExecutable is returned when a body reaches a member the
	// lowering marked as not executable in that kind of body (lower.Unsupported).
	ErrStatementNotExecutable = errors.New("statement not executable")

	// ErrCalcUsageRecursion is returned when a calc or analysis usage's body reads
	// the usage itself: bound once, it would run itself without end.
	ErrCalcUsageRecursion = errors.New("a calc usage runs itself")

	// ErrReturnOutsideCalc is returned when a `return` is executed by a host that
	// has no result to return, an action node's body.
	ErrReturnOutsideCalc = errors.New("'return' outside a calculation body")

	// ErrAcceptDeadlock is returned when an action can no longer progress
	// because every token it has left is parked at an accept, so no token can
	// post the message any of them waits for. An accept suspends the action
	// rather than failing, so this is how a suspension that can never end is
	// reported instead of hanging.
	ErrAcceptDeadlock = errors.New("accept deadlock")

	// ErrActionDeadlock is returned when action tokens cannot make progress.
	ErrActionDeadlock = errors.New("action deadlock")

	// ErrExecutorReleased is returned when a released executor is stepped.
	ErrExecutorReleased = errors.New("executor released")

	// ErrInvalidActionFlow is returned for a structurally invalid action graph.
	ErrInvalidActionFlow = errors.New("invalid action flow")

	// ErrNoEnabledSuccession is returned when a decision can select no branch.
	ErrNoEnabledSuccession = errors.New("no enabled succession")

	// ErrNegativeDuration is returned when a delay — an `accept after`, a time
	// transition's or an advance of the clock — is negative, infinite, no number
	// at all, or leads past the last instant the clock can hold: the clock never
	// runs backwards and always reads a finite instant.
	ErrNegativeDuration = errors.New("negative duration")

	// ErrNothingDue is returned when one step is asked of an executor whose only
	// remaining work waits on the clock for an instant it has not reached.
	ErrNothingDue = errors.New("nothing due at the current instant")

	// ErrTimeTriggerType is returned when a time trigger's argument is declared as
	// no value of the type the trigger takes — the judgement validation makes of it.
	ErrTimeTriggerType = errors.New("time trigger argument of the wrong type")

	// ErrActionResultParameter is returned when an action to perform declares a
	// `return` parameter, which only a function or expression owns.
	ErrActionResultParameter = errors.New("action declares a return parameter")

	// ErrCalcRecursionLimit is returned when calc invocation nests deeper than
	// the run's calc depth budget, which an unbounded recursion would otherwise
	// do until the process ran out of stack.
	ErrCalcRecursionLimit = errors.New("calc recursion limit exceeded")

	// ErrActionStepLimitExceeded is returned when an action executor exceeds
	// its token-flow step budget.
	ErrActionStepLimitExceeded = errors.New("action step limit exceeded")

	// ErrStateEventLimitExceeded is returned when state processing exceeds its
	// event budget.
	ErrStateEventLimitExceeded = errors.New("state event limit exceeded")

	// ErrNoInitialState is returned when a state machine initializes with no
	// entry into its states: it states no `entry; then <state>;`.
	ErrNoInitialState = errors.New("no initial state found")

	// ErrNoEntryTransitionHolds is returned when a body's guarded entry transitions
	// (`entry; if c then s;`) all have false guards, so it has no state to start in.
	ErrNoEntryTransitionHolds = errors.New("no entry transition holds")

	// ErrStatePerformanceOccurrence is returned when an exhibited machine cannot
	// read or write the occurrence of its state usage.
	ErrStatePerformanceOccurrence = errors.New("state performance occurrence unavailable")

	// ErrActionPerformanceOccurrence is returned when a performed action cannot
	// read or write the occurrence of its action usage.
	ErrActionPerformanceOccurrence = errors.New("action performance occurrence unavailable")

	// ErrDoStepLimitExceeded is returned when a state do behavior exceeds its
	// action-step budget.
	ErrDoStepLimitExceeded = errors.New("state do-step limit exceeded")

	// ErrStateBehaviorWaits is returned when an entry or exit behavior, or a
	// transition effect, waits for the clock: those are performed whole at the
	// instant they are triggered, and only a do behavior pauses on the clock.
	ErrStateBehaviorWaits = errors.New("state behavior waits for the clock")

	// ErrActionArity is returned when an action invocation passes more
	// positional arguments than the action declares input parameters.
	ErrActionArity = errors.New("action argument count mismatch")

	// ErrDuplicateArgument is returned when an action invocation binds one input
	// parameter twice: by two named arguments, or by a positional and a named one.
	ErrDuplicateArgument = errors.New("argument bound more than once")

	// ErrNodeNotPerformed is returned when a pin of an action node is read before
	// any performance of the node has started.
	ErrNodeNotPerformed = errors.New("action node read before it is performed")

	// ErrNodePin is returned when a pin read, flow, or binding names a feature the
	// action node does not declare, or the node's result where it has none.
	ErrNodePin = errors.New("action node pin not declared")

	// ErrViolated is returned when an asserted constraint or a required
	// condition evaluates to false. It is a verdict about the model, not a
	// failure to evaluate, so callers can tell the two apart.
	ErrViolated = errors.New("evaluated to false")

	// ErrNoValue is returned when a feature a condition names carries no value:
	// neither a feature value on the object being checked nor a declared default.
	ErrNoValue = errors.New("no value")

	// ErrNoConditions is returned when a constraint or requirement carries no
	// condition to evaluate: reporting a verdict would claim a check that never ran.
	ErrNoConditions = errors.New("no condition to evaluate")

	// ErrStatementNotExecuted is returned when a constraint body states an action
	// statement: the evaluator does not run it, so a verdict would ignore it.
	ErrStatementNotExecuted = errors.New("statement in a constraint body is not executed by OpenSysML")

	// ErrUnboundSubject is returned when a condition reads a subject nothing
	// supplied: the check is about no object, so it reaches no verdict.
	ErrUnboundSubject = errors.New("subject is unbound")

	// ErrCyclicFeatureValue is returned when a feature value's default value depends, directly or
	// through other feature values, on the one being computed.
	ErrCyclicFeatureValue = errors.New("cyclic feature value dependency")

	// ErrConnectorEnd is returned when a connector cannot be attached to the
	// features its ends name: an end naming nothing reachable from the object
	// owning the connector, or one carrying no value. A connector whose ends
	// cannot be attached relates nothing, so it is an error rather than an object
	// with defaults at its ends.
	ErrConnectorEnd = errors.New("connector end cannot be attached")

	// ErrNotAQuantity is returned when `x [y]` is not a quantity expression:
	// y names no measurement unit, or x is no magnitude.
	ErrNotAQuantity = errors.New("not a quantity expression")

	// ErrIncommensurableUnits is returned when an operation combines quantities
	// whose units measure different things; see semantics.ErrIncommensurableUnits.
	ErrIncommensurableUnits = semantics.ErrIncommensurableUnits

	// ErrUnitRoot is returned when the root of a quantity is taken whose unit
	// has none: `sqrt(9 [m])`, since no unit squares to a metre.
	ErrUnitRoot = errors.New("unit has no root")

	// ErrScalePoint is returned when an operation is asked of a point on a
	// measurement scale that has no meaning for a point (its multiple, the sum of two).
	ErrScalePoint = errors.New("operation is not defined on a point of a measurement scale")

	// ErrNotASatisfaction is returned when a satisfaction assertion is asked of
	// an element that states none.
	ErrNotASatisfaction = errors.New("not a satisfaction assertion")

	// ErrNoRequirement is returned when a satisfaction assertion states no
	// requirement to evaluate: it references none, or references one that
	// resolves to nothing.
	ErrNoRequirement = errors.New("no requirement to satisfy")

	// ErrUnresolvedClassifierBehavior is returned when a type exhibits or
	// performs a behavior whose body no element states, so the objects of that
	// type have nothing to run.
	ErrUnresolvedClassifierBehavior = errors.New("classifier behavior names no body")

	// ErrUnsupportedClassifierBehavior is returned when a type binds a behavior
	// the runtime does not execute on an object.
	ErrUnsupportedClassifierBehavior = errors.New("unsupported classifier behavior")

	// ErrNoSuchBehavior is returned when a behavior asked of an object is none
	// the object's type owns, exhibits or performs.
	ErrNoSuchBehavior = errors.New("object has no such behavior")

	// ErrNotABehavior is returned when a name invoked on an object resolves to an
	// element that states no behavior to run.
	ErrNotABehavior = errors.New("not a behavior")

	// ErrNotASignal is returned when a message injected from outside the model
	// names an element that is no definition, so no accept could be typed by it.
	ErrNotASignal = errors.New("not a signal definition")

	// ErrSignalArgument is returned when a message injected from outside the
	// model carries an argument its signal definition has no feature for.
	ErrSignalArgument = errors.New("signal argument")

	// ErrBehaviorBudget is returned when the behaviors of materialized objects
	// never reach quiescence within the event budget.
	ErrBehaviorBudget = errors.New("object behaviors exceeded their budget")

	// ErrNotACalcUsage is returned when an output feature is read from a symbol
	// that is not a calc usage: only a usage carries an evaluation whose outputs
	// are features.
	ErrNotACalcUsage = errors.New("not a calc usage")

	// ErrUnknownOutput is returned when a name read from a calc usage is not one
	// of the output features its calc declares.
	ErrUnknownOutput = errors.New("unknown output")

	// ErrOutputNotAssigned is returned when a declared output carries no value
	// because the activation never assigned it. It is a kind of ErrNoValue.
	ErrOutputNotAssigned = fmt.Errorf("%w: output never assigned", ErrNoValue)

	// ErrConflictingOutput is returned when one activation would bind an output
	// twice: by its declaration and by an assignment, or by two assignments.
	ErrConflictingOutput = errors.New("output bound more than once")

	// ErrCyclicOutput is returned when an output feature's binding depends,
	// directly or through other outputs, on the output being computed.
	ErrCyclicOutput = errors.New("cyclic output dependency")

	// ErrAmbiguousResult is returned when a calc declaring several output
	// features is invoked as an expression. A function invocation has exactly
	// one result (KerML 7.4.9), so a calc that designates none has no value to
	// hand back and is read through a calc usage's output features instead.
	ErrAmbiguousResult = errors.New("calculation has no single result")

	// ErrIndexOutOfRange is returned when an index names no position of the
	// sequence or string it indexes; indices are 1-based, so 0 is out of range
	// as much as size+1 is, and each operation names what it indexed.
	ErrIndexOutOfRange = errors.New("index out of range")

	// ErrBodyArity is returned when the body expression a collection operation
	// is given declares a number of parameters the operation cannot call it
	// with: `select` calls its selector with one element, so a selector
	// declaring two parameters has no second argument to receive.
	ErrBodyArity = errors.New("body parameter count mismatch")

	// ErrUnsupportedBodyDeclaration is returned when a body expression declares
	// features of its own: the evaluator binds its parameters, not its
	// declarations, so applying it would read them as unresolved.
	ErrUnsupportedBodyDeclaration = errors.New("unsupported declaration in a body expression")

	// ErrReceiverWithNamedArgs is returned when a receiver is written before a
	// call whose arguments are named, `x->f(a = 1)`. The receiver binds by
	// position and the arguments by name, so which parameter the receiver binds
	// to is unstated; it is reported rather than dropped.
	ErrReceiverWithNamedArgs = errors.New("receiver combined with named arguments")

	// ErrVariationUnselected is returned when a variation is read without having
	// been bound to one of its variants: it classifies its variants abstractly,
	// so it stands for no one value until a variant is selected.
	ErrVariationUnselected = errors.New("variation has no variant selected")

	// ErrNotAVariant is returned when a variation is bound to something that is
	// not one of the variants it offers.
	ErrNotAVariant = errors.New("not a variant of the variation")

	// ErrMultipleVariants is returned when a variation is bound to more than one
	// variant, which selects no single configuration.
	ErrMultipleVariants = errors.New("more than one variant selected")

	// ErrNotALiteral is returned when a name qualified by an enumeration
	// definition names something the enumeration does not declare as a literal.
	ErrNotALiteral = errors.New("not a literal of the enumeration")

	// ErrConflictingRedefinition is returned when one declaration values the
	// same feature under two of its names: a redefinition renames one feature,
	// so which of the two values it holds would be a silent pick.
	ErrConflictingRedefinition = errors.New("one feature valued under two names")

	// ErrValuedFeatureRestated is returned when a feature is both bound to a
	// value and given a body restating features of it: the bound value supplies
	// those features, so the restatement could only be silently dropped.
	ErrValuedFeatureRestated = errors.New("feature both valued and restated in a body")

	// ErrFeatureValueMaterialization marks an error as a feature value that could not be
	// materialized, whatever kept it from materializing. Reading a feature value is what
	// finds such a failure, so a surface reporting one answered nothing about
	// that feature value rather than deciding anything about the model.
	ErrFeatureValueMaterialization = errors.New("feature value could not be materialized")

	// ErrNoSuchFeature is returned when a member read, a write, or a chained
	// assignment reaches an object whose type declares no feature of that name:
	// the object has nothing to answer with and nowhere to hold the value.
	ErrNoSuchFeature = errors.New("object has no such feature")

	// ErrNoSubject is returned when the feature a satisfaction assertion names
	// with `by` cannot supply a subject: it resolves to nothing, or no object of
	// it can be created.
	ErrNoSubject = errors.New("no subject to satisfy the requirement")

	// ErrPerformerFeatureNotInScope is returned when a behavior body names a
	// feature only the object performing it declares: the performing object is
	// not a namespace the body's names resolve in, so the name has no referent.
	ErrPerformerFeatureNotInScope = errors.New("name is not in scope of the behavior body")

	// ErrThisNotAnObject is returned when `this` is read where no object owns
	// what is being evaluated: the context occurrence is the performance itself,
	// whose features a name written in its body does not reach.
	ErrThisNotAnObject = errors.New("this names no object here")
)

type budgetExceededError struct {
	message string
	errs    []error
}

func (e *budgetExceededError) Error() string { return e.message }

func (e *budgetExceededError) Unwrap() []error { return e.errs }

func budgetExceeded(sentinel error, message string, causes ...error) error {
	errs := make([]error, 0, len(causes)+1)
	errs = append(errs, sentinel)
	errs = append(errs, causes...)
	return &budgetExceededError{message: message, errs: errs}
}

// NoValueError reports a feature a condition names that carries no value,
// naming the feature so a caller can tell which one is uninitialized.
type NoValueError struct {
	Feature string
	// Ref is the written name whose read found no value, so a caller can tell a
	// read of its own expression from one made while evaluating a default.
	Ref *ast.QualifiedName
	// Symbol is the feature declaration the read reached, when it is known.
	Symbol *symbols.Symbol
}

func (e *NoValueError) Error() string {
	return fmt.Sprintf("%v for feature %s", ErrNoValue, e.Feature)
}

func (e *NoValueError) Unwrap() error { return ErrNoValue }

// UnboundSubjectError reports a check whose subject nothing supplied, naming
// the subject and how a caller supplies one.
type UnboundSubjectError struct {
	Kind    string // "constraint", "requirement", "analysis", "verification" or "objective"
	Element string // name of the element declaring the subject
	Subject string // name of the subject parameter
}

func (e *UnboundSubjectError) Error() string {
	switch e.Kind {
	case "analysis", "verification":
		return fmt.Sprintf("%s %s: %s %v: bind it (`subject %s = <element>`) or run it on an object",
			e.Kind, e.Element, e.Subject, ErrUnboundSubject, e.Subject)
	case "objective":
		return fmt.Sprintf("%s %s: %s %v: bind it (`subject %s = <element>`) or return a result from the case for it to default to",
			e.Kind, e.Element, e.Subject, ErrUnboundSubject, e.Subject)
	}
	return fmt.Sprintf("%s %s: %s %v: bind it (`subject %s = <element>`), check it on an object, or assert `satisfy %s by <element>`",
		e.Kind, e.Element, e.Subject, ErrUnboundSubject, e.Subject, e.Element)
}

func (e *UnboundSubjectError) Unwrap() error { return ErrUnboundSubject }

// ViolationError reports a condition that evaluated to false, naming the
// condition so a verdict says which one failed. It unwraps to ErrViolated,
// since it is a verdict about the model rather than a failure to evaluate.
type ViolationError struct {
	Kind      string // "constraint" or "requirement"
	Element   string // name of the element stating the condition
	What      string // "assertion" or "require condition"
	Condition string // the condition, rendered
}

func (e *ViolationError) Error() string {
	return fmt.Sprintf("%s %s: %s %v: %s", e.Kind, e.Element, e.What, ErrViolated, e.Condition)
}

func (e *ViolationError) Unwrap() error { return ErrViolated }

// FeatureValueError marks a feature value that could not be materialized. It reads as the error
// that kept the feature value from materializing and unwraps to it as well as to
// ErrFeatureValueMaterialization, so a caller tests either.
type FeatureValueError struct {
	Err error
}

func (e *FeatureValueError) Error() string { return e.Err.Error() }

func (e *FeatureValueError) Unwrap() []error { return []error{ErrFeatureValueMaterialization, e.Err} }

// OperandTypeError reports an operator applied to operand types it is not
// defined for, naming the operator and both operands and carrying the span of
// the expression so a surface holding the source can point at it.
type OperandTypeError struct {
	Op      string      // the operator, as written
	Left    string      // description of the left operand's type
	Right   string      // description of the right operand's type
	Library string      // the library function that would have to declare it, if any
	Span    source.Span // span of the operator expression
}

func (e *OperandTypeError) Error() string {
	msg := fmt.Sprintf("%v: operator '%s' is not defined for %s and %s", ErrTypeMismatch, e.Op, e.Left, e.Right)
	if e.Library != "" {
		msg += "; " + e.Library
	}
	return msg
}

func (e *OperandTypeError) Unwrap() error { return ErrTypeMismatch }

// CalcFrameError reports an error raised inside a calc invocation, counting the
// calc frames it propagated through so a recursion reports a depth rather than
// one wrapped line per frame.
type CalcFrameError struct {
	Kind   string // the notation keyword of the calc: `calc` or `analysis`
	Calc   string // the calc the error surfaced from
	Frames int    // calc frames the error propagated through
	Err    error

	// calcs names every calc the chain already passed through, so a cycle is
	// counted rather than wrapped again.
	calcs map[string]bool
}

func (e *CalcFrameError) Error() string {
	if e.Frames > 1 {
		return fmt.Sprintf("%s %s: … %d frames: %v", e.Kind, e.Calc, e.Frames, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Kind, e.Calc, e.Err)
}

func (e *CalcFrameError) Unwrap() error { return e.Err }

// calcFrame adds one calc frame to err. A calc the chain already passed through
// is counted rather than wrapped again, so a recursion reports a depth instead
// of one line per frame, while a calc calling another still names both.
func calcFrame(kind, calc string, err error) error {
	var framed *CalcFrameError
	if errors.As(err, &framed) {
		if framed.calcs[calc] {
			return &CalcFrameError{
				Kind:   kind,
				Calc:   calc,
				Frames: framed.Frames + 1,
				Err:    framed.Err,
				calcs:  framed.calcs,
			}
		}
		calcs := make(map[string]bool, len(framed.calcs)+1)
		for name := range framed.calcs {
			calcs[name] = true
		}
		calcs[calc] = true
		return &CalcFrameError{Kind: kind, Calc: calc, Frames: 1, Err: err, calcs: calcs}
	}
	return &CalcFrameError{Kind: kind, Calc: calc, Frames: 1, Err: err, calcs: map[string]bool{calc: true}}
}

// calcDefaultError reports err raised evaluating calc's default for param as one
// frame of calc, so a default re-invoking its own calc collapses into a count.
func calcDefaultError(kind, calc, param string, err error) error {
	return calcFrame(kind, calc, fmt.Errorf("default for parameter %q: %w", param, err))
}
