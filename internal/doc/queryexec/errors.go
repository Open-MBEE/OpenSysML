package queryexec

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	ErrorUnknownElement        ErrorKind = "unknown-element"
	// ErrorAmbiguousElement: Named was given a name several elements carry — a
	// finding about the model, where ErrorUnknownElement is one about the query.
	ErrorAmbiguousElement    ErrorKind = "ambiguous-element"
	ErrorUnknownRelationship ErrorKind = "unknown-relationship"
	// ErrorMissingFeature: Project or WhereFeature read a feature an element row
	// lacks — a path error about that element; ErrorUnknownProperty: no row had it.
	ErrorMissingFeature        ErrorKind = "missing-feature"
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
	// ErrorObjectDestroyed: States or Events was given an object the run destroyed.
	ErrorObjectDestroyed ErrorKind = "object-destroyed"
	// ErrorNoTrace: Events ran in a session that records no trace.
	ErrorNoTrace ErrorKind = "no-trace"
	// ErrorInvalidInterval: an Events bound is not an instant on the clock, or the interval is empty.
	ErrorInvalidInterval ErrorKind = "invalid-interval"
	// ErrorTraceTruncated: Events reaches back to records the session's bounded trace has dropped.
	ErrorTraceTruncated ErrorKind = "trace-truncated"
	// ErrorUndeclaredRow: a RelatedColumn was to traverse from a row no element declares.
	ErrorUndeclaredRow ErrorKind = "undeclared-row"
	// ErrorProjectedAncestors: Tree was given ancestors for rows already projected.
	ErrorProjectedAncestors ErrorKind = "projected-ancestors"
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
	Origin    symbols.Origin
	// Candidates are what the model offers in place of what the query asked for:
	// the spellings nearest an unresolved name, or the elements an ambiguous name denotes.
	Candidates []string
	// Outcome is the QVTS record behind a path or model error (outcome.go); nil for
	// the failures that are neither, the flat fields above being its rendering.
	Outcome *Outcome
	// Also names the other elements the same fan-out failed on (missing-feature).
	Also []string
	// Cause is the evaluator's reason an unevaluable feature could not be read.
	Cause error
}

func (e *Error) Unwrap() error { return e.Cause }

func asError(err error, target **Error) bool { return errors.As(err, target) }

// declaredAs lists the qualified names an ambiguous name denotes, a repeated
// one once with its count: `Duplicates::a (declared 2 times)`.
func declaredAs(fqns []string) string {
	var out []string
	for i, fqn := range fqns {
		if i > 0 && fqn == fqns[i-1] {
			continue
		}
		n := 0
		for _, other := range fqns {
			if other == fqn {
				n++
			}
		}
		if n > 1 {
			fqn = fmt.Sprintf("%s (declared %d times)", fqn, n)
		}
		out = append(out, fqn)
	}
	return strings.Join(out, ", ")
}

// unknownElementMessage says where the name stopped resolving (Target has no
// member Expected) and what is spelled nearest; an alias denoting nothing says so.
func (e *Error) unknownElementMessage() string {
	msg := fmt.Sprintf("query %s names no element %s", e.Query, e.Actual)
	switch {
	case len(e.Path) > 0:
		return fmt.Sprintf("%s: the alias %s denotes no element", msg, e.Path[0])
	case e.Target != "":
		msg += fmt.Sprintf(": %s has no member %s", e.Target, e.Expected)
	case e.Expected != "":
		msg += fmt.Sprintf(": nothing is named %s", e.Expected)
	}
	return suggest.With(msg, e.Expected, e.Candidates)
}

