package docplan

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
)

// ErrorKind classifies a document-planning failure.
type ErrorKind string

const (
	ErrorInvalidContext          ErrorKind = "invalid-context"
	ErrorLibraryUnavailable      ErrorKind = "library-unavailable"
	ErrorNotDocumentDefinition   ErrorKind = "not-document-definition"
	ErrorMissingTitle            ErrorKind = "missing-title"
	ErrorInvalidAttribute        ErrorKind = "invalid-attribute"
	ErrorInvalidContent          ErrorKind = "invalid-content"
	ErrorNestedDocument          ErrorKind = "nested-document"
	ErrorMissingText             ErrorKind = "missing-text"
	ErrorConflictingText         ErrorKind = "conflicting-text"
	ErrorMissingQuery            ErrorKind = "missing-query"
	ErrorConflictingQuery        ErrorKind = "conflicting-query"
	ErrorUnknownQuery            ErrorKind = "unknown-query"
	ErrorQueryPlanning           ErrorKind = "query-planning"
	ErrorInvalidStyle            ErrorKind = "invalid-style"
	ErrorUnknownParameter        ErrorKind = "unknown-parameter"
	ErrorDuplicateBinding        ErrorKind = "duplicate-binding"
	ErrorMissingBinding          ErrorKind = "missing-binding"
	ErrorUnsupportedBinding      ErrorKind = "unsupported-binding"
	ErrorBindingType             ErrorKind = "binding-type"
	ErrorBindingMultiplicity     ErrorKind = "binding-multiplicity"
	ErrorMissingViewSource       ErrorKind = "missing-view-source"
	ErrorInvalidViewSource       ErrorKind = "invalid-view-source"
	ErrorUnknownViewSource       ErrorKind = "unknown-view-source"
	ErrorMissingDiagramKind      ErrorKind = "missing-diagram-kind"
	ErrorConflictingKind         ErrorKind = "conflicting-diagram-kind"
	ErrorUnsupportedKind         ErrorKind = "unsupported-diagram-kind"
	ErrorInvalidDirection        ErrorKind = "invalid-direction"
	ErrorUnsupportedDirection    ErrorKind = "unsupported-direction"
	ErrorConflictingRuns         ErrorKind = "conflicting-runs"
	ErrorAmbiguousRun            ErrorKind = "ambiguous-run"
	ErrorMissingRunText          ErrorKind = "missing-run-text"
	ErrorInvalidRunStyle         ErrorKind = "invalid-run-style"
	ErrorMissingLinkTarget       ErrorKind = "missing-link-target"
	ErrorMissingRefTarget        ErrorKind = "missing-ref-target"
	ErrorUnknownRefTarget        ErrorKind = "unknown-ref-target"
	ErrorInvalidRefTarget        ErrorKind = "invalid-ref-target"
	ErrorAmbiguousRefTarget      ErrorKind = "ambiguous-ref-target"
	ErrorUnknownGroupColumn      ErrorKind = "unknown-group-column"
	ErrorColumnRunWithoutQuery   ErrorKind = "column-run-without-query"
	ErrorConflictingColumnRuns   ErrorKind = "conflicting-column-runs"
	ErrorMissingRunColumn        ErrorKind = "missing-run-column"
	ErrorConflictingRunStyle     ErrorKind = "conflicting-run-style"
	ErrorUnknownRunColumn        ErrorKind = "unknown-run-column"
	ErrorMissingDefinitionColumn ErrorKind = "missing-definition-column"
	ErrorUnknownDefinitionColumn ErrorKind = "unknown-definition-column"
)

