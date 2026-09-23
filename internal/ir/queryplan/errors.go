package queryplan

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// ErrorKind classifies a query-planning failure.
type ErrorKind string

const (
	ErrorInvalidContext         ErrorKind = "invalid-context"
	ErrorLibraryUnavailable     ErrorKind = "library-unavailable"
	ErrorNotQueryDefinition     ErrorKind = "not-query-definition"
	ErrorMissingResultParameter ErrorKind = "missing-result-parameter"
	ErrorInvalidParameter       ErrorKind = "invalid-parameter"
	ErrorMissingResult          ErrorKind = "missing-result"
	ErrorConflictingResult      ErrorKind = "conflicting-result"
	ErrorUnsupportedResult      ErrorKind = "unsupported-result"
	ErrorUnknownInvocation      ErrorKind = "unknown-invocation"
	ErrorAmbiguousInvocation    ErrorKind = "ambiguous-invocation"
	ErrorPositionalQueryArgs    ErrorKind = "positional-query-arguments"
	ErrorDuplicateArgument      ErrorKind = "duplicate-argument"
	ErrorUnknownArgument        ErrorKind = "unknown-argument"
	ErrorMissingArgument        ErrorKind = "missing-argument"
	ErrorArgumentCount          ErrorKind = "argument-count"
	ErrorArgumentType           ErrorKind = "argument-type"
	ErrorArgumentMultiplicity   ErrorKind = "argument-multiplicity"
	ErrorCompositionCycle       ErrorKind = "composition-cycle"
	ErrorUnknownParameter       ErrorKind = "unknown-parameter"
	ErrorUnsupportedExpression  ErrorKind = "unsupported-expression"
	ErrorUnsupportedDefault     ErrorKind = "unsupported-default"
	ErrorDefaultType            ErrorKind = "default-type"
	ErrorDefaultMultiplicity    ErrorKind = "default-multiplicity"
	ErrorInvalidColumn          ErrorKind = "invalid-column"
	ErrorColumnName             ErrorKind = "column-name"
	ErrorUnknownColumnProperty  ErrorKind = "unknown-column-property"
	ErrorColumnOperator         ErrorKind = "column-operator"
	ErrorColumnType             ErrorKind = "column-type"
	ErrorDuplicateColumn        ErrorKind = "duplicate-column"
	ErrorEmptyProjection        ErrorKind = "empty-projection"
	ErrorColumnAggregate        ErrorKind = "column-aggregate"
)

// Error is a typed query-planning failure with its source location.
type Error struct {
	Kind      ErrorKind
	Query     string
	Target    string
	Parameter string
	Path      []string
	Expected  string
	Actual    string
	Origin    symbols.Origin
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorInvalidContext:
		return "document query planning requires an index, resolver, and semantic model"
	case ErrorLibraryUnavailable:
		return "document query library is unavailable"
	case ErrorNotQueryDefinition:
		return fmt.Sprintf("%s is not a document query definition", e.Query)
	case ErrorMissingResultParameter:
		return fmt.Sprintf("query %s has no result parameter", e.Query)
	case ErrorInvalidParameter:
		return fmt.Sprintf("query %s parameter %s must be an input parameter", e.Query, e.Parameter)
	case ErrorMissingResult:
		return fmt.Sprintf("query %s has no result expression", e.Query)
	case ErrorConflictingResult:
		return fmt.Sprintf("query %s inherits conflicting result expressions", e.Query)
	case ErrorUnsupportedResult:
		return fmt.Sprintf("query %s result must be one query expression", e.Query)
	case ErrorUnknownInvocation:
		return fmt.Sprintf("query %s invokes unknown operation %s", e.Query, e.Target)
	case ErrorAmbiguousInvocation:
		return fmt.Sprintf("query %s invokes %s, ambiguous between %s", e.Query, e.Target, strings.Join(e.Path, ", "))
	case ErrorPositionalQueryArgs:
		return fmt.Sprintf("query %s must invoke query %s with named arguments", e.Query, e.Target)
	case ErrorDuplicateArgument:
		return fmt.Sprintf("query %s binds parameter %s more than once", e.Query, e.Parameter)
	case ErrorUnknownArgument:
		return fmt.Sprintf("query %s binds unknown parameter %s of %s", e.Query, e.Parameter, e.Target)
	case ErrorMissingArgument:
		return fmt.Sprintf("query %s does not bind required parameter %s of %s", e.Query, e.Parameter, e.Target)
	case ErrorArgumentCount:
		return fmt.Sprintf("query %s invokes %s with the wrong number of arguments", e.Query, e.Target)
	case ErrorArgumentType:
		return fmt.Sprintf(
			"query %s binds parameter %s of %s with type %s, expected %s",
			e.Query,
			e.Parameter,
			e.Target,
			e.Actual,
			e.Expected,
		)
	case ErrorArgumentMultiplicity:
		return fmt.Sprintf(
			"query %s binds parameter %s of %s with multiplicity %s, expected %s",
			e.Query,
			e.Parameter,
			e.Target,
			e.Actual,
			e.Expected,
		)
	case ErrorCompositionCycle:
		return fmt.Sprintf("document query composition cycle: %s", strings.Join(e.Path, " -> "))
	case ErrorUnknownParameter:
		return fmt.Sprintf("query %s references unknown parameter %s", e.Query, e.Parameter)
	case ErrorUnsupportedExpression:
		return fmt.Sprintf("query %s contains an unsupported result expression", e.Query)
	case ErrorUnsupportedDefault:
		return fmt.Sprintf("query %s parameter %s declares a default the plan cannot represent", e.Query, e.Parameter)
	case ErrorDefaultType:
		return fmt.Sprintf(
			"query %s parameter %s declares a default of type %s, expected %s",
			e.Query,
			e.Parameter,
			e.Actual,
			e.Expected,
		)
	case ErrorDefaultMultiplicity:
		return fmt.Sprintf(
			"query %s parameter %s declares a default with multiplicity %s, expected %s",
			e.Query,
			e.Parameter,
			e.Actual,
			e.Expected,
		)
	case ErrorInvalidColumn:
		return fmt.Sprintf("query %s must build the columns of Project from Column(name, expression) or RelatedColumn(name, relationshipKind, direction, maxDepth, aggregate) invocations", e.Query)
	case ErrorColumnName:
		return fmt.Sprintf("query %s must name each computed column with a string literal", e.Query)
	case ErrorUnknownColumnProperty:
		return fmt.Sprintf("query %s column %s references unknown property %s", e.Query, e.Target, e.Parameter)
	case ErrorColumnOperator:
		return fmt.Sprintf("query %s column %s does not support operator %q", e.Query, e.Target, e.Actual)
	case ErrorColumnType:
		return fmt.Sprintf(
			"query %s column %s cannot apply %q to %s",
			e.Query,
			e.Target,
			e.Parameter,
			e.Actual,
		)
	case ErrorDuplicateColumn:
		return fmt.Sprintf("query %s projects column %s more than once", e.Query, e.Parameter)
	case ErrorEmptyProjection:
		return fmt.Sprintf("query %s must project at least one property or computed column", e.Query)
	case ErrorColumnAggregate:
		return fmt.Sprintf("query %s column %s does not support aggregate %q", e.Query, e.Target, e.Actual)
	default:
		return fmt.Sprintf("query planning failed for %s", e.Query)
	}
}