// missingFeatureMessage names the element lacking the feature and the features
// it does have, so the reader sees where the path left the model.
func (e *Error) missingFeatureMessage() string {
	msg := fmt.Sprintf("query %s reads feature %s of %s, which has no such feature", e.Query, e.Property, e.Target)
	if e.Actual != "" {
		msg = fmt.Sprintf("query %s reads feature %s of %s, but the path stops at %s", e.Query, e.Property, e.Target, e.Actual)
	}
	if len(e.Candidates) > 0 {
		msg += ": its features are " + strings.Join(e.Candidates, ", ")
		if len(e.Candidates) > 3 {
			msg = suggest.With(msg, e.missingSegment(), e.Candidates)
		}
	}
	if len(e.Also) > 0 {
		msg += fmt.Sprintf(" (also absent on %s)", strings.Join(e.Also, ", "))
	}
	return msg
}

// missingSegment is the segment the record says names nothing: the failed step's
// argument, so a path that repeats a name cannot misdirect the suggestion.
func (e *Error) missingSegment() string {
	if e.Outcome != nil && e.Outcome.Path != nil && len(e.Outcome.Path.Failures) > 0 {
		return e.Outcome.Path.Failures[0].Step.Argument
	}
	return e.Property
}

// column names the projected column an error arose in, when it did in one.
func (e *Error) column() string {
	if e.Property == "" {
		return ""
	}
	return " column " + e.Property
}

func (e *Error) Error() string {
	if message, ok := e.columnMessage(); ok {
		return message
	}
	if message, ok := e.sessionMessage(); ok {
		return message
	}
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
		return fmt.Sprintf("query %s operation %s%s has invalid argument %s", e.Query, e.Operation, e.column(), e.Parameter)
	case ErrorProjectedAncestors:
		return fmt.Sprintf("query %s operation Tree takes ancestors over unprojected rows, and its source is projected: nest before Project", e.Query)
	case ErrorInvalidOperator:
		return fmt.Sprintf("query %s operation %s%s does not support %q", e.Query, e.Operation, e.column(), e.Actual)
	case ErrorInvalidOrder:
		if e.Expected != "" || e.Actual != "" {
			return fmt.Sprintf("query %s cannot order property %s across incommensurable units %s and %s", e.Query, e.Property, e.Expected, e.Actual)
		}
		return fmt.Sprintf("query %s cannot order incomparable values of property %s", e.Query, e.Property)
	case ErrorUnknownProperty:
		return fmt.Sprintf("query %s references unknown property %s", e.Query, e.Property)
	case ErrorUnknownClassification:
		return fmt.Sprintf("query %s references unknown classification %s", e.Query, e.Actual)
	case ErrorMissingFeature:
		return e.missingFeatureMessage()
	case ErrorUnknownElement:
		return e.unknownElementMessage()
	case ErrorAmbiguousElement:
		return fmt.Sprintf("query %s names %d elements %s: %s", e.Query, len(e.Candidates), e.Actual, declaredAs(e.Candidates))
	case ErrorUnknownRelationship:
		return fmt.Sprintf("query %s%s does not support relationship kind %q", e.Query, e.column(), e.Actual)
	case ErrorUnevaluableFeature:
		if e.Property == "" {
			message := fmt.Sprintf("query %s cannot evaluate %s", e.Query, e.Target)
			if e.Cause != nil {
				message += ": " + e.Cause.Error()
			}
			return message
		}
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
		if e.Property != "" {
			return fmt.Sprintf("query %s exceeded its visit budget in column %s", e.Query, e.Property)
		}
		return fmt.Sprintf("query %s exceeded its visit budget", e.Query)
	case ErrorResultType:
		return fmt.Sprintf("query %s produced %s, expected %s", e.Query, e.Actual, e.Expected)
	case ErrorResultMultiplicity:
		return fmt.Sprintf("query %s produced multiplicity %s, expected %s", e.Query, e.Actual, e.Expected)
	default:
		return fmt.Sprintf("query execution failed for %s", e.Query)
	}
}