// Error is a typed document-planning failure with its source location.
type Error struct {
	Kind      ErrorKind
	Document  string
	Content   string
	Query     string
	Parameter string
	Expected  string
	Actual    string
	Origin    provenance.Origin
	Err       error
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorInvalidContext:
		return "document planning requires an index, resolver, and semantic model"
	case ErrorLibraryUnavailable:
		return "document query library is unavailable"
	case ErrorNotDocumentDefinition:
		return fmt.Sprintf("%s is not a document definition", e.Document)
	case ErrorMissingTitle:
		return fmt.Sprintf("document %s content %s has no title", e.Document, e.Content)
	case ErrorInvalidAttribute:
		return fmt.Sprintf(
			"document %s content %s attribute %s must be one string literal",
			e.Document,
			e.Content,
			e.Parameter,
		)
	case ErrorInvalidContent:
		return fmt.Sprintf("document %s contains unsupported content %s", e.Document, e.Content)
	case ErrorNestedDocument:
		return fmt.Sprintf("document %s nests document %s", e.Document, e.Content)
	case ErrorMissingText:
		return fmt.Sprintf("document %s paragraph %s has neither text, inline runs, nor a query", e.Document, e.Content)
	case ErrorConflictingText:
		return fmt.Sprintf("document %s paragraph %s has both text and a query", e.Document, e.Content)
	case ErrorMissingQuery:
		return fmt.Sprintf("document %s content %s references no query", e.Document, e.Content)
	case ErrorConflictingQuery:
		return fmt.Sprintf("document %s content %s references more than one query", e.Document, e.Content)
	case ErrorUnknownQuery:
		return fmt.Sprintf("document %s content %s references unknown query %s", e.Document, e.Content, e.Query)
	case ErrorQueryPlanning:
		return fmt.Sprintf("document %s content %s query %s: %v", e.Document, e.Content, e.Query, e.Err)
	case ErrorInvalidStyle:
		return fmt.Sprintf(
			"document %s list %s style must be %q or %q, got %q",
			e.Document,
			e.Content,
			ListBullet,
			ListNumber,
			e.Actual,
		)
	case ErrorUnknownParameter:
		return fmt.Sprintf("document %s content %s binds unknown parameter %s of %s", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorDuplicateBinding:
		return fmt.Sprintf("document %s content %s binds parameter %s of %s more than once", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorMissingBinding:
		return fmt.Sprintf("document %s content %s does not bind required parameter %s of %s", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorUnsupportedBinding:
		return fmt.Sprintf("document %s content %s binds parameter %s of %s with an unsupported expression", e.Document, e.Content, e.Parameter, e.Query)
	case ErrorBindingType:
		return fmt.Sprintf(
			"document %s content %s binds parameter %s of %s with type %s, expected %s",
			e.Document,
			e.Content,
			e.Parameter,
			e.Query,
			e.Actual,
			e.Expected,
		)
	case ErrorBindingMultiplicity:
		return fmt.Sprintf(
			"document %s content %s binds parameter %s of %s with multiplicity %s, expected %s",
			e.Document,
			e.Content,
			e.Parameter,
			e.Query,
			e.Actual,
			e.Expected,
		)
	case ErrorMissingViewSource:
		return fmt.Sprintf("document %s diagram %s names no source view or element", e.Document, e.Content)
	case ErrorInvalidViewSource:
		return fmt.Sprintf("document %s diagram %s source must name a view or an element", e.Document, e.Content)
	case ErrorUnknownViewSource:
		return fmt.Sprintf("document %s diagram %s names unknown source %s", e.Document, e.Content, e.Actual)
	case ErrorMissingDiagramKind:
		return fmt.Sprintf("document %s diagram %s renders a plain element and must state a kind", e.Document, e.Content)
	case ErrorConflictingKind:
		return fmt.Sprintf("document %s diagram %s states kind %q, but its source is a view that states its own rendering", e.Document, e.Content, e.Actual)
	case ErrorUnsupportedKind:
		if e.Err != nil {
			return fmt.Sprintf("document %s diagram %s: %v", e.Document, e.Content, e.Err)
		}
		return fmt.Sprintf("document %s diagram %s states unsupported kind %q", e.Document, e.Content, e.Actual)
	case ErrorInvalidDirection:
		return fmt.Sprintf("document %s diagram %s direction must be \"TB\", \"LR\", \"RL\" or \"BT\", got %q", e.Document, e.Content, e.Actual)
	case ErrorUnsupportedDirection:
		return fmt.Sprintf("document %s diagram %s states direction %q, but a %s rendering has none", e.Document, e.Content, e.Actual, e.Expected)
	case ErrorConflictingRuns:
		return fmt.Sprintf("document %s paragraph %s declares inline runs alongside text or a query", e.Document, e.Content)
	case ErrorAmbiguousRun:
		return fmt.Sprintf("document %s run %s conforms to more than one run kind", e.Document, e.Content)
	case ErrorMissingRunText:
		return fmt.Sprintf("document %s run %s states no text", e.Document, e.Content)
	case ErrorInvalidRunStyle:
		return fmt.Sprintf(
			"document %s run %s style must be %q, %q, %q or %q, got %q",
			e.Document,
			e.Content,
			StylePlain,
			StyleEmphasis,
			StyleStrong,
			StyleCode,
			e.Actual,
		)
	case ErrorMissingLinkTarget:
		return fmt.Sprintf("document %s link %s states no target", e.Document, e.Content)
	case ErrorMissingRefTarget:
		return fmt.Sprintf("document %s reference %s names no target", e.Document, e.Content)
	case ErrorUnknownRefTarget:
		return fmt.Sprintf("document %s reference %s names unknown target %s", e.Document, e.Content, e.Actual)
	case ErrorAmbiguousRefTarget:
		return fmt.Sprintf("document %s reference %s targets a usage typed by more than one document: %s", e.Document, e.Content, e.Actual)
	case ErrorInvalidRefTarget:
		return fmt.Sprintf("document %s reference %s must target a named content block of a document, or another document itself, got %s", e.Document, e.Content, e.Actual)
	case ErrorUnknownGroupColumn:
		return fmt.Sprintf("document %s table %s groups by %q, which its query does not project", e.Document, e.Content, e.Actual)
	case ErrorColumnRunWithoutQuery:
		return fmt.Sprintf("document %s paragraph %s declares column runs but no query", e.Document, e.Content)
	case ErrorConflictingColumnRuns:
		return fmt.Sprintf("document %s paragraph %s declares column runs alongside text or inline runs", e.Document, e.Content)
	case ErrorMissingRunColumn:
		return fmt.Sprintf("document %s column run %s states no %s", e.Document, e.Content, e.Parameter)
	case ErrorConflictingRunStyle:
		return fmt.Sprintf("document %s column run %s states both style and styleColumn", e.Document, e.Content)
	case ErrorUnknownRunColumn:
		return fmt.Sprintf("document %s column run %s names column %q, which query %s does not project", e.Document, e.Content, e.Actual, e.Query)
	case ErrorMissingDefinitionColumn:
		return fmt.Sprintf("document %s definitions %s states no %s column", e.Document, e.Content, e.Parameter)
	case ErrorUnknownDefinitionColumn:
		return fmt.Sprintf("document %s definitions %s names %s column %q, which query %s does not project", e.Document, e.Content, e.Parameter, e.Actual, e.Query)
	default:
		return fmt.Sprintf("document planning failed for %s", e.Document)
	}
}

// Unwrap returns the underlying query-planning failure, if any.
func (e *Error) Unwrap() error { return e.Err }
