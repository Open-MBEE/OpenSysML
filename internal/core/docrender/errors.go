package docrender

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// ErrorKind classifies a document-rendering failure.
type ErrorKind string

const (
	ErrorNilDocument         ErrorKind = "nil-document"
	ErrorUnknownContent      ErrorKind = "unknown-content"
	ErrorMissingRendering    ErrorKind = "missing-rendering"
	ErrorUnrenderableDiagram ErrorKind = "unrenderable-diagram"
	ErrorUnrenderableForm    ErrorKind = "unrenderable-diagram-form"
	ErrorUnknownForm         ErrorKind = "unknown-diagram-form"
	ErrorEmptyStylesheet     ErrorKind = "empty-stylesheet"
	ErrorAmbiguousStylesheet ErrorKind = "ambiguous-stylesheet"
	ErrorUnsafeStylesheet    ErrorKind = "unsafe-stylesheet"
	ErrorUnknownTheme        ErrorKind = "unknown-theme"
)

// Error is a typed document-rendering failure.
type Error struct {
	Kind    ErrorKind
	Content string
	Actual  string
	// DiagramForm is the diagram form asked for, when a failure is about one.
	DiagramForm view.Form
	// Form is the backend that failed, "Markdown" when empty.
	Form string
}

// form names the backend a failure came from.
func (e *Error) form() string {
	if e.Form == "" {
		return "Markdown"
	}
	return e.Form
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorNilDocument:
		return "rendering requires an evaluated document"
	case ErrorUnknownContent:
		return fmt.Sprintf("content %s has unknown kind %q", e.Content, e.Actual)
	case ErrorMissingRendering:
		return fmt.Sprintf("diagram %s carries no view rendering", e.Content)
	case ErrorUnrenderableDiagram:
		return fmt.Sprintf("diagram %s has kind %q, which %s cannot draw", e.Content, e.Actual, e.form())
	case ErrorUnrenderableForm:
		return fmt.Sprintf("diagram %s has kind %q, which is not written as %s", e.Content, e.Actual, e.DiagramForm)
	case ErrorUnknownForm:
		return fmt.Sprintf("no diagram form is named %q; diagrams are written as %s", e.DiagramForm, view.FormNames(view.DiagramForms()))
	case ErrorEmptyStylesheet:
		return "a stylesheet must carry content to inline or a URL to link"
	case ErrorAmbiguousStylesheet:
		return fmt.Sprintf("stylesheet %s carries both content to inline and a URL to link; it can be one or the other", e.Actual)
	case ErrorUnsafeStylesheet:
		return "stylesheet content closes the style element it would be inlined in; link it by URL instead"
	case ErrorUnknownTheme:
		return fmt.Sprintf("no bundled theme is named %q; the themes are %s", e.Actual, strings.Join(Themes(), ", "))
	default:
		return "document rendering failed"
	}
}