// columnMessage spells the failures of a computed column.
func (e *Error) columnMessage() (string, bool) {
	switch e.Kind {
	case ErrorColumnOperand:
		return fmt.Sprintf(
			"query %s column %s requires one value per %q operand, got %s for %s",
			e.Query,
			e.Property,
			e.Parameter,
			e.Actual,
			e.Target,
		), true
	case ErrorColumnOperandType:
		return fmt.Sprintf(
			"query %s column %s cannot apply %q to %s for %s",
			e.Query,
			e.Property,
			e.Parameter,
			e.Actual,
			e.Target,
		), true
	case ErrorColumnAbsent:
		return fmt.Sprintf(
			"query %s column %s has no value for %s; use ?? to supply a default",
			e.Query,
			e.Property,
			e.Target,
		), true
	case ErrorColumnCardinality:
		return fmt.Sprintf(
			"query %s column %s produced %s values for %s, outside its declared multiplicity %s",
			e.Query,
			e.Property,
			e.Actual,
			e.Target,
			e.Expected,
		), true
	case ErrorColumnDivisionByZero:
		return fmt.Sprintf("query %s column %s divides by zero for %s", e.Query, e.Property, e.Target), true
	case ErrorColumnIncommensurable:
		return fmt.Sprintf(
			"query %s column %s cannot apply %q to quantities in incommensurable units %s for %s",
			e.Query,
			e.Property,
			e.Parameter,
			e.Actual,
			e.Target,
		), true
	case ErrorColumnArithmetic:
		return fmt.Sprintf("query %s column %s cannot compute %q for %s: %s", e.Query, e.Property, e.Parameter, e.Target, e.Actual), true
	default:
		return "", false
	}
}

// sessionMessage spells the failures of an operation over a session's objects.
func (e *Error) sessionMessage() (string, bool) {
	switch e.Kind {
	case ErrorNoRuntime:
		return fmt.Sprintf("query %s operation %s reads a session's objects, and this execution has no session: instantiate an object first", e.Query, e.Operation), true
	case ErrorObjectRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to object %s", e.Query, e.Operation, e.Target), true
	case ErrorVerdictRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to verdict %s", e.Query, e.Operation, e.Target), true
	case ErrorNotAnObject:
		return fmt.Sprintf("query %s operation %s cannot check %s, which declares no object", e.Query, e.Operation, e.Target), true
	case ErrorIncompleteValidation:
		return fmt.Sprintf("query %s operation %s could not check every assertion about %s: %v", e.Query, e.Operation, e.Target, e.Cause), true
	case ErrorStateRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to state %s", e.Query, e.Operation, e.Target), true
	case ErrorEventRow:
		return fmt.Sprintf("query %s operation %s applies to model elements, not to event %s", e.Query, e.Operation, e.Target), true
	case ErrorUndeclaredRow:
		return fmt.Sprintf("query %s operation %s%s traverses from %s, which no element declares", e.Query, e.Operation, e.column(), e.Target), true
	case ErrorNotHeld:
		return fmt.Sprintf("query %s operation %s reads the objects of %s, and the session holds none", e.Query, e.Operation, e.Target), true
	case ErrorNoStateMachine:
		return fmt.Sprintf("query %s operation %s asks the state of %s, which exhibits no state machine", e.Query, e.Operation, e.Target), true
	case ErrorUnknownState:
		return fmt.Sprintf("query %s operation %s names state %s, which no state machine the session runs declares", e.Query, e.Operation, e.Actual), true
	case ErrorObjectDestroyed:
		return fmt.Sprintf("query %s operation %s asks after %s, an object destroyed at %s", e.Query, e.Operation, e.Target, e.Actual), true
	case ErrorNoTrace:
		return fmt.Sprintf("query %s operation %s reads the session's trace, and this session records none: turn tracing on before running", e.Query, e.Operation), true
	case ErrorTraceTruncated:
		return fmt.Sprintf("query %s operation %s reaches back to records the session's trace no longer keeps (%s): bound since to a later instant", e.Query, e.Operation, e.Actual), true
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
		return message, true
	default:
		return "", false
	}
}
