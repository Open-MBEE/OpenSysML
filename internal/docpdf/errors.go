package docpdf

import (
	"fmt"
	"strings"
)

// ErrorKind classifies a PDF-rendering failure.
type ErrorKind string

const (
	// ErrorUnknownEngine reports a -pdf-engine name no converter answers to.
	ErrorUnknownEngine ErrorKind = "unknown-engine"
	// ErrorToolMissing reports an external tool the selected converter needs
	// that could not be found.
	ErrorToolMissing ErrorKind = "tool-missing"
	// ErrorToolFailed reports an external tool that ran and failed.
	ErrorToolFailed ErrorKind = "tool-failed"
	// ErrorNoPDF reports a converter that succeeded without producing a PDF.
	ErrorNoPDF ErrorKind = "no-pdf"
	// ErrorUnclosedFence reports a diagram fence the Markdown never closes.
	ErrorUnclosedFence ErrorKind = "unclosed-fence"
	// ErrorDanglingCaption reports a caption marker not followed by a
	// fully-emphasized caption line.
	ErrorDanglingCaption ErrorKind = "dangling-caption"
)

// Error is a typed PDF-rendering failure.
type Error struct {
	Kind ErrorKind

	// Engine is the converter that was selected, where one was.
	Engine string

	// Tool is the external executable involved: its configured override or
	// its default name for ErrorToolMissing, its path for ErrorToolFailed.
	Tool string

	// EnvVar is the environment variable that overrides where Tool is found.
	EnvVar string

	// Engines lists the converters an unknown -pdf-engine could have named.
	Engines []string

	// Detail carries what the tool said on stderr, trimmed.
	Detail string
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorUnknownEngine:
		return fmt.Sprintf("unknown PDF engine %q; -pdf-engine takes %s", e.Engine, strings.Join(e.Engines, ", "))
	case ErrorToolMissing:
		who := "rendering the document's diagrams"
		if e.Engine != "" {
			who = "the " + e.Engine + " engine"
		}
		msg := fmt.Sprintf("%s needs %s, which was not found", who, e.Tool)
		if e.EnvVar != "" {
			hint := "install it (scripts/download-doc-pdf-toolchain.sh provisions a pinned copy)"
			if e.EnvVar == PrinceEnv {
				hint = "install it"
			}
			msg += fmt.Sprintf("; %s, point %s at it, or select another engine with -pdf-engine", hint, e.EnvVar)
		}
		return msg
	case ErrorToolFailed:
		msg := fmt.Sprintf("%s failed", e.Tool)
		if e.Detail != "" {
			msg += ": " + e.Detail
		}
		return msg
	case ErrorNoPDF:
		return fmt.Sprintf("%s reported success but wrote no PDF", e.Tool)
	case ErrorUnclosedFence:
		return "the document's Markdown opens a diagram fence it never closes"
	case ErrorDanglingCaption:
		return "the document's Markdown has a caption marker without a caption line after it"
	default:
		return "PDF rendering failed"
	}
}
