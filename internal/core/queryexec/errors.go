package queryexec

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
)

// ErrorKind classifies a document-query execution failure.
type ErrorKind string

const (
	ErrorInvalidContext        ErrorKind = "invalid-context"
	ErrorUnknownBinding        ErrorKind = "unknown-binding"
	ErrorMissingBinding        ErrorKind = "missing-binding"
	ErrorBindingType           ErrorKind = "binding-type"
	ErrorBindingMultiplicity   ErrorKind = "binding-multiplicity"
	ErrorMissingElement        ErrorKind = "missing-element"
	ErrorUnsupportedOperation  ErrorKind = "unsupported-operation"
	ErrorInvalidArgument       ErrorKind = "invalid-argument"
	ErrorInvalidOperator       ErrorKind = "invalid-operator"
	ErrorInvalidOrder          ErrorKind = "invalid-order"
	ErrorUnknownProperty       ErrorKind = "unknown-property"
	ErrorUnknownClassification ErrorKind = "unknown-classification"
	ErrorUnknownRelationship   ErrorKind = "unknown-relationship"
	ErrorUnevaluableFeature    ErrorKind = "unevaluable-feature"
	ErrorUnknownInvocation     ErrorKind = "unknown-invocation"
	ErrorInvocationCycle       ErrorKind = "invocation-cycle"
	ErrorInvocationDepth       ErrorKind = "invocation-depth"
	ErrorInvocationBudget      ErrorKind = "invocation-budget"
	ErrorVisitBudget           ErrorKind = "visit-budget"
	ErrorResultType            ErrorKind = "result-type"
	ErrorResultMultiplicity    ErrorKind = "result-multiplicity"
	ErrorColumnOperand         ErrorKind = "column-operand"
	ErrorColumnOperandType     ErrorKind = "column-operand-type"
	ErrorColumnDivisionByZero  ErrorKind = "column-division-by-zero"
	ErrorColumnIncommensurable ErrorKind = "column-incommensurable"
	ErrorColumnArithmetic      ErrorKind = "column-arithmetic"
	ErrorColumnAbsent          ErrorKind = "column-absent"
	ErrorColumnCardinality     ErrorKind = "column-cardinality"
	// ErrorNoRuntime: an operation over a session's objects ran with no session.
	ErrorNoRuntime ErrorKind = "no-runtime"
	// ErrorObjectRow: a model-only operation was given a runtime object row.
	ErrorObjectRow ErrorKind = "object-row"
	// ErrorVerdictRow: a model-only operation was given a verdict row.
	ErrorVerdictRow ErrorKind = "verdict-row"
	// ErrorNotAnObject: Verdicts was asked about an element that declares no object.
	ErrorNotAnObject ErrorKind = "not-an-object"
	// ErrorIncompleteValidation: Verdicts could not check every assertion about a row.
	ErrorIncompleteValidation ErrorKind = "incomplete-validation"
	// ErrorStateRow: a model-only operation was given a state row.
	ErrorStateRow ErrorKind = "state-row"
	// ErrorEventRow: a model-only operation was given an event row.
	ErrorEventRow ErrorKind = "event-row"
	// ErrorNotHeld: a runtime operation was given an element the session holds no object of.
	ErrorNotHeld ErrorKind = "not-held"
	// ErrorNoStateMachine: States was asked about an object exhibiting no state machine.
	ErrorNoStateMachine ErrorKind = "no-state-machine"
	// ErrorUnknownState: InState named a state no session object's machine declares.
	ErrorUnknownState ErrorKind = "unknown-state"
	// ErrorNoTrace: Events ran in a session that records no trace.
	ErrorNoTrace ErrorKind = "no-trace"
	// ErrorInvalidInterval: an Events bound is not an instant on the clock, or the interval is empty.
	ErrorInvalidInterval ErrorKind = "invalid-interval"
)

