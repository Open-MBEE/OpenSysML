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
	// ErrorUnsupportedOption reports a stylesheet option the selected
	// converter cannot apply, as it writes its own HTML.
	ErrorUnsupportedOption ErrorKind = "unsupported-option"
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

	// Option names the command-line option a converter cannot apply.
	Option string
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorUnknownEngine:
		return fmt.Sprintf("unknown PDF engine %q; -pdf-engine takes %s", e.Engine, strings.Join(e.Engines, ", "))
	case ErrorToolMissing:
		who := "rendering the document's diagrams"
		switch {
		case e.Engine != "":
			who = "the " + e.Engine + " engine"
		case e.EnvVar == KatexEnv || e.EnvVar == KatexCSSEnv:
			who = "typesetting the document's formulas"
		}
		msg := fmt.Sprintf("%s needs %s, which was not found", who, e.Tool)
		if e.EnvVar != "" {
			hint := "install it (scripts/download-doc-pdf-toolchain.sh provisions a pinned copy)"
			if e.EnvVar == PrinceEnv {
				hint = "install it"
			}
			msg += fmt.Sprintf("; %s and point %s at it", hint, e.EnvVar)
			if e.Engine != "" {
				msg += ", or select another engine with -pdf-engine"
			}
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
	case ErrorUnsupportedOption:
		return fmt.Sprintf("%s styles the HTML backend's page, which the %s engine does not read; select an engine reading HTML with -pdf-engine", e.Option, e.Engine)
	default:
		return "PDF rendering failed"
	}
}
