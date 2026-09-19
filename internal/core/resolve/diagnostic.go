package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// Diagnostic is a name-resolution problem tied to a source span.
type Diagnostic struct {
	Span    source.Span
	Message string
	// Code names the kind of problem; empty leaves it to the message.
	Code string
	// Fixes are the unambiguous edits resolving the diagnostic, offered by an
	// editor as quick fixes.
	Fixes []diag.Fix
	// Warning reports a well-formedness rule the reference states as a warning:
	// the model still resolves, so validation continues past it.
	Warning bool
}

// CodeNameConflict marks a name declared twice in one namespace, counting the
// members it inherits (SysML 7.6.1, KerML 7.3.2.1).
const CodeNameConflict = "name-conflict"
