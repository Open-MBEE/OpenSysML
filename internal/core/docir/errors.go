package docir

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrorKind classifies a document-evaluation failure.
type ErrorKind string

const (
	ErrorInvalidContext          ErrorKind = "invalid-context"
	ErrorInvalidPlan             ErrorKind = "invalid-plan"
	ErrorQueryExecution          ErrorKind = "query-execution"
	ErrorViewRendering           ErrorKind = "view-rendering"
	ErrorUnknownGroup            ErrorKind = "unknown-group-column"
	ErrorUnknownRunColumn        ErrorKind = "unknown-run-column"
	ErrorInvalidRunStyle         ErrorKind = "invalid-run-style"
	ErrorInvalidRunTarget        ErrorKind = "invalid-run-target"
	ErrorBlankMath               ErrorKind = "blank-math"
	ErrorUnknownDefinitionColumn ErrorKind = "unknown-definition-column"
)

// Error is a typed document-evaluation failure with its source location.
type Error struct {
	Kind     ErrorKind
	Document string
	Content  string
	Query    string
	Column   string
	Row      int
	Actual   string
	Origin   symbols.Origin
	Err      error
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorInvalidContext:
		return "document evaluation requires an index, resolver, and semantic model"
	case ErrorInvalidPlan:
		return "document evaluation requires a compiled document plan"
	case ErrorQueryExecution:
		return fmt.Sprintf("document %s content %s query %s: %v", e.Document, e.Content, e.Query, e.Err)
	case ErrorViewRendering:
		return fmt.Sprintf("document %s diagram %s: %v", e.Document, e.Content, e.Err)
	case ErrorUnknownGroup:
		return fmt.Sprintf("document %s table %s groups by %q, which query %s did not project", e.Document, e.Content, e.Actual, e.Query)
	case ErrorUnknownRunColumn:
		return fmt.Sprintf("document %s content %s column run names column %q, which query %s did not project", e.Document, e.Content, e.Column, e.Query)
	case ErrorInvalidRunStyle:
		return fmt.Sprintf(
			"document %s content %s query %s row %d column %q must supply one style \"plain\", \"emphasis\", \"strong\", \"code\" or \"math\", got %s",
			e.Document, e.Content, e.Query, e.Row, e.Column, e.Actual,
		)
	case ErrorBlankMath:
		return fmt.Sprintf(
			"document %s content %s query %s row %d column %q is typeset as math but supplies no LaTeX",
			e.Document, e.Content, e.Query, e.Row, e.Column,
		)
	case ErrorInvalidRunTarget:
		return fmt.Sprintf(
			"document %s content %s query %s row %d column %q must supply one non-empty link target, got %s",
			e.Document, e.Content, e.Query, e.Row, e.Column, e.Actual,
		)
	case ErrorUnknownDefinitionColumn:
		return fmt.Sprintf("document %s definitions %s names column %q, which query %s did not project", e.Document, e.Content, e.Column, e.Query)
	default:
		return fmt.Sprintf("document evaluation failed for %s", e.Document)
	}
}

// Unwrap returns the underlying execution or rendering failure, if any.
func (e *Error) Unwrap() error { return e.Err }