// Error is a typed query-execution failure with plan provenance.
type Error struct {
	Kind      ErrorKind
	Query     string
	Operation queryplan.Operation
	Parameter string
	Property  string
	Target    string
	Path      []string
	Expected  string
	Actual    string
	Origin    provenance.Origin
	// Cause is the evaluator's reason an unevaluable feature could not be read.
	Cause error
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorInvalidContext:
		return "document query execution requires a program, index, resolver, and semantic model"
	case ErrorUnknownBinding:
		return fmt.Sprintf("query %s received unknown binding %s", e.Query, e.Parameter)
	case ErrorMissingBinding:
		return fmt.Sprintf("query %s requires binding %s", e.Query, e.Parameter)
	case ErrorBindingType:
		return fmt.Sprintf("query %s binding %s has type %s, expected %s", e.Query, e.Parameter, e.Actual, e.Expected)
	case ErrorBindingMultiplicity:
		return fmt.Sprintf("query %s binding %s has multiplicity %s, expected %s", e.Query, e.Parameter, e.Actual, e.Expected)
	case ErrorMissingElement:
		return fmt.Sprintf("query %s names an element that is not retained in the plan", e.Query)
	case ErrorUnsupportedOperation:
		return fmt.Sprintf("query %s operation %s is not executable in this engine version", e.Query, e.Operation)
	case ErrorInvalidArgument:
		return fmt.Sprintf("query %s operation %s has invalid argument %s", e.Query, e.Operation, e.Parameter)
	case ErrorInvalidOperator:
		return fmt.Sprintf("query %s operation %s does not support %q", e.Query, e.Operation, e.Actual)
	case ErrorInvalidOrder:
		if e.Expected != "" || e.Actual != "" {
			return fmt.Sprintf("query %s cannot order property %s across incommensurable units %s and %s", e.Query, e.Property, e.Expected, e.Actual)
		}
		return fmt.Sprintf("query %s cannot order incomparable values of property %s", e.Query, e.Property)
	case ErrorUnknownProperty:
		return fmt.Sprintf("query %s references unknown property %s", e.Query, e.Property)
	case ErrorUnknownClassification:
		return fmt.Sprintf("query %s references unknown classification %s", e.Query, e.Actual)
	case ErrorUnknownRelationship:
		return fmt.Sprintf("query %s does not support relationship kind %q", e.Query, e.Actual)
	case ErrorUnevaluableFeature:
		message := fmt.Sprintf("query %s cannot evaluate feature %s", e.Query, e.Property)
		if e.Target != "" {
			message += " of " + e.Target
		}
		if e.Cause != nil {
			message += ": " + e.Cause.Error()
		}
		return message
	case ErrorUnknownInvocation:
		return fmt.Sprintf("query %s invokes %s, which is not compiled into the plan", e.Query, e.Target)
	case ErrorInvocationCycle:
		return fmt.Sprintf("query %s re-entered %s during invocation: %s", e.Query, e.Target, strings.Join(e.Path, " -> "))
	case ErrorInvocationDepth:
		return fmt.Sprintf("query %s exceeded the invocation depth limit invoking %s", e.Query, e.Target)
	case ErrorInvocationBudget:
		return fmt.Sprintf("query %s exceeded the invocation budget invoking %s", e.Query, e.Target)
	case ErrorVisitBudget:
		return fmt.Sprintf("query %s exceeded its visit budget", e.Query)
	case ErrorNoRuntime:
		return fmt.Sprintf("query %s operation %s reads a session's objects, and this execution has no session: instantiate an object first", e.Query, e.Operation)
	case ErrorObjectRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to object %s", e.Query, e.Operation, e.Target)
	case ErrorVerdictRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to verdict %s", e.Query, e.Operation, e.Target)
	case ErrorNotAnObject:
		return fmt.Sprintf("query %s operation %s cannot check %s, which declares no object", e.Query, e.Operation, e.Target)
	case ErrorIncompleteValidation:
		return fmt.Sprintf("query %s operation %s could not check every assertion about %s: %v", e.Query, e.Operation, e.Target, e.Cause)
	case ErrorStateRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to state %s", e.Query, e.Operation, e.Target)
	case ErrorEventRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to event %s", e.Query, e.Operation, e.Target)
	case ErrorNotHeld:
		return fmt.Sprintf("query %s operation %s reads the objects of %s, and the session holds none", e.Query, e.Operation, e.Target)
	case ErrorNoStateMachine:
		return fmt.Sprintf("query %s operation %s asks the state of %s, which exhibits no state machine", e.Query, e.Operation, e.Target)
	case ErrorUnknownState:
		return fmt.Sprintf("query %s operation %s names state %s, which no state machine the session runs declares", e.Query, e.Operation, e.Actual)
	case ErrorNoTrace:
		return fmt.Sprintf("query %s operation %s reads the session's trace, and this session records none: turn tracing on before running", e.Query, e.Operation)
	case ErrorInvalidInterval:
		message := fmt.Sprintf("query %s operation %s has an invalid time interval", e.Query, e.Operation)
		if e.Parameter != "" {
			message = fmt.Sprintf("query %s operation %s bound %s is not an instant on the clock", e.Query, e.Operation, e.Parameter)
		}
		if e.Cause != nil {
			message += ": " + e.Cause.Error()
		} else if e.Actual != "" {
			message += ": " + e.Actual
		}
		return message
	case ErrorResultType:
		return fmt.Sprintf("query %s produced %s, expected %s", e.Query, e.Actual, e.Expected)
	case ErrorResultMultiplicity:
		return fmt.Sprintf("query %s produced multiplicity %s, expected %s", e.Query, e.Actual, e.Expected)
	case ErrorColumnOperand:
		return fmt.Sprintf(
			"query %s column %s requires one value per %q operand, got %s for %s",
			e.Query,
			e.Property,
			e.Parameter,
			e.Actual,
			e.Target,
		)
	case ErrorColumnOperandType:
		return fmt.Sprintf(
			"query %s column %s cannot apply %q to %s for %s",
			e.Query,
			e.Property,
			e.Parameter,
			e.Actual,
			e.Target,
		)
	case ErrorColumnAbsent:
		return fmt.Sprintf(
			"query %s column %s has no value for %s; use ?? to supply a default",
			e.Query,
			e.Property,
			e.Target,
		)
	case ErrorColumnCardinality:
		return fmt.Sprintf(
			"query %s column %s produced %s values, expected one for %s",
			e.Query,
			e.Property,
			e.Actual,
			e.Target,
		)
	case ErrorColumnDivisionByZero:
		return fmt.Sprintf("query %s column %s divides by zero for %s", e.Query, e.Property, e.Target)
	case ErrorColumnIncommensurable:
		return fmt.Sprintf(
			"query %s column %s cannot apply %q to quantities in incommensurable units %s for %s",
			e.Query,
			e.Property,
			e.Parameter,
			e.Actual,
			e.Target,
		)
	case ErrorColumnArithmetic:
		return fmt.Sprintf("query %s column %s cannot compute %q for %s: %s", e.Query, e.Property, e.Parameter, e.Target, e.Actual)
	default:
		return fmt.Sprintf("query execution failed for %s", e.Query)
	}
}
